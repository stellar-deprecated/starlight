package state

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
)

// The steps for a channel close are as follows:
// 1. A submits latest declaration tx
// 2. A calls ProposeClose to propose an immediate close by resigning the
//    current close tx
// 3. B calls ConfirmClose to sign and store result
// 4. A calls ConfirmClose to store result
// 5. A or B submit the new close tx
// 6. If A or B declines or is not responsive at any step, A or B may submit the
//    original close tx after the observation period.

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

// ProposeClose proposes that the latest authorized close agreement be submitted
// without waiting the observation period. This should be used when participants
// are in agreement on the final close state, but would like to submit earlier
// than the original observation time.
func (c *Channel) ProposeClose() (CloseAgreement, error) {
	d := c.latestAuthorizedCloseAgreement.Details
	d.ObservationPeriodTime = 0
	d.ObservationPeriodLedgerGap = 0

	_, txClose, err := c.CloseTxs(d)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("making coordianted close transactions: %w", err)
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing close transaction: %w", err)
	}

	// Store the close agreement while participants iterate on signatures.
	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:         d,
		CloseSignatures: txClose.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

// ConfirmClose agrees to a close agreement to be submitted without waiting the
// observation period. The agreement will always be accepted if it is identical
// to the latest authorized close agreement, and it is signed by the participant
// proposing the close.
func (c *Channel) ConfirmClose(ca CloseAgreement) (closeAgreement CloseAgreement, authorized bool, err error) {
	latestWithoutObservation := c.latestAuthorizedCloseAgreement.Details
	latestWithoutObservation.ObservationPeriodTime = 0
	latestWithoutObservation.ObservationPeriodLedgerGap = 0

	if ca.Details != latestWithoutObservation {
		return CloseAgreement{}, authorized, fmt.Errorf("close agreement details do not match saved latest authorized close agreement")
	}

	_, txClose, err := c.CloseTxs(ca.Details)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("making close transactions: %w", err)
	}

	// If remote has not signed, error as is invalid.
	signed, err := c.verifySigned(txClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying close signature with remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying close: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, authorized, fmt.Errorf("verifying close signature with local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, authorized, fmt.Errorf("signing close transaction: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txClose.Signatures()...)
	}

	// The new close agreement is valid and authorized, store and promote it.
	authorized = true
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details:               ca.Details,
		CloseSignatures:       appendNewSignatures(c.latestUnauthorizedCloseAgreement.CloseSignatures, ca.CloseSignatures),
		DeclarationSignatures: c.latestUnauthorizedCloseAgreement.DeclarationSignatures,
	}
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}
	return c.latestAuthorizedCloseAgreement, authorized, nil
}
