package state

import (
	"fmt"
	"strconv"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// TODO - should store on channel like Update and Close proposals?
type Open struct {
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FormationSignatures   []xdr.DecoratedSignature

	Asset      Asset
	AssetLimit string
}

// OpenParams are the parameters selected by the participant proposing an open channel.
type OpenParams struct {
	Asset      Asset
	AssetLimit string
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
func (c *Channel) ProposeOpen(p OpenParams) (Open, error) {
	if !p.Asset.IsNative() {
		if _, err := strconv.Atoi(p.AssetLimit); err != nil {
			return Open{}, fmt.Errorf("parsing asset limit: %w", err)
		}
	}
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, _, _, err := c.OpenTxs(p)
	if err != nil {
		return Open{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return Open{}, err
	}
	open := Open{
		CloseSignatures: txClose.Signatures(),
		Asset:           p.Asset,
		AssetLimit:      p.AssetLimit,
	}
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
func (c *Channel) ConfirmOpen(m Open) (open Open, fullySigned bool, err error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, txDecl, formation, err := c.OpenTxs(OpenParams{m.Asset, m.AssetLimit})
	if err != nil {
		return m, fullySigned, err
	}

	// If remote has not signed close, error as is invalid.
	signed, err := c.verifySigned(txClose, m.CloseSignatures, c.remoteSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("verifying close signed by remote: %w", err)
	}
	if !signed {
		return m, fullySigned, fmt.Errorf("verifying close signed by remote: not signed by remote")
	}

	// If local has not signed close, sign it.
	signed, err = c.verifySigned(txClose, m.CloseSignatures, c.localSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("verifying close signed by local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fullySigned, fmt.Errorf("signing close with local: %w", err)
		}
		m.CloseSignatures = append(m.CloseSignatures, txClose.Signatures()...)
	}

	// If local has not signed declaration, sign it.
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.localSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("verifying declaration with local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fullySigned, fmt.Errorf("signing declaration with local: decl %w", err)
		}
		m.DeclarationSignatures = append(m.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If remote has not signed declaration, don't perform any others signing.
	signed, err = c.verifySigned(txDecl, m.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("verifying declaration with remote: decl: %w", err)
	}
	if !signed {
		return m, fullySigned, nil
	}

	// If local has not signed formation, sign it.
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.localSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("verifying formation with local: %w", err)
	}
	if !signed {
		formation, err = formation.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fullySigned, fmt.Errorf("signing formation with local: %w", err)
		}
		m.FormationSignatures = append(m.FormationSignatures, formation.Signatures()...)
	}

	// If remote has not signed formation, it is incomplete.
	signed, err = c.verifySigned(formation, m.FormationSignatures, c.remoteSigner)
	if err != nil {
		return m, fullySigned, fmt.Errorf("open confirm: formation remote %w", err)
	}
	if !signed {
		return m, fullySigned, nil
	}

	// All signatures are present that would be required to submit all
	// transactions in the open.
	fullySigned = true
	c.latestCloseAgreement = CloseAgreement{
		IterationNumber:       1,
		Balance:               Amount{Asset: m.Asset},
		CloseSignatures:       m.CloseSignatures,
		DeclarationSignatures: m.DeclarationSignatures,
	}
	return m, fullySigned, nil
}
