package state

import (
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type OpenAgreementDetails struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	AssetLimit                 int64
}

type OpenAgreement struct {
	Details               OpenAgreementDetails
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FormationSignatures   []xdr.DecoratedSignature
}

func (oa OpenAgreement) isEmpty() bool {
	return oa.Details == OpenAgreementDetails{}
}

// OpenParams are the parameters selected by the participant proposing an open channel.
type OpenParams struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	Asset                      Asset
	AssetLimit                 int64
}

func (c *Channel) OpenTxs(d OpenAgreementDetails) (txClose, txDecl, formation *txnbuild.Transaction, err error) {
	txClose, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            1,
		AmountToInitiator:          0,
		AmountToResponder:          0,
		Asset:                      d.Asset,
	})
	if err != nil {
		return
	}
	txDecl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         1,
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return
	}
	formation, err = txbuild.Formation(txbuild.FormationParams{
		InitiatorSigner: c.initiatorSigner(),
		ResponderSigner: c.responderSigner(),
		InitiatorEscrow: c.initiatorEscrowAccount().Address,
		ResponderEscrow: c.responderEscrowAccount().Address,
		StartSequence:   c.startingSequence,
		Asset:           d.Asset,
		AssetLimit:      d.AssetLimit,
	})
	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	d := OpenAgreementDetails(p)

	txClose, _, _, err := c.OpenTxs(d)
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
	// If the open agreement details don't match the open agreement in progress, error.
	if !c.openAgreement.isEmpty() && m.Details != c.openAgreement.Details {
		return m, authorized, fmt.Errorf("input open agreement details do not match the saved open agreement details")
	}

	// at the end of this method, if no error, then save a new channel openAgreement. Use the
	// channel's saved open agreement details if present, to prevent other party from changing.
	defer func() {
		if err != nil {
			return
		}
		c.openAgreement = OpenAgreement{
			Details: m.Details,
			CloseSignatures:       appendNewSignatures(c.openAgreement.CloseSignatures, m.CloseSignatures),
			DeclarationSignatures: appendNewSignatures(c.openAgreement.DeclarationSignatures, m.DeclarationSignatures),
			FormationSignatures:   appendNewSignatures(c.openAgreement.FormationSignatures, m.FormationSignatures),
		}
	}()

	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, txDecl, formation, err := c.OpenTxs(m.Details)
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
			Balance:                    Amount{Asset: m.Details.Asset},
			ObservationPeriodTime:      m.Details.ObservationPeriodTime,
			ObservationPeriodLedgerGap: m.Details.ObservationPeriodLedgerGap,
		},
		CloseSignatures:       m.CloseSignatures,
		DeclarationSignatures: m.DeclarationSignatures,
	}
	return m, authorized, nil
}
