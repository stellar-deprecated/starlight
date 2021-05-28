package state

import (
	"errors"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/xdr"
)

type PaymentProposal struct {
	IterationNumber            int64
	ObservarvationPeriodTime   time.Duration
	ObservationPeriodLedgerGap int64
	AmountToInitiator          int64
	AmountToResponder          int64
	CloseSignatures            xdr.DecoratedSignature
	DeclarationSignatures      xdr.DecoratedSignature
}

// TODO - validate inputs? (eg. no negative amounts)
// initiator will only call this
func (c *Channel) NewPaymentProposal(me Participant, other Participant, amountToInitiator int64, amountToResponder int64, startSequence int64, i int64, e int64, o time.Duration, observationPeriodLedgerGap int64, networkPassphrase string) (*PaymentProposal, error) {
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      o,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		InitiatorSigner:            me.KP.FromAddress(),
		ResponderSigner:            other.KP.FromAddress(),
		InitiatorEscrow:            me.Escrow.FromAddress(),
		ResponderEscrow:            other.Escrow.FromAddress(),
		StartSequence:              startSequence,
		IterationNumber:            i,
		AmountToInitiator:          amountToInitiator,
		AmountToResponder:          amountToResponder,
	})
	if err != nil {
		return nil, err
	}
	txC, err = txC.Sign(networkPassphrase, initiator.KP)
	if err != nil {
		return nil, err
	}
	c.Balance += amountToResponder
	c.Balance -= amountToInitiator
	return &PaymentProposal{
		IterationNumber:            i,
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		AmountToInitiator:          amountToInitiator,
		AmountToResponder:          amountToResponder,
		CloseSignatures:            txC.Signatures(),
	}, nil
}

// TODO - what do to when you don't have the full other's KP? - Change to FullAddress?
func (c *Channel) ConfirmPayment(p *PaymentProposal, initiator Participant, responder Participant, isInitiator bool, networkPassphrase string) (*PaymentProposal, error) {
	if !isInitiator {
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
			ObservationPeriodTime:      p.ObservationPeriodTime,
			ObservationPeriodLedgerGap: p.ObservationPeriodLedgerGap,
			InitiatorSigner:            initiator.KP.FromAddress(),
			ResponderSigner:            responder.KP.FromAddress(),
			InitiatorEscrow:            initiator.Escrow.FromAddress(),
			ResponderEscrow:            responder.Escrow.FromAddress(),
			StartSequence:              p.StartSequence,
			IterationNumber:            p.IterationNumber,
			AmountToInitiator:          p.AmountToInitiator,
			AmountToResponder:          p.AmountToResponder,
		})
		if err != nil {
			return nil, err
		}
		if !verifyTxSignatures(txC, p.CloseSignatures) {
			return nil, errors.New("invalid declaration transaction")
		}
		txC, err = txC.AddSignatureDecorated(p.CloseSignatures)
		if err != nil {
			return nil, err
		}
		// TODO - why is signing here bad?
		txC, err = txC.Sign(networkPassphrase, me.KP)
		if err != nil {
			return nil, err
		}

		txD, err := txbuild.Declaration(txbuild.DeclarationParams{
			InitiatorEscrow:         initiator.Escrow.FromAddress(),
			StartSequence:           p.StartSequence,
			IterationNumber:         p.IterationNumber,
			IterationNumberExecuted: p.ExecutionNumber,
		})
		if err != nil {
			return nil, err
		}
		txD, err := txD.Sign(networkPassphrase, me.KP)
		if err != nil {
			return nil, err
		}

		c.Balance = newBalance
		p.CloseSignatures = txC.Signatures()
		p.DeclarationSignatures = txD.Signatures()
		return p
	}

	txD, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         initiator.Escrow.FromAddress(),
		StartSequence:           p.StartSequence,
		IterationNumber:         p.IterationNumber,
		IterationNumberExecuted: p.ExecutionNumber,
	})
	if err != nil {
		return nil, err
	}

	txD, err := txD.Sign(networkPassphrase, me.KP)
	if err != nil {
		return nil, err
	}

	p.DeclarationSignatures = txD.Signatures()
	return p
}
