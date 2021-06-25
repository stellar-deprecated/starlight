package state

import (
	"errors"
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
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
	IterationNumber int64
	Balance         Amount
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

func (c *Channel) ProposePayment(amount Amount) (CloseAgreement, error) {
	if amount.Amount <= 0 {
		return CloseAgreement{}, errors.New("payment amount must be greater than 0")
	}
	if amount.Asset != c.latestAuthorizedCloseAgreement.Details.Balance.Asset {
		return CloseAgreement{}, fmt.Errorf("payment asset type is invalid, got: %s want: %s",
			amount.Asset, c.latestAuthorizedCloseAgreement.Details.Balance.Asset)
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
		Asset:                      amount.Asset,
	})
	if err != nil {
		return CloseAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, err
	}

	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber: c.NextIterationNumber(),
			Balance:         Amount{Asset: amount.Asset, Amount: newBalance},
		},
		CloseSignatures: txClose.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

func (c *Channel) PaymentTxs(ca CloseAgreement) (close, decl *txnbuild.Transaction, err error) {
	close, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.NextIterationNumber(),
		AmountToInitiator:          maxInt64(0, ca.Details.Balance.Amount*-1),
		AmountToResponder:          maxInt64(0, ca.Details.Balance.Amount),
		Asset:                      ca.Details.Balance.Asset,
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

// ConfirmPayment confirms a close agreement. The original proposer should only have to call this once, and the
// receiver should call twice. First to sign the agreement and store signatures, second to just store the new signatures
// from the other party's confirmation.
func (c *Channel) ConfirmPayment(ca CloseAgreement) (closeAgreement CloseAgreement, authorized bool, err error) {
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
			Details:       ca.Details,
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

	// validate payment
	if ca.Details.IterationNumber != c.NextIterationNumber() {
		return ca, authorized, fmt.Errorf("invalid payment iteration number, got: %d want: %d", ca.Details.IterationNumber, c.NextIterationNumber())
	}
	if !c.latestUnauthorizedCloseAgreement.isEmpty() && c.latestUnauthorizedCloseAgreement.Details != ca.Details {
		return ca, authorized, errors.New("close agreement does not match the close agreement already in progress")
	}

	if ca.Details.Balance.Asset != c.latestAuthorizedCloseAgreement.Details.Balance.Asset {
		return ca, authorized, fmt.Errorf("payment asset type is invalid, got: %s want: %s",
			ca.Details.Balance.Asset, c.latestAuthorizedCloseAgreement.Details.Balance.Asset)
	}

	// create payment transactions
	txClose, txDecl, err := c.PaymentTxs(ca)
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

	// If local has not signed close, sign.
	signed, err = c.verifySigned(txClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return ca, authorized, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
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

func maxInt64(x int64, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
