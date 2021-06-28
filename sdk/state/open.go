package state

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type OpenAgreement struct {
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FormationSignatures   []xdr.DecoratedSignature

	Asset      Asset
	AssetLimit int64
}

// OpenParams are the parameters selected by the participant proposing an open channel.
type OpenParams struct {
	Asset      Asset
	AssetLimit int64
}

func (c *Channel) OpenTxs(p OpenParams) (txClose, txDecl, formation *txnbuild.Transaction, err error) {
	txClose, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            1,
		AmountToInitiator:          0,
		AmountToResponder:          0,
		Asset:                      p.Asset,
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
		Asset:           p.Asset,
		AssetLimit:      p.AssetLimit,
	})
	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen(p OpenParams) (OpenAgreement, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, _, _, err := c.OpenTxs(p)
	if err != nil {
		return OpenAgreement{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return OpenAgreement{}, err
	}
	open := OpenAgreement{
		CloseSignatures: txClose.Signatures(),
		Asset:           p.Asset,
		AssetLimit:      p.AssetLimit,
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
	// at the end of this method if no error then save a new channel openAgreement. Use the
	// channel's saved open agreement details if present, to prevent other party trying to change.
	defer func() {
		if err != nil {
			return
		}
		asset := m.Asset
		assetLimit := m.AssetLimit
		if c.openAgreement.Asset != nil {
			asset = c.openAgreement.Asset
		}
		if c.openAgreement.AssetLimit != 0 {
			assetLimit = c.openAgreement.AssetLimit
		}

		c.openAgreement = OpenAgreement{
			CloseSignatures:       appendNewSignatures(c.openAgreement.CloseSignatures, m.CloseSignatures),
			DeclarationSignatures: appendNewSignatures(c.openAgreement.DeclarationSignatures, m.DeclarationSignatures),
			FormationSignatures:   appendNewSignatures(c.openAgreement.FormationSignatures, m.FormationSignatures),
			Asset:                 asset,
			AssetLimit:            assetLimit,
		}
	}()

	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, txDecl, formation, err := c.OpenTxs(OpenParams{m.Asset, m.AssetLimit})
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
			IterationNumber: 1,
			Balance:         Amount{Asset: m.Asset},
		},
		CloseSignatures:       m.CloseSignatures,
		DeclarationSignatures: m.DeclarationSignatures,
	}
	return m, authorized, nil
}
