package state

import (
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
)

// The steps for a channel coordinated close are as followed:
// 1. Initiator or Responder submits latest declaration tx
// 2. Initiator calls ProposeCoordinatedClose (in steps 2-4 Initiator and Responder are interchangeable,
//    as long as they alternate)
// 3. Responder calls ConfirmCoordinatedClose
// 4. Initiator calls ConfirmCoordinatedClose

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

// ProposeCoordinatedClose proposes a close transaction to be submitted immediately.
// This should be used when participants are in agreement on the final txClose parameters, but would
// like to submit earlier than the original observation time.
func (c *Channel) ProposeCoordinatedClose() (CloseAgreement, error) {
	txCoordinatedClose, err := c.makeCloseTx(0, 0)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("making coordianted close transactions: %w", err)
	}
	txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing coordinated close transaction: %w", err)
	}

	// store an unconfirmed close agreement with new close signatures
	c.latestUnconfirmedCloseAgreement = CloseAgreement{
		IterationNumber: c.latestCloseAgreement.IterationNumber,
		Balance:         c.latestCloseAgreement.Balance,
		CloseSignatures: txCoordinatedClose.Signatures(),
	}
	return c.latestUnconfirmedCloseAgreement, nil
}

func (c *Channel) ConfirmCoordinatedClose(ca CloseAgreement) (closeAgreement CloseAgreement, fullySigned bool, err error) {
	txCoordinatedClose, err := c.makeCloseTx(0, 0)
	if err != nil {
		return CloseAgreement{}, fullySigned, fmt.Errorf("making coordinated close transactions: %w", err)
	}

	// If remote has not signed, error as is invalid.
	signed, err := c.verifySigned(txCoordinatedClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fullySigned, fmt.Errorf("verifying coordinated close signature with remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, fullySigned, fmt.Errorf("verifying coordinated close: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txCoordinatedClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fullySigned, fmt.Errorf("verifying coordinated close signature with local: %w", err)
	}
	if !signed {
		txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fullySigned, fmt.Errorf("signing coordinated close transaction: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txCoordinatedClose.Signatures()...)
	}
	fullySigned = true

	// new close agreement is valid and fully signed, store it as latestCloseAgreement and clear latestUnconfirmedCloseAgreement
	c.latestCloseAgreement = CloseAgreement{
		IterationNumber:       ca.IterationNumber,
		Balance:               ca.Balance,
		CloseSignatures:       appendNewSignatures(c.latestUnconfirmedCloseAgreement.CloseSignatures, ca.CloseSignatures),
		DeclarationSignatures: c.latestUnconfirmedCloseAgreement.DeclarationSignatures,
	}
	c.latestUnconfirmedCloseAgreement = CloseAgreement{}
	return c.latestCloseAgreement, fullySigned, nil
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
