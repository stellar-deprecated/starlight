package state

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
)

// The steps for a channel coordinated close are as followed:
// 1. Initiator or Responder submits latest declaration tx
// 2. Initiator calls ProposeCoordinatedClose (in steps 2-4 Initiator and Responder are interchangeable,
//    as long as they alternate)
// 3. Responder calls ConfirmCoordinatedClose
// 4. Initiator calls ConfirmCoordinatedClose

func (c *Channel) CloseTxs(d CloseAgreementDetails) (txDecl *txnbuild.Transaction, txClose *txnbuild.Transaction, err error) {
	txDecl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         d.IterationNumber,
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return nil, nil, err
	}
	txClose, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            d.IterationNumber,
		AmountToInitiator:          amountToInitiator(d.Balance.Amount),
		AmountToResponder:          amountToResponder(d.Balance.Amount),
		Asset:                      d.Balance.Asset,
	})
	if err != nil {
		return nil, nil, err
	}
	return txDecl, txClose, nil
}

func amountToInitiator(balance int64) int64 {
	if balance < 0 {
		return balance * -1
	}
	return 0
}

func amountToResponder(balance int64) int64 {
	if balance > 0 {
		return balance
	}
	return 0
}

// ProposeCoordinatedClose proposes a close transaction to be submitted immediately.
// This should be used when participants are in agreement on the final txClose parameters, but would
// like to submit earlier than the original observation time.
func (c *Channel) ProposeCoordinatedClose() (CloseAgreement, error) {
	d := c.latestAuthorizedCloseAgreement.Details
	d.ObservationPeriodTime = 0
	d.ObservationPeriodLedgerGap = 0

	_, txCoordinatedClose, err := c.CloseTxs(d)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("making coordianted close transactions: %w", err)
	}
	txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing coordinated close transaction: %w", err)
	}

	// Store the close agreement while participants iterate on signatures.
	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:         d,
		CloseSignatures: txCoordinatedClose.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

func (c *Channel) ConfirmCoordinatedClose(ca CloseAgreement) (closeAgreement CloseAgreement, authorized bool, err error) {
	latestWithoutObservation := c.latestAuthorizedCloseAgreement.Details
	latestWithoutObservation.ObservationPeriodTime = 0
	latestWithoutObservation.ObservationPeriodLedgerGap = 0

	if ca.Details != latestWithoutObservation {
		return CloseAgreement{}, authorized, fmt.Errorf("close agreement details do not match saved latest authorized close agreement")
	}

	_, txCoordinatedClose, err := c.CloseTxs(ca.Details)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("making coordinated close transactions: %w", err)
	}

	// If remote has not signed, error as is invalid.
	signed, err := c.verifySigned(txCoordinatedClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying coordinated close signature with remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying coordinated close: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txCoordinatedClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying coordinated close signature with local: %w", err)
	}
	if !signed {
		txCoordinatedClose, err = txCoordinatedClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, authorized, fmt.Errorf("signing coordinated close transaction: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txCoordinatedClose.Signatures()...)
	}

	// The new close agreement is valid and fully signed, store and promote it.
	authorized = true
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details:               ca.Details,
		CloseSignatures:       ca.CloseSignatures,
		DeclarationSignatures: c.latestAuthorizedCloseAgreement.DeclarationSignatures,
	}
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}
	return c.latestAuthorizedCloseAgreement, authorized, nil
}
