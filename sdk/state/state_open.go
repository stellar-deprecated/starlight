package state

import (
	"errors"
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type Open struct {
	CloseSignatures       []xdr.DecoratedSignature
	DeclarationSignatures []xdr.DecoratedSignature
	FormationSignatures   []xdr.DecoratedSignature
}

func (c *Channel) OpenTxs() (txClose, txDecl, formation *txnbuild.Transaction, err error) {
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
	})
	return
}

// ProposeOpen proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) ProposeOpen() (Open, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, _, _, err := c.OpenTxs()
	if err != nil {
		return Open{}, err
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return Open{}, err
	}
	open := Open{
		CloseSignatures: txClose.Signatures(),
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
func (c *Channel) ConfirmOpen(m Open) (Open, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	txClose, txDecl, formation, err := c.OpenTxs()
	if err != nil {
		return m, err
	}

	// If remote has not signed close, error as is invalid.
	err = c.verifySigned(txClose, m.CloseSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: close invalid %w", err)
	}

	// If local has not signed close, sign it.
	err = c.verifySigned(txClose, m.CloseSignatures, c.localSigner)
	if errors.Is(err, ErrNotSigned{}) {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fmt.Errorf("open confirm: close incomplete: %w", err)
		}
		m.CloseSignatures = append(m.CloseSignatures, txClose.Signatures()...)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: close error: %w", err)
	}

	// If local has not signed declaration, sign it.
	err = c.verifySigned(txDecl, m.DeclarationSignatures, c.localSigner)
	if errors.Is(err, ErrNotSigned{}) {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fmt.Errorf("open confirm: decl %w", err)
		}
		m.DeclarationSignatures = append(m.DeclarationSignatures, txDecl.Signatures()...)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: decl incomplete: %w", err)
	}

	// If remote has not signed declaration, error as is incomplete.
	err = c.verifySigned(txDecl, m.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: decl incomplete: %w", err)
	}

	// If local has not signed formation, sign it.
	err = c.verifySigned(formation, m.FormationSignatures, c.localSigner)
	if errors.Is(err, ErrNotSigned{}) {
		formation, err = formation.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return m, fmt.Errorf("open confirm: formation local error %w", err)
		}
		m.FormationSignatures = append(m.FormationSignatures, formation.Signatures()...)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: formation local %w", err)
	}

	// If remote has not signed formation, error as is incomplete.
	err = c.verifySigned(formation, m.FormationSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: formation remote %w", err)
	}

	// TODO: channel status = open
	c.latestCloseAgreement = &CloseAgreement{
		IterationNumber:       1,
		Balance:               Amount{},
		CloseSignatures:       m.CloseSignatures,
		DeclarationSignatures: m.DeclarationSignatures,
	}
	return m, nil
}
