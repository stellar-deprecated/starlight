package state

import (
	"errors"
	"fmt"
	"time"

	"github.com/stellar/go/xdr"
)

// The high level steps for creating a channel update should be as follows, where the returned payments
// flow to the next step:
// 1. Sender calls ProposePayment
// 2. Receiver calls ConfirmPayment
// 3. Sender calls ConfirmPayment
// 4. Receiver calls ConfirmPayment

// CloseAgreementDetails contains the details that the participants agree on.
type CloseAgreementDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	IterationNumber            int64
	Balance                    int64
}

// CloseAgreement contains everything a participant needs to execute the close
// agreement on the Stellar network.
type CloseAgreement struct {
	Details               CloseAgreementDetails
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
}

func (ca CloseAgreement) isEmpty() bool {
	return ca.Details == CloseAgreementDetails{} && len(ca.CloseSignatures) == 0 && len(ca.DeclarationSignatures) == 0
}

func (c *Channel) ProposePayment(amount int64) (CloseAgreement, error) {
	if amount <= 0 {
		return CloseAgreement{}, errors.New("payment amount must be greater than 0")
	}
	newBalance := int64(0)
	if c.initiator {
		newBalance = c.Balance() + amount
	} else {
		newBalance = c.Balance() - amount
	}
	if c.amountToRemote(newBalance) > c.localEscrowAccount.Balance {
		return CloseAgreement{}, fmt.Errorf("amount over commits: %w", ErrUnderfunded)
	}

	d := CloseAgreementDetails{
		ObservationPeriodTime:      c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime,
		ObservationPeriodLedgerGap: c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap,
		IterationNumber:            c.NextIterationNumber(),
		Balance:                    newBalance,
	}
	_, txClose, err := c.CloseTxs(d)
	if err != nil {
		return CloseAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, err
	}

	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:         d,
		CloseSignatures: txClose.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

var ErrUnderfunded = fmt.Errorf("account is underfunded to make payment")

// ConfirmPayment confirms a close agreement. The original proposer should only have to call this once, and the
// receiver should call twice. First to sign the agreement and store signatures, second to just store the new signatures
// from the other party's confirmation.
func (c *Channel) ConfirmPayment(ca CloseAgreement) (closeAgreement CloseAgreement, authorized bool, err error) {
	// If the close agreement details don't match the close agreement in progress, error.
	if ca.Details.IterationNumber != c.NextIterationNumber() {
		return ca, authorized, fmt.Errorf("invalid payment iteration number, got: %d want: %d", ca.Details.IterationNumber, c.NextIterationNumber())
	}
	if ca.Details.ObservationPeriodTime != c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime || ca.Details.ObservationPeriodLedgerGap != c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap {
		return ca, authorized, fmt.Errorf("invalid payment observation period: different than channel state")
	}
	if !c.latestUnauthorizedCloseAgreement.isEmpty() && c.latestUnauthorizedCloseAgreement.Details != ca.Details {
		return ca, authorized, errors.New("close agreement does not match the close agreement already in progress")
	}

	// If the agreement is signed by all participants at the end of this method,
	// promote the agreement to authorized. If not signed by all participants,
	// save it as the latest unauthorized agreement, as we are still in the
	// process of collecting signatures for it. If an error occurred during this
	// process don't save any new state, as something went wrong.
	defer func() {
		if err != nil {
			return
		}
		updatedCA := CloseAgreement{
			Details:               ca.Details,
			CloseSignatures:       appendNewSignatures(c.latestUnauthorizedCloseAgreement.CloseSignatures, ca.CloseSignatures),
			DeclarationSignatures: appendNewSignatures(c.latestUnauthorizedCloseAgreement.DeclarationSignatures, ca.DeclarationSignatures),
		}
		if authorized {
			c.latestUnauthorizedCloseAgreement = CloseAgreement{}
			c.latestAuthorizedCloseAgreement = updatedCA
		} else {
			c.latestUnauthorizedCloseAgreement = updatedCA
		}
	}()

	// create payment transactions
	txDecl, txClose, err := c.CloseTxs(ca.Details)
	if err != nil {
		return ca, authorized, err
	}

	// If remote has not signed close, error as is invalid.
	signed, err := c.verifySigned(txClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return ca, authorized, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return ca, authorized, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}

	// If local has not signed close, check that the payment is not to the proposer, then sign.
	signed, err = c.verifySigned(txClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return ca, authorized, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		if (c.initiator && ca.Details.Balance > c.latestAuthorizedCloseAgreement.Details.Balance) ||
			(!c.initiator && ca.Details.Balance < c.latestAuthorizedCloseAgreement.Details.Balance) {
			return ca, authorized, fmt.Errorf("close agreement is a payment to the proposer")
		}
		if c.amountToLocal(ca.Details.Balance) > c.remoteEscrowAccount.Balance {
			return ca, authorized, fmt.Errorf("close agreement over commits: %w", ErrUnderfunded)
		}
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return ca, authorized, fmt.Errorf("signing close with local: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txClose.Signatures()...)
	}

	// Local should always sign declaration if have not yet.
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.localSigner)
	if err != nil {
		return ca, authorized, fmt.Errorf("verifying declaration signed by local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return ca, authorized, err
		}
		ca.DeclarationSignatures = append(ca.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If remote has not signed declaration, it is incomplete.
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return ca, authorized, fmt.Errorf("verifying declaration signed by remote: %w", err)
	}
	if !signed {
		return ca, authorized, nil
	}

	// All signatures are present that would be required to submit all
	// transactions in the payment.
	authorized = true
	return ca, authorized, nil
}
