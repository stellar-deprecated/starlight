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
	ConfirmingSigner           *keypair.FromAddress
}

func (d OpenAgreementDetails) Equal(d2 OpenAgreementDetails) bool {
	return d.ObservationPeriodTime == d2.ObservationPeriodTime &&
		d.ObservationPeriodLedgerGap == d2.ObservationPeriodLedgerGap &&
		d.Asset == d2.Asset &&
		d.ExpiresAt.Equal(d2.ExpiresAt) &&
		((d.ConfirmingSigner == nil && d2.ConfirmingSigner == nil) ||
			(d.ConfirmingSigner != nil && d2.ConfirmingSigner != nil &&
				d.ConfirmingSigner.Address() == d2.ConfirmingSigner.Address()))
}

type OpenAgreement struct {
	Details               OpenAgreementDetails
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FormationSignatures   []xdr.DecoratedSignature
}

func (oa OpenAgreement) isEmpty() bool {
	return oa.Equal(OpenAgreement{})
}

func (oa OpenAgreement) Equal(oa2 OpenAgreement) bool {
	// TODO: Replace cmp.Equal with a hand written equals.
	type OA OpenAgreement
	return cmp.Equal(OA(oa), OA(oa2))
}

// OpenParams are the parameters selected by the participant proposing an open channel.
type OpenParams struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	ExpiresAt                  time.Time
}

func (c *Channel) openTxs(d OpenAgreementDetails) (decl, close, formation *txnbuild.Transaction, err error) {
	cad := CloseAgreementDetails{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		IterationNumber:            1,
		Balance:                    0,
		ConfirmingSigner:           d.ConfirmingSigner,
	}

	decl, close, err = c.closeTxs(d, cad)
	if err != nil {
		err = fmt.Errorf("building close txs for open: %w", err)
		return
	}
	declHash, err := decl.Hash(c.networkPassphrase)
	if err != nil {
		err = fmt.Errorf("generating hash for declaration tx for open: %w", err)
		return
	}
	closeHash, err := close.Hash(c.networkPassphrase)
	if err != nil {
		err = fmt.Errorf("generating hash for close tx for open: %w", err)
		return
	}

	formation, err = txbuild.Formation(txbuild.FormationParams{
		InitiatorSigner:   c.initiatorSigner(),
		ResponderSigner:   c.responderSigner(),
		InitiatorEscrow:   c.initiatorEscrowAccount().Address,
		ResponderEscrow:   c.responderEscrowAccount().Address,
		StartSequence:     c.startingSequence,
		Asset:             d.Asset.Asset(),
		ExpiresAt:         d.ExpiresAt,
		DeclarationTxHash: declHash,
		CloseTxHash:       closeHash,
		ConfirmingSigner:  d.ConfirmingSigner,
	})
	if err != nil {
		err = fmt.Errorf("building formation tx for open: %w", err)
	}

	return
}

// OpenTx builds the formation transaction used for opening the channel. The
// transaction is signed and ready to submit. ProposeOpen and ConfirmOpen must
// be used prior to prepare an open agreement with the other participant.
func (c *Channel) OpenTx() (formationTx *txnbuild.Transaction, err error) {
	openAgreement := c.openAgreement
	declTx, closeTx, formationTx, err := c.openTxs(openAgreement.Details)
	if err != nil {
		return nil, fmt.Errorf("building txs for for open agreement: %w", err)
	}
	formationTx, err = formationTx.AddSignatureDecorated(openAgreement.FormationSignatures...)
	if err != nil {
		return nil, fmt.Errorf("attaching signatures to formation tx for latest close agreement: %w", err)
	}
	for _, s := range openAgreement.DeclarationSignatures {
		var signed bool
		signed, err = c.verifySigned(declTx, []xdr.DecoratedSignature{s}, openAgreement.Details.ConfirmingSigner)
		if err != nil {
			return nil, fmt.Errorf("finding signatures of confirming signer of declaration tx for formation tx: %w", err)
		}
		if signed {
			formationTx, err = formationTx.AddSignatureDecorated(s)
			if err != nil {
				return nil, fmt.Errorf("attaching signatures to formation tx for open agreement: %w", err)
			}
		}
	}
	for _, s := range openAgreement.CloseSignatures {
		var signed bool
		signed, err = c.verifySigned(closeTx, []xdr.DecoratedSignature{s}, openAgreement.Details.ConfirmingSigner)
		if err != nil {
			return nil, fmt.Errorf("finding signatures of confirming signer of close tx for formation tx: %w", err)
		}
		if signed {
			formationTx, err = formationTx.AddSignatureDecorated(s)
			if err != nil {
				return nil, fmt.Errorf("attaching signatures to formation tx for open agreement: %w", err)
			}
		}
	}
	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	d := OpenAgreementDetails{
		ObservationPeriodTime:      p.ObservationPeriodTime,
		ObservationPeriodLedgerGap: p.ObservationPeriodLedgerGap,
		Asset:                      p.Asset,
		ExpiresAt:                  p.ExpiresAt,
		ConfirmingSigner:           c.remoteSigner,
	}

	txDecl, txClose, txFormation, err := c.openTxs(d)
	if err != nil {
		return OpenAgreement{}, err
	}
	txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, err
	}
	txFormation, err = txFormation.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, err
	}
	open := OpenAgreement{
		Details:               d,
		CloseSignatures:       txClose.Signatures(),
		DeclarationSignatures: txDecl.Signatures(),
		FormationSignatures:   txFormation.Signatures(),
	}
	c.openAgreement = open
	return open, nil
}

func (c *Channel) validateOpen(m OpenAgreement) error {
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
// copy of the agreement signed by the destination.
func (c *Channel) ConfirmOpen(m OpenAgreement) (open OpenAgreement, err error) {
	err = c.validateOpen(m)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("validating open agreement: %w", err)
	}

	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txDecl, txClose, formation, err := c.openTxs(m.Details)
	if err != nil {
		return OpenAgreement{}, err
	}

	// If remote has not signed the txs, error as is invalid.
	signed, err := c.verifySigned(txClose, m.CloseSignatures, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return OpenAgreement{}, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying declaration with remote: %w", err)
	}
	if !signed {
		return OpenAgreement{}, fmt.Errorf("verifying declaration signed by remote: not signed by remote")
	}
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.remoteSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying formation with remote: %w", err)
	}
	if !signed {
		return OpenAgreement{}, fmt.Errorf("verifying formation signed by remote: not signed by remote")
	}

	// If local has not signed the txs, sign them.
	signed, err = c.verifySigned(txClose, m.CloseSignatures, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return OpenAgreement{}, fmt.Errorf("signing close with local: %w", err)
		}
		m.CloseSignatures = append(m.CloseSignatures, txClose.Signatures()...)
	}
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying declaration with local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return OpenAgreement{}, fmt.Errorf("signing declaration with local: decl %w", err)
		}
		m.DeclarationSignatures = append(m.DeclarationSignatures, txDecl.Signatures()...)
	}
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.localSigner)
	if err != nil {
		return OpenAgreement{}, fmt.Errorf("verifying formation with local: %w", err)
	}
	if !signed {
		formation, err = formation.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return OpenAgreement{}, fmt.Errorf("signing formation with local: %w", err)
		}
		m.FormationSignatures = append(m.FormationSignatures, formation.Signatures()...)
	}

	// If an agreement ever surpasses 2 signatures per tx, error.
	if len(m.DeclarationSignatures) > 2 || len(m.CloseSignatures) > 2 || len(m.FormationSignatures) > 2 {
		return OpenAgreement{}, fmt.Errorf("input open agreement has too many signatures,"+
			" has declaration: %d, close: %d, formation: %d, max of 2 allowed for each",
			len(m.DeclarationSignatures), len(m.CloseSignatures), len(m.FormationSignatures))
	}

	// All signatures are present that would be required to submit all
	// transactions in the open.
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    0,
			ObservationPeriodTime:      m.Details.ObservationPeriodTime,
			ObservationPeriodLedgerGap: m.Details.ObservationPeriodLedgerGap,
			ConfirmingSigner:           m.Details.ConfirmingSigner,
		},
		CloseSignatures:       appendNewSignatures(c.openAgreement.CloseSignatures, m.CloseSignatures),
		DeclarationSignatures: appendNewSignatures(c.openAgreement.DeclarationSignatures, m.DeclarationSignatures),
	}
	c.openAgreement = OpenAgreement{
		Details:               m.Details,
		CloseSignatures:       appendNewSignatures(c.openAgreement.CloseSignatures, m.CloseSignatures),
		DeclarationSignatures: appendNewSignatures(c.openAgreement.DeclarationSignatures, m.DeclarationSignatures),
		FormationSignatures:   appendNewSignatures(c.openAgreement.FormationSignatures, m.FormationSignatures),
	}
	return c.openAgreement, nil
}
