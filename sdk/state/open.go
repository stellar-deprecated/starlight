package state

import (
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type OpenAgreementDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	ExpiresAt                  time.Time
}

func (d OpenAgreementDetails) Equal(d2 OpenAgreementDetails) bool {
	return d.ObservationPeriodTime == d2.ObservationPeriodTime &&
		d.ObservationPeriodLedgerGap == d2.ObservationPeriodLedgerGap &&
		d.Asset == d2.Asset &&
		d.ExpiresAt.Equal(d2.ExpiresAt)
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
	}

	decl, close, err = c.closeTxs(d, cad)
	if err != nil {
		err = fmt.Errorf("building close txs for open: %w", err)
		return
	}

	formation, err = txbuild.Formation(txbuild.FormationParams{
		InitiatorSigner: c.initiatorSigner(),
		ResponderSigner: c.responderSigner(),
		InitiatorEscrow: c.initiatorEscrowAccount().Address,
		ResponderEscrow: c.responderEscrowAccount().Address,
		StartSequence:   c.startingSequence,
		Asset:           d.Asset.Asset(),
		ExpiresAt:       d.ExpiresAt,
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
	_, _, formationTx, err = c.openTxs(openAgreement.Details)
	if err != nil {
		return nil, fmt.Errorf("building declaration and close txs for latest close agreement: %w", err)
	}
	formationTx, err = formationTx.AddSignatureDecorated(openAgreement.FormationSignatures...)
	if err != nil {
		return nil, fmt.Errorf("attaching signatures to formation tx for latest close agreement: %w", err)
	}
	return
}

// OpenTx builds the formation transaction used for opening the channel. The
// transaction is signed and ready to submit. ProposeOpen and ConfirmOpen must
// be used prior to prepare an open agreement with the other participant.
func (c *Channel) OpenTx() (formationTx *txnbuild.Transaction, err error) {
	openAgreement := c.OpenAgreement()
	formationTx, err = c.openTxs(openAgreement.Details)
	if err != nil {
		return nil, fmt.Errorf("building declaration and close txs for latest close agreement: %w", err)
	}
	formationTx, err = formationTx.AddSignatureDecorated(openAgreement.FormationSignatures...)
	if err != nil {
		return nil, fmt.Errorf("attaching signatures to formation tx for latest close agreement: %w", err)
	}
	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	d := OpenAgreementDetails(p)

	_, txClose, _, err := c.openTxs(d)
	if err != nil {
		return OpenAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, err
	}
	open := OpenAgreement{
		Details:         d,
		CloseSignatures: txClose.Signatures(),
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

// ConfirmOpen confirms an open that was proposed. It is called by both
// participants as they both participate in the open process.
//
// If there are no sigs on the open, the open is invalid, it must be signed by
// the proposing participant.
//
// If there is a close signature on the open by the remote participant, the
// local participant will add close and/or declaration signatures, as required.
//
// If there are close and declaration signatures for all participants, the local
// participant will add a formation signature, if required.
//
// If there are close, declaration, and formation signatures for all
// participants, the channel will be considered open.
//
// If after confirming the open has all the signatures it needs to be fully and
// completely signed, fully signed will be true, otherwise it will be false.
func (c *Channel) ConfirmOpen(m OpenAgreement) (open OpenAgreement, authorized bool, err error) {
	err = c.validateOpen(m)
	if err != nil {
		return m, authorized, fmt.Errorf("validating open agreement: %w", err)
	}

	// at the end of this method, if no error, then save a new channel openAgreement. Use the
	// channel's saved open agreement details if present, to prevent other party from changing.
	defer func() {
		if err != nil {
			return
		}
		c.openAgreement = OpenAgreement{
			Details:               m.Details,
			CloseSignatures:       appendNewSignatures(c.openAgreement.CloseSignatures, m.CloseSignatures),
			DeclarationSignatures: appendNewSignatures(c.openAgreement.DeclarationSignatures, m.DeclarationSignatures),
			FormationSignatures:   appendNewSignatures(c.openAgreement.FormationSignatures, m.FormationSignatures),
		}
	}()

	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txDecl, txClose, formation, err := c.openTxs(m.Details)
	if err != nil {
		return m, authorized, err
	}

	// If remote has not signed close, error as is invalid.
	signed, err := c.verifySigned(txClose, m.CloseSignatures, c.remoteSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return m, authorized, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}

	// If local has not signed close, sign it.
	signed, err = c.verifySigned(txClose, m.CloseSignatures, c.localSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, authorized, fmt.Errorf("signing close with local: %w", err)
		}
		m.CloseSignatures = append(m.CloseSignatures, txClose.Signatures()...)
	}

	// If local has not signed declaration, sign it.
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.localSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("verifying declaration with local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, authorized, fmt.Errorf("signing declaration with local: decl %w", err)
		}
		m.DeclarationSignatures = append(m.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If remote has not signed declaration, don't perform any others signing.
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("verifying declaration with remote: decl: %w", err)
	}
	if !signed {
		return m, authorized, nil
	}

	// If local has not signed formation, sign it.
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.localSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("verifying formation with local: %w", err)
	}
	if !signed {
		formation, err = formation.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, authorized, fmt.Errorf("signing formation with local: %w", err)
		}
		m.FormationSignatures = append(m.FormationSignatures, formation.Signatures()...)
	}

	// If remote has not signed formation, it is incomplete.
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.remoteSigner)
	if err != nil {
		return m, authorized, fmt.Errorf("open confirm: formation remote %w", err)
	}
	if !signed {
		return m, authorized, nil
	}

	// All signatures are present that would be required to submit all
	// transactions in the open.
	authorized = true
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    0,
			ObservationPeriodTime:      m.Details.ObservationPeriodTime,
			ObservationPeriodLedgerGap: m.Details.ObservationPeriodLedgerGap,
		},
		CloseSignatures:       m.CloseSignatures,
		DeclarationSignatures: m.DeclarationSignatures,
	}
	return m, authorized, nil
}
