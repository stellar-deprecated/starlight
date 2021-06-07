package state

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type Payment struct {
	IterationNumber       int64
	Amount                Amount
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FromInitiator         bool
}

func (p Payment) isEqual(p2 Payment) bool {
	return p.IterationNumber == p2.IterationNumber && p.Amount == p2.Amount && p.FromInitiator == p2.FromInitiator
}

func (p Payment) isEmpty() bool {
	return p.IterationNumber == 0 && p.Amount == (Amount{}) && p.FromInitiator == false && len(p.CloseSignatures) == 0 && len(p.DeclarationSignatures) == 0
}

type CloseAgreement struct {
	IterationNumber       int64
	Balance               Amount
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
}

func (c *Channel) ProposePayment(amount Amount) (Payment, error) {
	if amount.Amount <= 0 {
		return Payment{}, errors.New("payment amount must be greater than 0")
	}
	newBalance := int64(0)
	if c.initiator {
		newBalance = c.Balance().Amount + amount.Amount
	} else {
		newBalance = c.Balance().Amount - amount.Amount
	}
	txClose, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.NextIterationNumber(),
		AmountToInitiator:          maxInt64(0, newBalance*-1),
		AmountToResponder:          maxInt64(0, newBalance),
	})
	if err != nil {
		return Payment{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return Payment{}, err
	}
	p := Payment{
		IterationNumber: c.NextIterationNumber(),
		Amount:          amount,
		CloseSignatures: txClose.Signatures(),
		FromInitiator:   c.initiator,
	}
	c.latestUnconfirmedPayment = p
	return p, nil
}

func (c *Channel) PaymentTxs(p Payment) (close, decl *txnbuild.Transaction, err error) {
	newBalance := c.newBalance(p)
	close, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.NextIterationNumber(),
		AmountToInitiator:          maxInt64(0, newBalance.Amount*-1),
		AmountToResponder:          maxInt64(0, newBalance.Amount),
	})
	if err != nil {
		return
	}
	decl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         c.NextIterationNumber(),
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return
	}
	return
}

func (c *Channel) ConfirmPayment(p Payment) (payment Payment, fullySigned bool, err error) {
	defer func() {
		if err != nil {
			return
		}
		if fullySigned {
			c.latestUnconfirmedPayment = Payment{}
			newBalance := c.newBalance(p)
			c.latestCloseAgreement = CloseAgreement{p.IterationNumber, newBalance, p.CloseSignatures, p.DeclarationSignatures}
		} else {
			c.latestUnconfirmedPayment = p
		}
	}()

	if p.IterationNumber != c.NextIterationNumber() {
		return p, fullySigned, errors.New(fmt.Sprintf("invalid payment iteration number, got: %s want: %s",
			strconv.FormatInt(p.IterationNumber, 10), strconv.FormatInt(c.NextIterationNumber(), 10)))
	}
	if !c.latestUnconfirmedPayment.isEmpty() && !c.latestUnconfirmedPayment.isEqual(p) {
		return p, fullySigned, errors.New("a different unconfirmed payment exists")
	}

	txClose, txDecl, err := c.PaymentTxs(p)
	if err != nil {
		return p, fullySigned, err
	}

	// If remote has not signed close, error as is invalid.
	signed, err := c.verifySigned(txClose, p.CloseSignatures, c.remoteSigner)
	if err != nil {
		return p, fullySigned, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return p, fullySigned, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}

	// If local has not signed close, sign.
	signed, err = c.verifySigned(txClose, p.CloseSignatures, c.localSigner)
	if err != nil {
		return p, fullySigned, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return p, fullySigned, fmt.Errorf("signing close with local: %w", err)
		}
		p.CloseSignatures = append(p.CloseSignatures, txClose.Signatures()...)
	}

	// Local should always sign declaration if have not yet.
	signed, err = c.verifySigned(txDecl, p.DeclarationSignatures, c.localSigner)
	if err != nil {
		return p, fullySigned, fmt.Errorf("verifying declaration signed by local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return p, fullySigned, err
		}
		p.DeclarationSignatures = append(p.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If remote has not signed declaration, it is incomplete.
	signed, err = c.verifySigned(txDecl, p.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return p, fullySigned, fmt.Errorf("verifying declaration signed by remote: %w", err)
	}
	if !signed {
		return p, fullySigned, nil
	}

	// All signatures are present that would be required to submit all
	// transactions in the payment.
	fullySigned = true
	return p, fullySigned, nil
}

func maxInt64(x int64, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
