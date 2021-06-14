package state

import (
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// The steps for a channel coordinated close are as followed:
// 1. Initiator or Responder submits latest declaration tx
// (in steps 2-4 Initiator and Responder are interchangeable, as long as they alternate)
// 2. Initiator calls ProposeCoordinatedClose
// 3. Responder calls ConfirmCoordinatedClose
// 4. Initiator calls ConfirmCoordinatedClose

type CoordinatedClose struct {
	observationPeriodTime      time.Duration
	observationPeriodLedgerGap int64
	closeSignatures            []xdr.DecoratedSignature
}

func (cc CoordinatedClose) CloseSignatures() []xdr.DecoratedSignature {
	return cc.closeSignatures
}

// mergeCoordinatedCloseData merges the data from a new coordinated close into an existing one. The signatures of the existing
// coordinated close are appended to so that existing signatures are not lost.
func mergeCoordinatedCloseData(cc CoordinatedClose, newCoordinatedClose CoordinatedClose) CoordinatedClose {
	return CoordinatedClose{
		observationPeriodTime:      newCoordinatedClose.observationPeriodTime,
		observationPeriodLedgerGap: newCoordinatedClose.observationPeriodLedgerGap,
		closeSignatures:            appendNewSignatures(cc.closeSignatures, newCoordinatedClose.closeSignatures),
	}
}

func (c *Channel) CloseTxs() (txDecl *txnbuild.Transaction, txClose *txnbuild.Transaction, err error) {
	txDecl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         c.latestCloseAgreement.IterationNumber,
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return nil, nil, err
	}
	txClose, err = c.makeCloseTx(c.observationPeriodTime, c.observationPeriodLedgerGap)
	if err != nil {
		return nil, nil, err
	}
	return txDecl, txClose, nil
}

func (c *Channel) CoordinatedCloseTx() (*txnbuild.Transaction, error) {
	txClose, err := c.makeCloseTx(c.coordinatedClose.observationPeriodTime, c.coordinatedClose.observationPeriodLedgerGap)
	if err != nil {
		return nil, err
	}
	return txClose, nil
}

// ProposeCoordinatedClose proposes parameters for a close transaction to be submitted earlier.
// This should be used when participants are in agreement on the final txClose parameters, but would
// like to submit earlier than the original observation time.
func (c *Channel) ProposeCoordinatedClose(observationPeriodTime time.Duration, observationPeriodLedgerGap int64) (CoordinatedClose, error) {
	txCoordinatedClose, err := c.makeCloseTx(observationPeriodTime, observationPeriodLedgerGap)
	if err != nil {
		return CoordinatedClose{}, err
	}
	txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CoordinatedClose{}, nil
	}
	return CoordinatedClose{
		observationPeriodTime:      observationPeriodTime,
		observationPeriodLedgerGap: observationPeriodLedgerGap,
		closeSignatures:            txCoordinatedClose.Signatures(),
	}, nil
}

func (c *Channel) ConfirmCoordinatedClose(cc CoordinatedClose) (CoordinatedClose, error) {
	txCoordinatedClose, err := c.makeCloseTx(cc.observationPeriodTime, cc.observationPeriodLedgerGap)
	if err != nil {
		return CoordinatedClose{}, err
	}

	// If remote has not signed coordinated close, error as is invalid.
	signed, err := c.verifySigned(txCoordinatedClose, cc.closeSignatures, c.remoteSigner)
	if err != nil {
		return CoordinatedClose{}, err
	}
	if !signed {
		return CoordinatedClose{}, fmt.Errorf("verifying coordinated close: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txCoordinatedClose, cc.closeSignatures, c.localSigner)
	if err != nil {
		return CoordinatedClose{}, err
	}
	if !signed {
		txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CoordinatedClose{}, err
		}
		cc.closeSignatures = append(cc.closeSignatures, txCoordinatedClose.Signatures()...)
	}

	c.coordinatedClose = mergeCoordinatedCloseData(c.coordinatedClose, cc)
	return cc, nil
}

// makeCloseTx is a helper method for creating a close transaction with custom observation values.
func (c *Channel) makeCloseTx(observationPeriodTime time.Duration, observationPeriodLedgerGap int64) (*txnbuild.Transaction, error) {
	return txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.latestCloseAgreement.IterationNumber,
		AmountToInitiator:          c.initiatorClaimAmount(),
		AmountToResponder:          c.responderClaimAmount(),
	})
}
