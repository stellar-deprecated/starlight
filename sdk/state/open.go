package state

import (
	"bytes"
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type OpenDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	ExpiresAt                  time.Time
	StartingSequence           int64
	ProposingSigner            *keypair.FromAddress
	ConfirmingSigner           *keypair.FromAddress
}

func (d OpenDetails) Equal(d2 OpenDetails) bool {
	return d.ObservationPeriodTime == d2.ObservationPeriodTime &&
		d.ObservationPeriodLedgerGap == d2.ObservationPeriodLedgerGap &&
		d.Asset == d2.Asset &&
		d.ExpiresAt.Equal(d2.ExpiresAt) &&
		d.StartingSequence == d2.StartingSequence &&
		d.ProposingSigner.Equal(d2.ProposingSigner) &&
		d.ConfirmingSigner.Equal(d2.ConfirmingSigner)
}

type OpenSignatures struct {
	Close       xdr.Signature
	Declaration xdr.Signature
	Open        xdr.Signature
}

func (oas OpenSignatures) Empty() bool {
	return len(oas.Declaration) == 0 && len(oas.Close) == 0 && len(oas.Open) == 0
}

func (oas OpenSignatures) HasAllSignatures() bool {
	return len(oas.Close) != 0 && len(oas.Declaration) != 0 && len(oas.Open) != 0
}

func (oas OpenSignatures) Equal(oas2 OpenSignatures) bool {
	return bytes.Equal(oas.Open, oas2.Open) &&
		bytes.Equal(oas.Declaration, oas2.Declaration) &&
		bytes.Equal(oas.Close, oas2.Close)
}

func signOpenAgreementTxs(txs OpenTransactions, closeTxs CloseTransactions, signer *keypair.Full) (s OpenSignatures, err error) {
	s.Declaration, err = signer.Sign(closeTxs.DeclarationHash[:])
	if err != nil {
		return OpenSignatures{}, fmt.Errorf("signing declaration: %w", err)
	}
	s.Close, err = signer.Sign(closeTxs.CloseHash[:])
	if err != nil {
		return OpenSignatures{}, fmt.Errorf("signing close: %w", err)
	}
	s.Open, err = signer.Sign(txs.OpenHash[:])
	if err != nil {
		return OpenSignatures{}, fmt.Errorf("signing open: %w", err)
	}
	return s, nil
}

func (s OpenSignatures) Verify(txs OpenTransactions, closeTxs CloseTransactions, signer *keypair.FromAddress) error {
	err := signer.Verify(closeTxs.DeclarationHash[:], s.Declaration)
	if err != nil {
		return fmt.Errorf("verifying declaration signed: %w", err)
	}
	err = signer.Verify(closeTxs.CloseHash[:], s.Close)
	if err != nil {
		return fmt.Errorf("verifying close signed: %w", err)
	}
	err = signer.Verify(txs.OpenHash[:], s.Open)
	if err != nil {
		return fmt.Errorf("verifying open signed: %w", err)
	}
	return nil
}

// OpenTransactions contain all the transaction hashes and transactions
// that make up the open agreement.
type OpenTransactions struct {
	OpenHash TransactionHash
	Open     *txnbuild.Transaction
}

type OpenEnvelope struct {
	Details             OpenDetails
	ProposerSignatures  OpenSignatures
	ConfirmerSignatures OpenSignatures
}

func (oa OpenEnvelope) Empty() bool {
	return oa.Equal(OpenEnvelope{})
}

// HasAllSignatures checks if the open agreement has the max amount of
// signatures, indicating it is fully signed by all parties.
func (oa OpenEnvelope) HasAllSignatures() bool {
	return oa.ProposerSignatures.HasAllSignatures() && oa.ConfirmerSignatures.HasAllSignatures()
}

func (oa OpenEnvelope) Equal(oa2 OpenEnvelope) bool {
	return oa.Details.Equal(oa2.Details) &&
		oa.ProposerSignatures.Equal(oa2.ProposerSignatures) &&
		oa.ConfirmerSignatures.Equal(oa2.ConfirmerSignatures)
}

func (oa OpenEnvelope) SignaturesFor(signer *keypair.FromAddress) *OpenSignatures {
	if oa.Details.ProposingSigner.Equal(signer) {
		return &oa.ProposerSignatures
	}
	if oa.Details.ConfirmingSigner.Equal(signer) {
		return &oa.ConfirmerSignatures
	}
	return nil
}

// CloseEnvelope gets the equivalent CloseEnvelope for this OpenEnvelope.
func (oe OpenEnvelope) CloseEnvelope() CloseEnvelope {
	return CloseEnvelope{
		Details: CloseDetails{
			IterationNumber:            1,
			Balance:                    0,
			ObservationPeriodTime:      oe.Details.ObservationPeriodTime,
			ObservationPeriodLedgerGap: oe.Details.ObservationPeriodLedgerGap,
			ProposingSigner:            oe.Details.ProposingSigner,
			ConfirmingSigner:           oe.Details.ConfirmingSigner,
		},
		ProposerSignatures: CloseSignatures{
			Declaration: oe.ProposerSignatures.Declaration,
			Close:       oe.ProposerSignatures.Close,
		},
		ConfirmerSignatures: CloseSignatures{
			Declaration: oe.ConfirmerSignatures.Declaration,
			Close:       oe.ConfirmerSignatures.Close,
		},
	}
}

// OpenAgreement contains all the information known for an agreement proposed or
// confirmed by the channel.
type OpenAgreement struct {
	Envelope          OpenEnvelope
	Transactions      OpenTransactions
	CloseTransactions CloseTransactions
}

func (oa OpenAgreement) CloseAgreement() CloseAgreement {
	return CloseAgreement{
		Envelope:     oa.Envelope.CloseEnvelope(),
		Transactions: oa.CloseTransactions,
	}
}

func (oa OpenAgreement) SignedTransactions() OpenTransactions {
	openTx := oa.Transactions.Open

	// Add the open signatures to the open tx.
	openTx, _ = openTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.Envelope.ProposerSignatures.Open, oa.Envelope.Details.ProposingSigner.Hint()))
	openTx, _ = openTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.Envelope.ConfirmerSignatures.Open, oa.Envelope.Details.ConfirmingSigner.Hint()))

	// Add the declaration signature provided by the confirming signer that is
	// required to be an extra signer on the open tx to the open tx.
	openTx, _ = openTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.Envelope.ConfirmerSignatures.Declaration, oa.Envelope.Details.ConfirmingSigner.Hint(), oa.CloseTransactions.DeclarationHash[:]))

	// Add the close signature provided by the confirming signer that is
	// required to be an extra signer on the open tx to the open tx.
	openTx, _ = openTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.Envelope.ConfirmerSignatures.Close, oa.Envelope.Details.ConfirmingSigner.Hint(), oa.CloseTransactions.CloseHash[:]))

	return OpenTransactions{
		OpenHash: oa.Transactions.OpenHash,
		Open:     openTx,
	}
}

// OpenParams are the parameters selected by the participant proposing an open channel.
type OpenParams struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	ExpiresAt                  time.Time
	StartingSequence           int64
}

// openTxs builds the transactions that embody the open agreement that can be
// submitted to open the channel with the state defined in the
// OpenAgreementDetails, and includes the first close agreement transactions. If
// the channel has previous build the open transactions then it will return
// those previously built transactions, otherwise the transactions will be built
// from scratch.
func (c *Channel) openTxs(d OpenDetails) (txs OpenTransactions, closeTxs CloseTransactions, err error) {
	if c.openAgreement.Envelope.Details.Equal(d) {
		return c.openAgreement.Transactions, c.openAgreement.CloseTransactions, nil
	}
	cad := CloseDetails{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		IterationNumber:            1,
		Balance:                    0,
		ConfirmingSigner:           d.ConfirmingSigner,
	}

	closeTxs, err = c.closeTxs(d, cad)
	if err != nil {
		err = fmt.Errorf("building close txs for open: %w", err)
		return
	}

	open, err := txbuild.Open(txbuild.OpenParams{
		InitiatorSigner:   c.initiatorSigner(),
		ResponderSigner:   c.responderSigner(),
		InitiatorMultisig: c.initiatorMultisigAccount().Address,
		ResponderMultisig: c.responderMultisigAccount().Address,
		StartSequence:     d.StartingSequence,
		Asset:             d.Asset.Asset(),
		ExpiresAt:         d.ExpiresAt,
		DeclarationTxHash: closeTxs.DeclarationHash,
		CloseTxHash:       closeTxs.CloseHash,
		ConfirmingSigner:  d.ConfirmingSigner,
	})
	if err != nil {
		err = fmt.Errorf("building open tx for open: %w", err)
		return
	}
	openHash, err := open.Hash(c.networkPassphrase)
	if err != nil {
		err = fmt.Errorf("hashing open tx: %w", err)
		return
	}

	txs = OpenTransactions{
		OpenHash: openHash,
		Open:     open,
	}
	return
}

// OpenTx builds the open transaction used for opening the channel. The
// transaction is signed and ready to submit. ProposeOpen and ConfirmOpen must
// be used prior to prepare an open agreement with the other participant.
func (c *Channel) OpenTx() (openTx *txnbuild.Transaction, err error) {
	oa := c.openAgreement
	txs := oa.SignedTransactions()
	return txs.Open, nil
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	// if the channel is already opening, error.
	if !c.openAgreement.Envelope.Empty() {
		return OpenAgreement{}, fmt.Errorf("cannot propose a new open if channel is already opening or already open")
	}

	d := OpenDetails{
		ObservationPeriodTime:      p.ObservationPeriodTime,
		ObservationPeriodLedgerGap: p.ObservationPeriodLedgerGap,
		Asset:                      p.Asset,
		ExpiresAt:                  p.ExpiresAt,
		StartingSequence:           p.StartingSequence,
		ProposingSigner:            c.localSigner.FromAddress(),
		ConfirmingSigner:           c.remoteSigner,
	}

	txs, closeTxs, err := c.openTxs(d)
	if err != nil {
		return OpenAgreement{}, err
	}
	sigs, err := signOpenAgreementTxs(txs, closeTxs, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("signing open agreement with local: %w", err)
	}
	open := OpenAgreement{
		Envelope: OpenEnvelope{
			Details:            d,
			ProposerSignatures: sigs,
		},
		Transactions:      txs,
		CloseTransactions: closeTxs,
	}
	c.openAgreement = open
	return open, nil
}

func (c *Channel) validateOpen(m OpenEnvelope) error {
	// if the channel is already open, error.
	if c.openAgreement.Envelope.HasAllSignatures() {
		return fmt.Errorf("cannot confirm a new open if channel is already opened")
	}

	// If the open agreement details don't match the open agreement in progress, error.
	if !c.openAgreement.Envelope.Empty() && !m.Details.Equal(c.openAgreement.Envelope.Details) {
		return fmt.Errorf("input open agreement details do not match the saved open agreement details")
	}

	// If the expiry of the agreement is past the max expiry the channel will accept, error.
	if m.Details.ExpiresAt.After(time.Now().Add(c.maxOpenExpiry)) {
		return fmt.Errorf("input open agreement expire too far into the future")
	}

	return nil
}

// ConfirmOpen confirms an open that was proposed.  ConfirmPayment confirms the
// agreement. The responder to the open process calls this once to sign and
// store the agreement. The initiator of the open process calls this once with a
// copy of the agreement signed by the destination to store the destination's signatures.
func (c *Channel) ConfirmOpen(m OpenEnvelope) (open OpenAgreement, err error) {
	err = c.validateOpen(m)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("validating open agreement: %w", err)
	}

	txs, closeTxs, err := c.openTxs(m.Details)
	if err != nil {
		return OpenAgreement{}, err
	}

	// If remote has not signed the txs, error as is invalid.
	remoteSigs := m.SignaturesFor(c.remoteSigner)
	if remoteSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = remoteSigs.Verify(txs, closeTxs, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("not signed by remote: %w", err)
	}

	// If local has not signed the txs, sign them.
	localSigs := m.SignaturesFor(c.localSigner.FromAddress())
	if localSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = localSigs.Verify(txs, closeTxs, c.localSigner.FromAddress())
	if err != nil {
		// If the local is not the confirmer, do not sign, because being the
		// proposer they should have signed earlier.
		if !m.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) {
			return OpenAgreement{}, fmt.Errorf("not signed by local: %w", err)
		}
		m.ConfirmerSignatures, err = signOpenAgreementTxs(txs, closeTxs, c.localSigner)
		if err != nil {
			return OpenAgreement{}, fmt.Errorf("local signing: %w", err)
		}
	}

	// All signatures are present that would be required to submit all
	// transactions in the open.
	c.openAgreement = OpenAgreement{
		Envelope:          m,
		Transactions:      txs,
		CloseTransactions: closeTxs,
	}
	c.latestAuthorizedCloseAgreement = c.openAgreement.CloseAgreement()
	return c.openAgreement, nil
}
