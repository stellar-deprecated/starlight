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
// 2. Initiator calls ProposeCoordinatedClose (in steps 2-4 Initiator and Responder are interchangeable,
//    as long as they alternate)
// 3. Responder calls ConfirmCoordinatedClose
// 4. Initiator calls ConfirmCoordinatedClose

type CoordinatedClose struct {
	CloseSignatures []xdr.DecoratedSignature
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
	txClose, err := c.makeCloseTx(0, 0)
	if err != nil {
		return nil, err
	}
	return txClose, nil
}

// ProposeCoordinatedClose proposes parameters for a close transaction to be submitted earlier.
// This should be used when participants are in agreement on the final txClose parameters, but would
// like to submit earlier than the original observation time.
func (c *Channel) ProposeCoordinatedClose() (CoordinatedClose, error) {
	txCoordinatedClose, err := c.makeCloseTx(0, 0)
	if err != nil {
		return CoordinatedClose{}, fmt.Errorf("making coordianted close transactions: %w", err)
	}
	txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CoordinatedClose{}, fmt.Errorf("signing coordinated close transaction: %w", err)
	}
	return CoordinatedClose{
		CloseSignatures: txCoordinatedClose.Signatures(),
	}, nil
}

func (c *Channel) ConfirmCoordinatedClose(cc CoordinatedClose) (coordinatedClose CoordinatedClose, fullySigned bool, err error) {
	txCoordinatedClose, err := c.makeCloseTx(0, 0)
	if err != nil {
		return CoordinatedClose{}, fullySigned, fmt.Errorf("making coordinated close transactions: %w", err)
	}

	// If remote has not signed coordinated close, error as is invalid.
	signed, err := c.verifySigned(txCoordinatedClose, cc.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CoordinatedClose{}, fullySigned, fmt.Errorf("verifying coordinated close signature with remote: %w", err)
	}
	if !signed {
		return CoordinatedClose{}, fullySigned, fmt.Errorf("verifying coordinated close: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txCoordinatedClose, cc.CloseSignatures, c.localSigner)
	if err != nil {
		return CoordinatedClose{}, fullySigned, fmt.Errorf("verifying coordinated close signature with local: %w", err)
	}
	if !signed {
		txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CoordinatedClose{}, fullySigned, fmt.Errorf("signing coordinated close transaction: %w", err)
		}
		cc.CloseSignatures = append(cc.CloseSignatures, txCoordinatedClose.Signatures()...)
	}
	fullySigned = true

	// TODO - merge instead of overwrite, similar to ConfirmProposal
	c.coordinatedClose = cc
	return cc, fullySigned, nil
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
		AmountToInitiator:          c.initiatorBalanceAmount(),
		AmountToResponder:          c.responderBalanceAmount(),
		Asset:                      c.latestCloseAgreement.Balance.Asset,
	})
}
