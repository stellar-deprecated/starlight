package state

import (
	"errors"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/xdr"
)

type PaymentProposal struct {
	IterationNumber            int64
	ObservarvationPeriodTime   time.Duration
	ObservationPeriodLedgerGap int64 // TODO - int64 right?
	AmountToInitiator          int64
	AmountToResponder          int64
	CloseSignatures            xdr.DecoratedSignature
	DeclarationSignatures      xdr.DecoratedSignature
}

// TODO - when to validate the payments?
func (c *Channel) ValidatePayment(p *PaymentProposal, expectedPayToInitiator int64, expectedPayToResponder int64) bool {
	return true
}

// TODO - validate inputs (eg. no negative amounts)
// initiator always creates the proposals
func (c *Channel) NewPaymentProposal(me Participant, other Participant, amountToInitiator int64, amountToResponder int64, startSequence int64, i int64, e int64, o time.Duration, observationPeriodLedgerGap int64, networkPassphrase string) (*PaymentProposal, error) {
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      o,
		ObservationPeriodLedgerGap: 0,
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

	// TODO - when should this happen?
	// c.ProposalStatus = ProposalStatusProposed
	// TODO - when should the channel balance be updated?
	// c.Balance += amountToResponder
	// c.Balance -= amountToInitiator

	// TODO - how to get the transaction signature?
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
		// 1. First confirmation:
		// TODO - validate P_i
		// build D_i based off of P_i
		// build C_i based off of P_i
		// check that the signatures from P_i are correct
		// sign C_i and D_i
		// send something back (P_i with your new signature?)

		txD, err := txbuild.Declaration(txbuild.DeclarationParams{
			InitiatorEscrow:         initiator.Escrow.FromAddress(),
			StartSequence:           p.StartSequence,
			IterationNumber:         p.IterationNumber,
			IterationNumberExecuted: p.ExecutionNumber,
		})
		if err != nil {
			return nil, err
		}

		if !verifyTxSignatures(txD, p.CloseSignatures) {
			return nil, errors.New("invalid declaration transaction")
		}

		// TODO - add the given signature to your new created transaction

		txD, err := txD.Sign(networkPassphrase, me.KP)
		if err != nil {
			return nil, err
		}

		txC, err := txbuild.Close(txbuild.CloseParams{
			ObservationPeriodTime:      p.ObservationPeriodTime,
			ObservationPeriodLedgerGap: p.ObservationPeriodLedgerGap,
			InitiatorSigner:            initiator.KP.FromAddress(),
			ResponderSigner:            responder.KP.FromAddress(),
			InitiatorEscrow:            initiator.Escrow.FromAddress(),
			ResponderEscrow:            responder.Escrow.FromAddress(),
			StartSequence:              p.StartSequence, // TODO - should this be on P?
			IterationNumber:            p.IterationNumber,
			AmountToInitiator:          p.AmountToInitiator,
			AmountToResponder:          p.AmountToResponder,
		})
		if err != nil {
			return nil, err
		}
		// TODO - why is signing here bad?
		txC, err = txC.Sign(networkPassphrase, me.KP)
		if err != nil {
			return nil, err
		}
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

// Common data both participants will have during the test.
type Participant struct {
	Name                 string
	KP                   *keypair.Full
	Escrow               *keypair.Full
	EscrowSequenceNumber int64
	Contribution         int64
}

// TODO
// - ProposalStatus is best?
// - message validation
