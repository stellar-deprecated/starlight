package state

import (
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/xdr"
)

type PaymentProposal struct {
	IterationNumber            int64
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	AmountToInitiator          int64
	AmountToResponder          int64
	CloseSignatures            []xdr.DecoratedSignature
	DeclarationSignatures      []xdr.DecoratedSignature
}

// TODO - validate inputs? (eg. no negative amounts)
// TODO - payments to be in Amount struct
// initiator will only call this
func (c *Channel) NewPaymentProposal(payToInitiator int64, payToResponder int64) (*PaymentProposal, error) {
	// TODO - remove
	fmt.Println(c.Balance, payToResponder, payToInitiator)
	newBalance := c.Balance + payToResponder - payToInitiator
	amountToInitiator := int64(0)
	amountToResponder := int64(0)
	if newBalance > 0 {
		amountToResponder = newBalance
	} else {
		amountToInitiator = newBalance * -1
	}
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.localSigner.FromAddress(),
		ResponderSigner:            c.remoteSigner,
		InitiatorEscrow:            c.localEscrowAccount.Address,
		ResponderEscrow:            c.remoteEscrowAccount.Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.iterationNumber,
		AmountToInitiator:          amountToInitiator,
		AmountToResponder:          amountToResponder,
	})
	if err != nil {
		return nil, err
	}
	txC, err = txC.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return nil, err
	}
	c.Balance += amountToResponder
	c.Balance -= amountToInitiator
	return &PaymentProposal{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		AmountToInitiator:          amountToInitiator,
		AmountToResponder:          amountToResponder,
		CloseSignatures:            txC.Signatures(),
	}, nil
}

// TODO - what do to when you don't have the full other's KP? - Change to FullAddress?
func (c *Channel) ConfirmPayment(p *PaymentProposal) (*PaymentProposal, error) {
	// TODO - is c.initiator good way of differentiating?
	if !c.initiator {
		newBalance := c.Balance + p.AmountToResponder - p.AmountToInitiator
		// TODO - better var names to differentiate from C_i fields?
		amountToInitiator := int64(0)
		amountToResponder := int64(0)
		if newBalance > 0 {
			amountToResponder = newBalance
		} else {
			amountToInitiator = newBalance
		}
		txC, err := txbuild.Close(txbuild.CloseParams{
			ObservationPeriodTime:      c.observationPeriodTime,
			ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
			InitiatorSigner:            c.remoteSigner,
			ResponderSigner:            c.localSigner.FromAddress(),
			InitiatorEscrow:            c.remoteEscrowAccount.Address,
			ResponderEscrow:            c.localEscrowAccount.Address,
			StartSequence:              c.startingSequence,
			IterationNumber:            c.iterationNumber,
			AmountToInitiator:          amountToInitiator,
			AmountToResponder:          amountToResponder,
		})
		if err != nil {
			return nil, err
		}
		if err := c.verifySigned(txC, p.CloseSignatures, c.remoteSigner); err != nil {
			return nil, err
		}
		for _, sig := range p.CloseSignatures {
			txC, err = txC.AddSignatureDecorated(sig)
			if err != nil {
				return nil, err
			}
		}
		// TODO - why is signing here bad?
		txC, err = txC.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return nil, err
		}
		txD, err := txbuild.Declaration(txbuild.DeclarationParams{
			InitiatorEscrow:         c.remoteEscrowAccount.Address,
			StartSequence:           c.startingSequence,
			IterationNumber:         c.iterationNumber,
			IterationNumberExecuted: c.iterationNumberExecuted,
		})
		if err != nil {
			return nil, err
		}
		txD, err = txD.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return nil, err
		}
		c.Balance = newBalance
		p.CloseSignatures = txC.Signatures()
		p.DeclarationSignatures = txD.Signatures()
		return p, nil
	}

	txD, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.localEscrowAccount.Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         c.iterationNumber,
		IterationNumberExecuted: c.iterationNumberExecuted,
	})
	if err != nil {
		return nil, err
	}

	txD, err = txD.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return nil, err
	}

	p.DeclarationSignatures = txD.Signatures()
	return p, nil
}
