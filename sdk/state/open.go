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

func signOpenAgreementTxs(decl, close, formation *txnbuild.Transaction, networkPassphrase string, signer *keypair.Full) (s OpenAgreementSignatures, err error) {
	s.Declaration, err = signTx(decl, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing declaration: %w", err)
	}
	s.Close, err = signTx(close, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing close: %w", err)
	}
	s.Formation, err = signTx(formation, networkPassphrase, signer)
	if err != nil {
		return OpenAgreementSignatures{}, fmt.Errorf("signing formation: %w", err)
	}
	return s, nil
}

func (s OpenAgreementSignatures) Verify(decl, close, formation *txnbuild.Transaction, networkPassphrase string, signer *keypair.FromAddress) error {
	err := verifySigned(decl, networkPassphrase, signer, s.Declaration)
	if err != nil {
		return fmt.Errorf("verifying declaration signed: %w", err)
	}
	err = verifySigned(close, networkPassphrase, signer, s.Close)
	if err != nil {
		return fmt.Errorf("verifying close signed: %w", err)
	}
	err = verifySigned(formation, networkPassphrase, signer, s.Formation)
	if err != nil {
		return fmt.Errorf("verifying formation signed: %w", err)
	}
	return nil
}

// OpenAgreementTransactionHashes contain all the transaction hashes for the
// transactions that make up the open agreement.
type OpenAgreementTransactionHashes struct {
	Close       TransactionHash
	Declaration TransactionHash
	Formation   TransactionHash
}

type OpenAgreement struct {
	Details             OpenAgreementDetails
	TransactionHashes   OpenAgreementTransactionHashes
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

func (c *Channel) openTxs(d OpenAgreementDetails) (declHash TransactionHash, decl *txnbuild.Transaction, closeHash TransactionHash, close *txnbuild.Transaction, formationHash [32]byte, formation *txnbuild.Transaction, err error) {
	cad := CloseAgreementDetails{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		IterationNumber:            1,
		Balance:                    0,
		ConfirmingSigner:           d.ConfirmingSigner,
	}

	declHash, decl, closeHash, close, err = c.closeTxs(d, cad)
	if err != nil {
		err = fmt.Errorf("building close txs for open: %w", err)
		return
	}

	formation, err = txbuild.Formation(txbuild.FormationParams{
		InitiatorSigner:   c.initiatorSigner(),
		ResponderSigner:   c.responderSigner(),
		InitiatorEscrow:   c.initiatorEscrowAccount().Address,
		ResponderEscrow:   c.responderEscrowAccount().Address,
		StartSequence:     d.StartingSequence,
		Asset:             d.Asset.Asset(),
		ExpiresAt:         d.ExpiresAt,
		DeclarationTxHash: declHash,
		CloseTxHash:       closeHash,
		ConfirmingSigner:  d.ConfirmingSigner,
	})
	if err != nil {
		err = fmt.Errorf("building formation tx for open: %w", err)
		return
	}
	formationHash, err = formation.Hash(c.networkPassphrase)
	if err != nil {
		err = fmt.Errorf("hashing formation tx: %w", err)
		return
	}

	return
}

// OpenTx builds the formation transaction used for opening the channel. The
// transaction is signed and ready to submit. ProposeOpen and ConfirmOpen must
// be used prior to prepare an open agreement with the other participant.
func (c *Channel) OpenTx() (formationTx *txnbuild.Transaction, err error) {
	oa := c.openAgreement
	declTxHash, _, closeTxHash, _, _, formationTx, err := c.openTxs(oa.Details)
	if err != nil {
		return nil, fmt.Errorf("building txs for for open agreement: %w", err)
	}

	// Add the formation signatures to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.ProposerSignatures.Formation, oa.Details.ProposingSigner.Hint()))
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignature(oa.ConfirmerSignatures.Formation, oa.Details.ConfirmingSigner.Hint()))

	// Add the declaration signature provided by the confirming signer that is
	// required to be an extra signer on the formation tx to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.ConfirmerSignatures.Declaration, oa.Details.ConfirmingSigner.Hint(), declTxHash[:]))

	// Add the close signature provided by the confirming signer that is
	// required to be an extra signer on the formation tx to the formation tx.
	formationTx, _ = formationTx.AddSignatureDecorated(xdr.NewDecoratedSignatureForPayload(oa.ConfirmerSignatures.Close, oa.Details.ConfirmingSigner.Hint(), closeTxHash[:]))

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

	txDeclHash, txDecl, txCloseHash, txClose, txFormationHash, txFormation, err := c.openTxs(d)
	if err != nil {
		return OpenAgreement{}, err
	}
	sigs, err := signOpenAgreementTxs(txDecl, txClose, txFormation, c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("signing open agreement with local: %w", err)
	}
	open := OpenAgreement{
		Details: d,
		TransactionHashes: OpenAgreementTransactionHashes{
			Declaration: txDeclHash,
			Close:       txCloseHash,
			Formation:   txFormationHash,
		},
		ProposerSignatures: sigs,
	}
	c.openAgreement = open
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

	txDeclHash, txDecl, txCloseHash, txClose, formationHash, formation, err := c.openTxs(m.Details)
	if err != nil {
		return OpenAgreement{}, err
	}

	// Check that the transactions built match the transaction hashes in the
	// open agreement.
	if m.TransactionHashes.Declaration != txDeclHash {
		// TODO
	}
	if m.TransactionHashes.Close != txCloseHash {
		// TODO
	}
	if m.TransactionHashes.Formation != formationHash {
		// TODO
	}

	// If remote has not signed the txs, error as is invalid.
	remoteSigs := m.SignaturesFor(c.remoteSigner)
	if remoteSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = remoteSigs.Verify(txDecl, txClose, formation, c.networkPassphrase, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("not signed by remote: %w", err)
	}

	// If local has not signed the txs, sign them.
	localSigs := m.SignaturesFor(c.localSigner.FromAddress())
	if localSigs == nil {
		return OpenAgreement{}, fmt.Errorf("remote is not a signer")
	}
	err = localSigs.Verify(txDecl, txClose, formation, c.networkPassphrase, c.localSigner.FromAddress())
	if err != nil {
		// If the local is not the confirmer, do not sign, because being the
		// proposer they should have signed earlier.
		if !m.Details.ConfirmingSigner.Equal(c.localSigner.FromAddress()) {
			return OpenAgreement{}, fmt.Errorf("not signed by local: %w", err)
		}
		m.ConfirmerSignatures, err = signOpenAgreementTxs(txDecl, txClose, formation, c.networkPassphrase, c.localSigner)
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
	c.openAgreement = OpenAgreement{
		Details:             m.Details,
		ProposerSignatures:  m.ProposerSignatures,
		ConfirmerSignatures: m.ConfirmerSignatures,
	}
	return c.openAgreement, nil
}
