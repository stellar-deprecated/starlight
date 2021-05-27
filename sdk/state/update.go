package state

import (
	"errors"
	"strconv"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type PaymentProposal struct {
	ClosingTxXDR     string
	DeclarationTxXDR string
}

func (c *Channel) ValidatePayment(p *PaymentProposal, expectedPaymentAmount int64) bool {

	// TODO validate txC is correct structure (eg. only 1 payment operation)

	txCGeneric, err := txnbuild.TransactionFromXDR(p.ClosingTxXDR)
	if err != nil {
		return false
	}
	txC, isSimple := txCGeneric.Transaction()
	if !isSimple {
		return false
	}

	payOpAmount := int64(0)
	for _, op := range txC.Operations() {
		payOp, ok := op.(*txnbuild.Payment)
		if !ok {
			continue
		}
		// validate payment is correct
		payOpAmount, err := strconv.ParseInt(payOp.Amount, 10, 64)
		if err != nil {
			return false
		}
		if payOpAmount-c.Balance != expectedPaymentAmount {
			return false
		}
	}

	c.Balance = payOpAmount

	return true
}

// TODO - validate inputs (eg. no negative amounts)
func (c *Channel) ProposePayment(initiator Participant, responder Participant, amountToInitiator int64, amountToResponder int64, startSequence int64, i int64, e int64, o time.Duration, networkPassphrase string) (*PaymentProposal, error) {
	txD, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         initiator.Escrow.FromAddress(),
		StartSequence:           startSequence,
		IterationNumber:         i,
		IterationNumberExecuted: e,
	})
	if err != nil {
		return nil, err
	}
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      o,
		ObservationPeriodLedgerGap: 0,
		InitiatorSigner:            initiator.KP.FromAddress(),
		ResponderSigner:            responder.KP.FromAddress(),
		InitiatorEscrow:            initiator.Escrow.FromAddress(),
		ResponderEscrow:            responder.Escrow.FromAddress(),
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

	cXDR, err := txC.Base64()
	if err != nil {
		return nil, err
	}
	dXDR, err := txD.Base64()
	if err != nil {
		return nil, err
	}
	c.ProposalStatus = ProposalStatusProposed
	c.Balance += amountToResponder
	c.Balance -= amountToInitiator
	return &PaymentProposal{
		ClosingTxXDR:     cXDR,
		DeclarationTxXDR: dXDR,
	}, nil
}

func (c *Channel) ConfirmPayment(p *PaymentProposal, participant Participant, networkPassphrase string) (*PaymentProposal, error) {

	txDGeneric, err := txnbuild.TransactionFromXDR(p.DeclarationTxXDR)
	if err != nil {
		return nil, err
	}
	txD, isSimple := txDGeneric.Transaction()
	if !isSimple {
		return nil, errors.New("not a generic transaction")
	}
	txCGeneric, err := txnbuild.TransactionFromXDR(p.ClosingTxXDR)
	if err != nil {
		return nil, err
	}
	txC, isSimple := txCGeneric.Transaction()
	if !isSimple {
		return nil, errors.New("not a generic transaction")
	}

	if c.ProposalStatus != ProposalStatusProposed {
		txD, err := txD.Sign(networkPassphrase, participant.KP)
		if err != nil {
			return nil, err
		}
		txC, err := txC.Sign(networkPassphrase, participant.KP)
		if err != nil {
			return nil, err
		}

		cXDR, err := txC.Base64()
		if err != nil {
			return nil, err
		}
		dXDR, err := txD.Base64()
		if err != nil {
			return nil, err
		}
		c.ProposalStatus = "confirmed"
		return &PaymentProposal{
			ClosingTxXDR:     cXDR,
			DeclarationTxXDR: dXDR,
		}, nil
	} else if c.ProposalStatus == ProposalStatusProposed {
		txD, err := txD.Sign(networkPassphrase, participant.KP)
		if err != nil {
			return nil, err
		}
		dXDR, err := txD.Base64()
		if err != nil {
			return nil, err
		}
		p.DeclarationTxXDR = dXDR
		c.ProposalStatus = "confirmed"
		return p, nil
	}
	// TODO - handle this case
	return nil, nil
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
