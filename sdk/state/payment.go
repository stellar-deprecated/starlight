package state

import (
	"bytes"
	"fmt"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"golang.org/x/sync/errgroup"
)

// The high level steps for creating a channel update should be as follows, where the returned payments
// flow to the next step:
// 1. Sender calls ProposePayment
// 2. Receiver calls ConfirmPayment
// 3. Sender calls ConfirmPayment
// 4. Receiver calls ConfirmPayment

// CloseDetails contains the details that the participants agree on.
type CloseDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	IterationNumber            int64
	Balance                    int64
	PaymentAmount              int64
	ProposingSigner            *keypair.FromAddress
	ConfirmingSigner           *keypair.FromAddress
}

func (d CloseDetails) Equal(d2 CloseDetails) bool {
	return d.ObservationPeriodTime == d2.ObservationPeriodTime &&
		d.ObservationPeriodLedgerGap == d2.ObservationPeriodLedgerGap &&
		d.IterationNumber == d2.IterationNumber &&
		d.Balance == d2.Balance &&
		d.PaymentAmount == d2.PaymentAmount &&
		d.ProposingSigner.Equal(d2.ProposingSigner) &&
		d.ConfirmingSigner.Equal(d2.ConfirmingSigner)
}

type CloseSignatures struct {
	Close       xdr.Signature
	Declaration xdr.Signature
}

func (cas CloseSignatures) Empty() bool {
	return len(cas.Declaration) == 0 && len(cas.Close) == 0
}

func (cas CloseSignatures) HasAllSignatures() bool {
	return len(cas.Close) != 0 && len(cas.Declaration) != 0
}

func (cas CloseSignatures) Equal(cas2 CloseSignatures) bool {
	return bytes.Equal(cas.Declaration, cas2.Declaration) &&
		bytes.Equal(cas.Close, cas2.Close)
}

func signCloseAgreementTxs(txs CloseTransactions, signer *keypair.Full) (s CloseSignatures, err error) {
	g := errgroup.Group{}
	g.Go(func() error {
		var err error
		s.Declaration, err = signer.Sign(txs.DeclarationHash[:])
		return err
	})
	g.Go(func() error {
		var err error
		s.Close, err = signer.Sign(txs.CloseHash[:])
		return err
	})
	return s, g.Wait()
}

// CloseTransactions contain all the transaction hashes and
// transactions for the transactions that make up the close agreement.
type CloseTransactions struct {
	CloseHash       TransactionHash
	Close           *txnbuild.Transaction
	DeclarationHash TransactionHash
	Declaration     *txnbuild.Transaction
}

// CloseEnvelope contains everything a participant needs to execute the close
// agreement on the Stellar network.
type CloseEnvelope struct {
	Details             CloseDetails
	ProposerSignatures  CloseSignatures
	ConfirmerSignatures CloseSignatures
}

func (ca CloseEnvelope) Empty() bool {
	return ca.Equal(CloseEnvelope{})
}

func (ca CloseEnvelope) Equal(ca2 CloseEnvelope) bool {
	return ca.Details.Equal(ca2.Details) &&
		ca.ProposerSignatures.Equal(ca2.ProposerSignatures) &&
		ca.ConfirmerSignatures.Equal(ca2.ConfirmerSignatures)
}

func (ca CloseEnvelope) SignaturesFor(signer *keypair.FromAddress) *CloseSignatures {
	if ca.Details.ProposingSigner.Equal(signer) {
		return &ca.ProposerSignatures
	}
	if ca.Details.ConfirmingSigner.Equal(signer) {
		return &ca.ConfirmerSignatures
	}
	return nil
}

// CloseAgreement contains all the information known for an agreement proposed
// or confirmed by the channel.
type CloseAgreement struct {
	Envelope     CloseEnvelope
	Transactions CloseTransactions
}

// SignedTransactions adds signatures from the CloseAgreement's Envelope to its
// Transactions.
func (ca CloseAgreement) SignedTransactions() CloseTransactions {
	declTx := ca.Transactions.Declaration
	closeTx := ca.Transactions.Close

	// Add the signatures that are from the proposer.
	declTx, _ = declTx.AddSignatureDecorated(xdr.NewDecoratedSignature(ca.Envelope.ProposerSignatures.Declaration, ca.Envelope.Details.ProposingSigner.Hint()))
	closeTx, _ = closeTx.AddSignatureDecorated(xdr.NewDecoratedSignature(ca.Envelope.ProposerSignatures.Close, ca.Envelope.Details.ProposingSigner.Hint()))

	// Add signatures that are from the confirmer.
	if ca.Envelope.ConfirmerSignatures.Declaration != nil {
		declTx, _ = declTx.AddSignatureDecorated(xdr.NewDecoratedSignature(ca.Envelope.ConfirmerSignatures.Declaration, ca.Envelope.Details.ConfirmingSigner.Hint()))
	}
	if ca.Envelope.ConfirmerSignatures.Close != nil {
		closeTx, _ = closeTx.AddSignatureDecorated(xdr.NewDecoratedSignature(ca.Envelope.ConfirmerSignatures.Close, ca.Envelope.Details.ConfirmingSigner.Hint()))

		// Add the close signature provided by the confirming signer that is
		// required to be an extra signer on the declaration tx to the formation tx.
		declTx, _ = declTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(ca.Envelope.ConfirmerSignatures.Close, ca.Envelope.Details.ConfirmingSigner.Hint(), ca.Transactions.CloseHash[:]))
	}

	return CloseTransactions{
		DeclarationHash: ca.Transactions.DeclarationHash,
		Declaration:     declTx,
		CloseHash:       ca.Transactions.CloseHash,
		Close:           closeTx,
	}
}

func (c *Channel) ProposePayment(amount int64) (CloseAgreement, error) {
	if amount < 0 {
		return CloseAgreement{}, fmt.Errorf("payment amount must not be less than 0")
	}

	// If the channel is not open yet, error.
	if c.latestAuthorizedCloseAgreement.Envelope.Empty() || !c.openExecutedAndValidated {
		return CloseAgreement{}, fmt.Errorf("cannot propose a payment before channel is opened")
	}

	// If a coordinated close has been accepted already, error.
	if !c.latestAuthorizedCloseAgreement.Envelope.Empty() && c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap == 0 {
		return CloseAgreement{}, fmt.Errorf("cannot propose payment after an accepted coordinated close")
	}

	// If a coordinated close has been proposed by this channel already, error.
	if !c.latestUnauthorizedCloseAgreement.Envelope.Empty() && c.latestUnauthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime == 0 &&
		c.latestUnauthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap == 0 {
		return CloseAgreement{}, fmt.Errorf("cannot propose payment after proposing a coordinated close")
	}

	// If an unfinished unauthorized agreement exists, error.
	if !c.latestUnauthorizedCloseAgreement.Envelope.Empty() {
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

	d := CloseDetails{
		ObservationPeriodTime:      c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime,
		ObservationPeriodLedgerGap: c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap,
		IterationNumber:            c.NextIterationNumber(),
		Balance:                    newBalance,
		PaymentAmount:              amount,
		ProposingSigner:            c.localSigner.FromAddress(),
		ConfirmingSigner:           c.remoteSigner,
	}
	txs, err := c.closeTxs(c.openAgreement.Envelope.Details, d)
	if err != nil {
		return CloseAgreement{}, err
	}
	sigs, err := signCloseAgreementTxs(txs, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing open agreement with local: %w", err)
	}

	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details:            d,
			ProposerSignatures: sigs,
		},
		Transactions: txs,
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

var ErrUnderfunded = fmt.Errorf("account is underfunded to make payment")

// validatePayment validates the close agreement given to the ConfirmPayment method. Note that
// there are additional verifications ConfirmPayment performs that are based
// on the state of the close agreement signatures.
func (c *Channel) validatePayment(ce CloseEnvelope) (err error) {
	// If the channel is not open yet, error.
	if c.latestAuthorizedCloseAgreement.Envelope.Empty() || !c.openExecutedAndValidated {
		return fmt.Errorf("cannot confirm a payment before channel is opened")
	}

	// If a coordinated close has been proposed by this channel already, error.
	if !c.latestUnauthorizedCloseAgreement.Envelope.Empty() && c.latestUnauthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime == 0 &&
		c.latestUnauthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap == 0 {
		return fmt.Errorf("cannot confirm payment after proposing a coordinated close")
	}

	// If a coordinated close has been accepted already, error.
	if !c.latestAuthorizedCloseAgreement.Envelope.Empty() && c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime == 0 &&
		c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap == 0 {
		return fmt.Errorf("cannot confirm payment after an accepted coordinated close")
	}

	// If the new close agreement details are incorrect, error.
	if ce.Details.IterationNumber != c.NextIterationNumber() {
		return fmt.Errorf("invalid payment iteration number, got: %d want: %d", ce.Details.IterationNumber, c.NextIterationNumber())
	}
	if ce.Details.ObservationPeriodTime != c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodTime ||
		ce.Details.ObservationPeriodLedgerGap != c.latestAuthorizedCloseAgreement.Envelope.Details.ObservationPeriodLedgerGap {
		return fmt.Errorf("invalid payment observation period: different than channel state")
	}
	if !c.latestUnauthorizedCloseAgreement.Envelope.Empty() && !ce.Details.Equal(c.latestUnauthorizedCloseAgreement.Envelope.Details) {
		return fmt.Errorf("close agreement does not match the close agreement already in progress")
	}
	if !ce.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) && !ce.Details.ConfirmingSigner.Equal(c.remoteSigner) {
		return fmt.Errorf("close agreement confirmer does not match a local or remote signer, got: %s", ce.Details.ConfirmingSigner.Address())
	}
	if !ce.Details.ProposingSigner.Equal(c.localSigner.FromAddress()) && !ce.Details.ProposingSigner.Equal(c.remoteSigner) {
		return fmt.Errorf("close agreement proposer does not match a local or remote signer, got: %s", ce.Details.ProposingSigner.Address())
	}

	// If the close agreement payment amount is incorrect, error.
	pa := ce.Details.PaymentAmount
	proposerIsResponder := ce.Details.ProposingSigner.Equal(c.responderSigner())
	if proposerIsResponder {
		pa = ce.Details.PaymentAmount * -1
	}
	if c.Balance()+pa != ce.Details.Balance {
		return fmt.Errorf("close agreement payment amount is unexpected: current balance: %d proposed balance: %d payment amount: %d initiator proposed: %t",
			c.Balance(), ce.Details.Balance, ce.Details.PaymentAmount, !proposerIsResponder)
	}
	return nil
}

// ConfirmPayment confirms an agreement. The destination of a payment calls this
// once to sign and store the agreement. The source of a payment calls this once
// with a copy of the agreement signed by the destination to store the destination's signatures.
func (c *Channel) ConfirmPayment(ce CloseEnvelope) (closeAgreement CloseAgreement, err error) {
	err = c.validatePayment(ce)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("validating payment: %w", err)
	}

	// create payment transactions
	txs, err := c.closeTxs(c.openAgreement.Envelope.Details, ce.Details)
	if err != nil {
		return CloseAgreement{}, err
	}

	remoteSigs := ce.SignaturesFor(c.remoteSigner)
	if remoteSigs == nil {
		return CloseAgreement{}, fmt.Errorf("remote is not a signer")
	}

	localSigs := ce.SignaturesFor(c.localSigner.FromAddress())
	if localSigs == nil {
		return CloseAgreement{}, fmt.Errorf("local is not a signer")
	}

	// If remote has not signed the txs or signatures is invalid, or the local
	// signatures if present are invalid, error as is invalid.
	verifyInputs := []signatureVerificationInput{
		{TransactionHash: txs.DeclarationHash, Signature: remoteSigs.Declaration, Signer: c.remoteSigner},
		{TransactionHash: txs.CloseHash, Signature: remoteSigs.Close, Signer: c.remoteSigner},
	}
	if !localSigs.Empty() {
		verifyInputs = append(verifyInputs, []signatureVerificationInput{
			{TransactionHash: txs.DeclarationHash, Signature: localSigs.Declaration, Signer: c.localSigner.FromAddress()},
			{TransactionHash: txs.CloseHash, Signature: localSigs.Close, Signer: c.localSigner.FromAddress()},
		}...)
	}
	err = verifySignatures(verifyInputs)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("invalid signature: %w", err)
	}

	// If local has not signed close, check that the payment is not to the proposer, then sign.
	if localSigs.Empty() {
		// If the local is not the confirmer, do not sign, because being the
		// proposer they should have signed earlier.
		if !ce.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) {
			return CloseAgreement{}, fmt.Errorf("not signed by local")
		}
		// If the payment is to the proposer, error, because the payment channel
		// only supports pushing money to the other participant not pulling.
		if (c.initiator && ce.Details.Balance > c.latestAuthorizedCloseAgreement.Envelope.Details.Balance) ||
			(!c.initiator && ce.Details.Balance < c.latestAuthorizedCloseAgreement.Envelope.Details.Balance) {
			return CloseAgreement{}, fmt.Errorf("close agreement is a payment to the proposer")
		}
		// If the payment over extends the proposers ability to pay, error.
		if c.amountToLocal(ce.Details.Balance) > c.remoteEscrowAccount.Balance {
			return CloseAgreement{}, fmt.Errorf("close agreement over commits: %w", ErrUnderfunded)
		}
		ce.ConfirmerSignatures, err = signCloseAgreementTxs(txs, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fmt.Errorf("local signing: %w", err)
		}
	}

	// All signatures are present that would be required to submit all
	// transactions in the payment.
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope:     ce,
		Transactions: txs,
	}
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}

	return c.latestAuthorizedCloseAgreement, nil
}
