package state

import (
	"errors"
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/xdr"
)

type Payment struct {
	IterationNumber       int64
	Amount                int64
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FromInitiator         bool
}

func (c *Channel) NewPayment(amount int64) (*Payment, error) {
	if amount <= 0 {
		return nil, errors.New("payment amount must be greater than 0")
	}
	newBalance := int64(0)
	if c.initiator {
		newBalance = c.amount.Amount + amount
	} else {
		newBalance = c.amount.Amount - amount
	}
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.iterationNumber,
		AmountToInitiator:          maxInt64(0, newBalance*-1),
		AmountToResponder:          maxInt64(0, newBalance),
	})
	if err != nil {
		return nil, err
	}
	txC, err = txC.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return nil, err
	}
	return &Payment{
		Amount:          amount,
		CloseSignatures: txC.Signatures(),
		FromInitiator:   c.initiator,
	}, nil
}

func (c *Channel) ConfirmPayment(p *Payment) (*Payment, error) {
	var newCloseSignatures, newDeclarationSignatures []xdr.DecoratedSignature
	var amountFromInitiator, amountFromResponder int64
	if p.FromInitiator {
		amountFromInitiator = p.Amount
	} else {
		amountFromResponder = p.Amount
	}
	newBalance := c.amount.Amount + amountFromInitiator - amountFromResponder

	// validate txC, should always be signed correctly
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.iterationNumber,
		AmountToInitiator:          maxInt64(0, newBalance*-1),
		AmountToResponder:          maxInt64(0, newBalance),
	})
	if err != nil {
		return nil, err
	}
	if err := c.verifySigned(txC, p.CloseSignatures, c.remoteSigner); err != nil {
		return nil, fmt.Errorf("incorrect closing transaction, the one given may have different data: %w", err)
	}
	// validate txD, may or may not be signed depending where in the payment step we are
	signedTxD := false
	txD, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         c.iterationNumber,
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return nil, err
	}
	if err := c.verifySigned(txD, p.DeclarationSignatures, c.remoteSigner); err == nil {
		signedTxD = true
	}
	// sign C_i if given a signed C_i with no D_i
	if !signedTxD {
		txC, err = txC.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return nil, err
		}
		newCloseSignatures = txC.Signatures()
	}
	// sign D_i always if above passes
	txD, err = txD.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return nil, err
	}
	newDeclarationSignatures = txD.Signatures()

	p.CloseSignatures = append(p.CloseSignatures, newCloseSignatures...)
	p.DeclarationSignatures = append(p.DeclarationSignatures, newDeclarationSignatures...)
	c.amount.Amount = newBalance
	return p, nil
}

func maxInt64(x int64, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
