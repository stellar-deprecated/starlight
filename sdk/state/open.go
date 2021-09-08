package state

import (
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type OpenAgreementDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	ExpiresAt                  time.Time
	StartingSequence           int64
	ProposingSigner            *keypair.FromAddress
	ConfirmingSigner           *keypair.FromAddress
}

func (d OpenAgreementDetails) Equal(d2 OpenAgreementDetails) bool {
	return d.ObservationPeriodTime == d2.ObservationPeriodTime &&
		d.ObservationPeriodLedgerGap == d2.ObservationPeriodLedgerGap &&
		d.Asset == d2.Asset &&
		d.ExpiresAt.Equal(d2.ExpiresAt) &&
		d.StartingSequence == d2.StartingSequence &&
		d.ProposingSigner.Equal(d2.ProposingSigner) &&
		d.ConfirmingSigner.Equal(d2.ConfirmingSigner)
}

type OpenAgreementSignatures struct {
	Close       xdr.Signature
	Declaration xdr.Signature
	Formation   xdr.Signature
}

func (oas OpenAgreementSignatures) isFull() bool {
	return len(oas.Close) > 0 && len(oas.Declaration) > 0 && len(oas.Formation) > 0
}

func signOpenAgreementTxs(txs OpenAgreementTransactions, networkPassphrase string, signer *keypair.Full) (s OpenAgreementSignatures, err error) {
	s.Declaration, err = signTx(txs.Declaration, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing declaration: %w", err)
	}
	s.Close, err = signTx(txs.Close, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing close: %w", err)
	}
	s.Formation, err = signTx(txs.Formation, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing formation: %w", err)
	}
	return s, nil
}

func (s OpenAgreementSignatures) Verify(txs OpenAgreementTransactions, networkPassphrase string, signer *keypair.FromAddress) error {
	err := verifySigned(txs.Declaration, networkPassphrase, signer, s.Declaration)
	if err != nil {
		return fmt.Errorf("verifying declaration signed: %w", err)
	}
	err = verifySigned(txs.Close, networkPassphrase, signer, s.Close)
	if err != nil {
		return fmt.Errorf("verifying close signed: %w", err)
	}
	err = verifySigned(txs.Formation, networkPassphrase, signer, s.Formation)
	if err != nil {
		return fmt.Errorf("verifying formation signed: %w", err)
	}
	return nil
}

// OpenAgreementTransactions contain all the transaction hashes and transactions
// that make up the open agreement.
type OpenAgreementTransactions struct {
	CloseHash       TransactionHash
	Close           *txnbuild.Transaction
	DeclarationHash TransactionHash
	Declaration     *txnbuild.Transaction
	FormationHash   TransactionHash
	Formation       *txnbuild.Transaction
}

type OpenAgreement struct {
	Details             OpenAgreementDetails
	ProposerSignatures  OpenAgreementSignatures
	ConfirmerSignatures OpenAgreementSignatures
}

func (oa OpenAgreement) isEmpty() bool {
	return oa.Equal(OpenAgreement{})
}

// isFull checks if the open agreement has the max amount of signatures,
// indicating it is fully signed by all parties.
func (oa OpenAgreement) isFull() bool {
	return oa.ProposerSignatures.isFull() && oa.ConfirmerSignatures.isFull()
}

func (oa OpenAgreement) Equal(oa2 OpenAgreement) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type OA OpenAgreement
	return cmp.Equal(OA(oa), OA(oa2))
}

func (oa OpenAgreement) SignaturesFor(signer *keypair.FromAddress) *OpenAgreementSignatures {
	if oa.Details.ProposingSigner.Equal(signer) {
		return &oa.ProposerSignatures
	}
	if oa.Details.ConfirmingSigner.Equal(signer) {
		return &oa.ConfirmerSignatures
	}
	return nil
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
func (c *Channel) openTxs(d OpenAgreementDetails) (txs OpenAgreementTransactions, err error) {
	if c.openAgreement.Details.Equal(d) {
		return c.openAgreementTransactions, nil
	}
	cad := CloseAgreementDetails{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		IterationNumber:            1,
		Balance:                    0,
		ConfirmingSigner:           d.ConfirmingSigner,
	}

	closeTxs, err := c.closeTxs(d, cad)
	if err != nil {
		err = fmt.Errorf("building close txs for open: %w", err)
		return
	}

	formation, err := txbuild.Formation(txbuild.FormationParams{
		InitiatorSigner:   c.initiatorSigner(),
		ResponderSigner:   c.responderSigner(),
		InitiatorEscrow:   c.initiatorEscrowAccount().Address,
		ResponderEscrow:   c.responderEscrowAccount().Address,
		StartSequence:     d.StartingSequence,
		Asset:             d.Asset.Asset(),
		ExpiresAt:         d.ExpiresAt,
		DeclarationTxHash: closeTxs.DeclarationHash,
		CloseTxHash:       closeTxs.CloseHash,
		ConfirmingSigner:  d.ConfirmingSigner,
	})
	if err != nil {
		err = fmt.Errorf("building formation tx for open: %w", err)
		return
	}
	formationHash, err := formation.Hash(c.networkPassphrase)
	if err != nil {
		err = fmt.Errorf("hashing formation tx: %w", err)
		return
	}

	txs = OpenAgreementTransactions{
		DeclarationHash: closeTxs.DeclarationHash,
		Declaration:     closeTxs.Declaration,
		CloseHash:       closeTxs.CloseHash,
		Close:           closeTxs.Close,
		FormationHash:   formationHash,
		Formation:       formation,
	}
	return
}

// OpenTx builds the formation transaction used for opening the channel. The
// transaction is signed and ready to submit. ProposeOpen and ConfirmOpen must
// be used prior to prepare an open agreement with the other participant.
func (c *Channel) OpenTx() (formationTx *txnbuild.Transaction, err error) {
	oa := c.openAgreement
	txs, err := c.openTxs(oa.Details)
	if err != nil {
		return nil, fmt.Errorf("building txs for for open agreement: %w", err)
	}

	formationTx = txs.Formation

	// Add the formation signatures to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.ProposerSignatures.Formation, oa.Details.ProposingSigner.Hint()))
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.ConfirmerSignatures.Formation, oa.Details.ConfirmingSigner.Hint()))

	// Add the declaration signature provided by the confirming signer that is
	// required to be an extra signer on the formation tx to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.ConfirmerSignatures.Declaration, oa.Details.ConfirmingSigner.Hint(), txs.DeclarationHash[:]))

	// Add the close signature provided by the confirming signer that is
	// required to be an extra signer on the formation tx to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.ConfirmerSignatures.Close, oa.Details.ConfirmingSigner.Hint(), txs.CloseHash[:]))

	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	// if the channel is already open, error.
	if c.openAgreement.isFull() {
		return OpenAgreement{}, fmt.Errorf("cannot propose a new open if channel is already opened")
	}

	d := OpenAgreementDetails{
		ObservationPeriodTime:      p.ObservationPeriodTime,
		ObservationPeriodLedgerGap: p.ObservationPeriodLedgerGap,
		Asset:                      p.Asset,
		ExpiresAt:                  p.ExpiresAt,
		StartingSequence:           p.StartingSequence,
		ProposingSigner:            c.localSigner.FromAddress(),
		ConfirmingSigner:           c.remoteSigner,
	}

	txs, err := c.openTxs(d)
	if err != nil {
		return OpenAgreement{}, err
	}
	sigs, err := signOpenAgreementTxs(txs, c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("signing open agreement with local: %w", err)
	}
	open := OpenAgreement{
		Details:            d,
		ProposerSignatures: sigs,
	}
	c.openAgreement = open
	c.openAgreementTransactions = txs
	return open, nil
}

func (c *Channel) validateOpen(m OpenAgreement) error {
	// if the channel is already open, error.
	if c.openAgreement.isFull() {
		return fmt.Errorf("cannot confirm a new open if channel is already opened")
	}

	// If the open agreement details don't match the open agreement in progress, error.
	if !c.openAgreement.isEmpty() && !m.Details.Equal(c.openAgreement.Details) {
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
func (c *Channel) ConfirmOpen(m OpenAgreement) (open OpenAgreement, err error) {
	err = c.validateOpen(m)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("validating open agreement: %w", err)
	}

	txs, err := c.openTxs(m.Details)
	if err != nil {
		return OpenAgreement{}, err
	}

	// If remote has not signed the txs, error as is invalid.
	remoteSigs := m.SignaturesFor(c.remoteSigner)
	if remoteSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = remoteSigs.Verify(txs, c.networkPassphrase, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("not signed by remote: %w", err)
	}

	// If local has not signed the txs, sign them.
	localSigs := m.SignaturesFor(c.localSigner.FromAddress())
	if localSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = localSigs.Verify(txs, c.networkPassphrase, c.localSigner.FromAddress())
	if err != nil {
		// If the local is not the confirmer, do not sign, because being the
		// proposer they should have signed earlier.
		if !m.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) {
			return OpenAgreement{}, fmt.Errorf("not signed by local: %w", err)
		}
		m.ConfirmerSignatures, err = signOpenAgreementTxs(txs, c.networkPassphrase, c.localSigner)
		if err != nil {
			return OpenAgreement{}, fmt.Errorf("local signing: %w", err)
		}
	}

	// All signatures are present that would be required to submit all
	// transactions in the open.
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    0,
			ObservationPeriodTime:      m.Details.ObservationPeriodTime,
			ObservationPeriodLedgerGap: m.Details.ObservationPeriodLedgerGap,
			ProposingSigner:            m.Details.ProposingSigner,
			ConfirmingSigner:           m.Details.ConfirmingSigner,
		},
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: m.ProposerSignatures.Declaration,
			Close:       m.ProposerSignatures.Close,
		},
		ConfirmerSignatures: CloseAgreementSignatures{
			Declaration: m.ConfirmerSignatures.Declaration,
			Close:       m.ConfirmerSignatures.Close,
		},
	}
	c.latestAuthorizedCloseAgreementTransactions = CloseAgreementTransactions{
		DeclarationHash: txs.DeclarationHash,
		Declaration:     txs.Declaration,
		CloseHash:       txs.CloseHash,
		Close:           txs.Close,
	}
	c.openAgreement = OpenAgreement{
		Details:             m.Details,
		ProposerSignatures:  m.ProposerSignatures,
		ConfirmerSignatures: m.ConfirmerSignatures,
	}
	c.openAgreementTransactions = txs
	return c.openAgreement, nil
}
