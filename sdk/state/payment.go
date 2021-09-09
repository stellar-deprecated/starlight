package state

import (
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/go/keypair"
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
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	IterationNumber            int64
	Balance                    int64
	PaymentAmount              int64
	ProposingSigner            *keypair.FromAddress
	ConfirmingSigner           *keypair.FromAddress
}

func (d CloseAgreementDetails) Equal(d2 CloseAgreementDetails) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type CAD CloseAgreementDetails
	return cmp.Equal(CAD(d), CAD(d2))
}

type CloseSignatures struct {
	Close       xdr.Signature
	Declaration xdr.Signature
}

func signCloseAgreementTxs(txs CloseTransactions, networkPassphrase string, signer *keypair.Full) (s CloseSignatures, err error) {
	s.Declaration, err = signTx(txs.Declaration, networkPassphrase, signer)
	if err != nil {
		return CloseSignatures{}, fmt.Errorf("signing declaration: %w", err)
	}
	s.Close, err = signTx(txs.Close, networkPassphrase, signer)
	if err != nil {
		return CloseSignatures{}, fmt.Errorf("signing close: %w", err)
	}
	return s, nil
}

func (s CloseSignatures) Verify(txs CloseTransactions, networkPassphrase string, signer *keypair.FromAddress) error {
	err := verifySigned(txs.Declaration, networkPassphrase, signer, s.Declaration)
	if err != nil {
		return fmt.Errorf("verifying declaration signed: %w", err)
	}
	err = verifySigned(txs.Close, networkPassphrase, signer, s.Close)
	if err != nil {
		return fmt.Errorf("verifying close signed: %w", err)
	}
	return nil
}

// CloseTransactions contain all the transaction hashes and
// transactions for the transactions that make up the close agreement.
type CloseTransactions struct {
	CloseHash       TransactionHash
	Close           *txnbuild.Transaction
	DeclarationHash TransactionHash
	Declaration     *txnbuild.Transaction
}

// CloseAgreement contains everything a participant needs to execute the close
// agreement on the Stellar network.
type CloseAgreement struct {
	Details             CloseAgreementDetails
	ProposerSignatures  CloseSignatures
	ConfirmerSignatures CloseSignatures
}

func (ca CloseAgreement) isEmpty() bool {
	return ca.Equal(CloseAgreement{})
}

func (ca CloseAgreement) Equal(ca2 CloseAgreement) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type CA CloseAgreement
	return cmp.Equal(CA(ca), CA(ca2))
}

func (ca CloseAgreement) SignaturesFor(signer *keypair.FromAddress) *CloseSignatures {
	if ca.Details.ProposingSigner.Equal(signer) {
		return &ca.ProposerSignatures
	}
	if ca.Details.ConfirmingSigner.Equal(signer) {
		return &ca.ConfirmerSignatures
	}
	return nil
}

func (c *Channel) ProposePayment(amount int64) (CloseAgreement, error) {
	if amount <= 0 {
		return CloseAgreement{}, fmt.Errorf("payment amount must be greater than 0")
	}

	// If the channel is not open yet, error.
	if c.latestAuthorizedCloseAgreement.isEmpty() || !c.openExecutedAndValidated {
		return CloseAgreement{}, fmt.Errorf("cannot propose a payment before channel is opened")
	}

	// If a coordinated close has been accepted already, error.
	if !c.latestAuthorizedCloseAgreement.isEmpty() && c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return CloseAgreement{}, fmt.Errorf("cannot propose payment after an accepted coordinated close")
	}

	// If a coordinated close has been proposed by this channel already, error.
	if !c.latestUnauthorizedCloseAgreement.isEmpty() && c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return CloseAgreement{}, fmt.Errorf("cannot propose payment after proposing a coordinated close")
	}

	// If an unfinished unauthorized agreement exists, error.
	if !c.latestUnauthorizedCloseAgreement.isEmpty() {
		return CloseAgreement{}, fmt.Errorf("cannot start a new payment while an unfinished one exists")
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
		PaymentAmount:              amount,
		ProposingSigner:            c.localSigner.FromAddress(),
		ConfirmingSigner:           c.remoteSigner,
	}
	txs, err := c.closeTxs(c.openAgreement.Details, d)
	if err != nil {
		return CloseAgreement{}, err
	}
	sigs, err := signCloseAgreementTxs(txs, c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing open agreement with local: %w", err)
	}

	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:            d,
		ProposerSignatures: sigs,
	}
	c.latestUnauthorizedCloseTransactions = txs
	return c.latestUnauthorizedCloseAgreement, nil
}

var ErrUnderfunded = fmt.Errorf("account is underfunded to make payment")

// validatePayment validates the close agreement given to the ConfirmPayment method. Note that
// there are additional verifications ConfirmPayment performs that are based
// on the state of the close agreement signatures.
func (c *Channel) validatePayment(ca CloseAgreement) (err error) {
	// If the channel is not open yet, error.
	if c.latestAuthorizedCloseAgreement.isEmpty() || !c.openExecutedAndValidated {
		return fmt.Errorf("cannot confirm a payment before channel is opened")
	}

	// If a coordinated close has been proposed by this channel already, error.
	if !c.latestUnauthorizedCloseAgreement.isEmpty() && c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return fmt.Errorf("cannot confirm payment after proposing a coordinated close")
	}

	// If a coordinated close has been accepted already, error.
	if !c.latestAuthorizedCloseAgreement.isEmpty() && c.latestAuthorizedCloseAgreement.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Details.ObservationPeriodLedgerGap == 0 {
		return fmt.Errorf("cannot confirm payment after an accepted coordinated close")
	}

	// If the new close agreement details are incorrect, error.
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
	if !ca.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) && !ca.Details.ConfirmingSigner.Equal(c.remoteSigner) {
		return fmt.Errorf("close agreement confirmer does not match a local or remote signer, got: %s", ca.Details.ConfirmingSigner.Address())
	}
	if !ca.Details.ProposingSigner.Equal(c.localSigner.FromAddress()) && !ca.Details.ProposingSigner.Equal(c.remoteSigner) {
		return fmt.Errorf("close agreement proposer does not match a local or remote signer, got: %s", ca.Details.ProposingSigner.Address())
	}

	// If the close agreement payment amount is incorrect, error.
	pa := ca.Details.PaymentAmount
	proposerIsResponder := ca.Details.ProposingSigner.Equal(c.responderSigner())
	if proposerIsResponder {
		pa = ca.Details.PaymentAmount * -1
	}
	if c.Balance()+pa != ca.Details.Balance {
		return fmt.Errorf("close agreement payment amount is unexpected: current balance: %d proposed balance: %d payment amount: %d initiator proposed: %t",
			c.Balance(), ca.Details.Balance, ca.Details.PaymentAmount, !proposerIsResponder)
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
	txs, err := c.closeTxs(c.openAgreement.Details, ca.Details)
	if err != nil {
		return CloseAgreement{}, err
	}

	// If remote has not signed the txs, error as is invalid.
	remoteSigs := ca.SignaturesFor(c.remoteSigner)
	if remoteSigs == nil {
		return CloseAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = remoteSigs.Verify(txs, c.networkPassphrase, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("not signed by remote: %w", err)
	}

	// If local has not signed close, check that the payment is not to the proposer, then sign.
	localSigs := ca.SignaturesFor(c.localSigner.FromAddress())
	if localSigs == nil {
		return CloseAgreement{}, fmt.Errorf("local is not a signer")
	}
	err = localSigs.Verify(txs, c.networkPassphrase, c.localSigner.FromAddress())
	if err != nil {
		// If the local is not the confirmer, do not sign, because being the
		// proposer they should have signed earlier.
		if !ca.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) {
			return CloseAgreement{}, fmt.Errorf("not signed by local: %w", err)
		}
		// If the payment is to the proposer, error, because the payment channel
		// only supports pushing money to the other participant not pulling.
		if (c.initiator && ca.Details.Balance > c.latestAuthorizedCloseAgreement.Details.Balance) ||
			(!c.initiator && ca.Details.Balance < c.latestAuthorizedCloseAgreement.Details.Balance) {
			return CloseAgreement{}, fmt.Errorf("close agreement is a payment to the proposer")
		}
		// If the payment over extends the proposers ability to pay, error.
		if c.amountToLocal(ca.Details.Balance) > c.remoteEscrowAccount.Balance {
			return CloseAgreement{}, fmt.Errorf("close agreement over commits: %w", ErrUnderfunded)
		}
		ca.ConfirmerSignatures, err = signCloseAgreementTxs(txs, c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fmt.Errorf("local signing: %w", err)
		}
	}

	// All signatures are present that would be required to submit all
	// transactions in the payment.
	c.latestAuthorizedCloseAgreement = ca
	c.latestAuthorizedCloseTransactions = txs
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}
	c.latestUnauthorizedCloseTransactions = CloseTransactions{}

	return c.latestAuthorizedCloseAgreement, nil
}
