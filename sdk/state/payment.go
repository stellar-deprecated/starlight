package state

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/go/keypair"
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
	ConfirmingSigner           *keypair.FromAddress
}

func (d CloseAgreementDetails) Equal(d2 CloseAgreementDetails) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type CAD CloseAgreementDetails
	return cmp.Equal(CAD(d), CAD(d2), cmp.AllowUnexported(keypair.FromAddress{}))
}

// CloseAgreement contains everything a participant needs to execute the close
// agreement on the Stellar network.
type CloseAgreement struct {
	Details               CloseAgreementDetails
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
}

func (ca CloseAgreement) isEmpty() bool {
	return ca.Equal(CloseAgreement{})
}

func (ca CloseAgreement) Equal(ca2 CloseAgreement) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type CA CloseAgreement
	return cmp.Equal(CA(ca), CA(ca2), cmp.AllowUnexported(keypair.FromAddress{}))
}

func (c *Channel) ProposePayment(amount int64) (CloseAgreement, error) {
	if amount <= 0 {
		return CloseAgreement{}, errors.New("payment amount must be greater than 0")
	}

	// If a coordinated close has been accepted already, error.
	if c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return CloseAgreement{}, errors.New("cannot propose payment after an accepted coordinated close")
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
		ConfirmingSigner:           c.remoteSigner,
	}
	txDecl, txClose, err := c.closeTxs(c.openAgreement.Details, d)
	if err != nil {
		return CloseAgreement{}, err
	}
	txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, err
	}

	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:               d,
		CloseSignatures:       txClose.Signatures(),
		DeclarationSignatures: txDecl.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

var ErrUnderfunded = fmt.Errorf("account is underfunded to make payment")

// validatePayment validates the close agreement given to the ConfirmPayment method. Note that
// there are additional verifications ConfirmPayment performs that are based
// on the state of the close agreement signatures.
func (c *Channel) validatePayment(ca CloseAgreement) (err error) {
	if ca.Details.IterationNumber != c.NextIterationNumber() {
		return fmt.Errorf("invalid payment iteration number, got: %d want: %d", ca.Details.IterationNumber, c.NextIterationNumber())
	}
	if ca.Details.ObservationPeriodTime != c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime ||
		ca.Details.ObservationPeriodLedgerGap != c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap {
		return fmt.Errorf("invalid payment observation period: different than channel state")
	}
	if !c.latestUnauthorizedCloseAgreement.isEmpty() && !ca.Details.Equal(c.latestUnauthorizedCloseAgreement.Details) {
		return fmt.Errorf("close agreement does not match the close agreement already in progress")
	}
	if ca.Details.ConfirmingSigner.Address() != c.localSigner.Address() &&
		ca.Details.ConfirmingSigner.Address() != c.remoteSigner.Address() {
		return fmt.Errorf("close agreement confirmer does not match a local or remote signer, got: %s", ca.Details.ConfirmingSigner.Address())
	}
	if c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return fmt.Errorf("cannot confirm payment after an accepted coordinated close")
	}
	return nil
}

// ConfirmPayment confirms an agreement. The destination of a payment calls this
// once to sign and store the agreement. The source of a payment calls this once
// with a copy of the agreement signed by the destination to store the destination's signatures.
func (c *Channel) ConfirmPayment(ca CloseAgreement) (closeAgreement CloseAgreement, err error) {
	err = c.validatePayment(ca)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("validating payment: %w", err)
	}

	// create payment transactions
	txDecl, txClose, err := c.closeTxs(c.openAgreement.Details, ca.Details)
	if err != nil {
		return CloseAgreement{}, err
	}

	// If remote has not signed the txs, error as is invalid.
	signed, err := c.verifySigned(txClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signed by remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signed by remote: not signed by remote")
	}

	// If local has not signed close, check that the payment is not to the proposer, then sign.
	signed, err = c.verifySigned(txClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		if (c.initiator && ca.Details.Balance > c.latestAuthorizedCloseAgreement.Details.Balance) ||
			(!c.initiator && ca.Details.Balance < c.latestAuthorizedCloseAgreement.Details.Balance) {
			return CloseAgreement{}, fmt.Errorf("close agreement is a payment to the proposer")
		}
		if c.amountToLocal(ca.Details.Balance) > c.remoteEscrowAccount.Balance {
			return CloseAgreement{}, fmt.Errorf("close agreement over commits: %w", ErrUnderfunded)
		}
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fmt.Errorf("signing close with local: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txClose.Signatures()...)
	}

	// If local has not signed declaration, sign it.
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signed by local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, err
		}
		ca.DeclarationSignatures = append(ca.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If an agreement ever surpasses 2 signatures per tx, error.
	if len(ca.DeclarationSignatures) > 2 || len(ca.CloseSignatures) > 2 {
		return CloseAgreement{}, fmt.Errorf("close agreement has too many signatures,"+
			" has declaration: %d, close: %d, max of 2 allowed for each",
			len(ca.DeclarationSignatures), len(ca.CloseSignatures))
	}

	// All signatures are present that would be required to submit all
	// transactions in the payment.
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details:               ca.Details,
		CloseSignatures:       appendNewSignatures(c.latestUnauthorizedCloseAgreement.CloseSignatures, ca.CloseSignatures),
		DeclarationSignatures: appendNewSignatures(c.latestUnauthorizedCloseAgreement.DeclarationSignatures, ca.DeclarationSignatures),
	}
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}

	return c.latestAuthorizedCloseAgreement, nil
}
