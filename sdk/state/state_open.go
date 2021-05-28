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

func (c *Channel) OpenTxs() (close, decl, formation *txnbuild.Transaction, err error) {
	close, err = txbuild.Close(txbuild.CloseParams{
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
	decl, err = txbuild.Declaration(txbuild.DeclarationParams{
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

// OpenPropose proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) OpenPropose() (Open, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	close, _, _, err := c.OpenTxs()
	if err != nil {
		return Open{}, err
	}
	closeSig, err := c.sign(close)
	if err != nil {
		return Open{}, err
	}
	open := Open{
		CloseSignatures: []xdr.DecoratedSignature{closeSig},
	}
	return open, nil
}

// OpenConfirm confirms an open that was proposed. It is called by both
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
func (c *Channel) OpenConfirm(m Open) (Open, error) {
	c.startingSequence = c.initiatorEscrowAccount().SequenceNumber + 1

	close, decl, formation, err := c.OpenTxs()
	if err != nil {
		return m, err
	}

	// If remote has not signed close, error as is invalid.
	err = c.verifySigned(close, m.CloseSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: close invalid %w", err)
	}

	// If local has not signed close, sign it.
	err = c.verifySigned(close, m.CloseSignatures, c.localSigner)
	if errors.As(err, &errNotSigned{}) {
		closeSig, err := c.sign(close)
		if err != nil {
			return m, fmt.Errorf("open confirm: close incomplete: %w", err)
		}
		m.CloseSignatures = append(m.CloseSignatures, closeSig)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: close error: %w", err)
	}

	// If local has not signed declaration, sign it.
	err = c.verifySigned(decl, m.DeclarationSignatures, c.localSigner)
	if errors.As(err, &errNotSigned{}) {
		declSig, err := c.sign(decl)
		if err != nil {
			return m, fmt.Errorf("open confirm: decl %w", err)
		}
		m.DeclarationSignatures = append(m.DeclarationSignatures, declSig)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: decl incomplete: %w", err)
	}

	// If remote has not signed declaration, error as is incomplete.
	err = c.verifySigned(decl, m.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: decl incomplete: %w", err)
	}

	// If local has not signed formation, sign it.
	err = c.verifySigned(formation, m.FormationSignatures, c.localSigner)
	if errors.As(err, &errNotSigned{}) {
		formationSig, err := c.sign(formation)
		if err != nil {
			return m, fmt.Errorf("open confirm: formation local error %w", err)
		}
		m.FormationSignatures = append(m.FormationSignatures, formationSig)
	} else if err != nil {
		return m, fmt.Errorf("open confirm: formation local %w", err)
	}

	// If remote has not signed formation, error as is incomplete.
	err = c.verifySigned(formation, m.FormationSignatures, c.remoteSigner)
	if err != nil {
		return m, fmt.Errorf("open confirm: formation remote %w", err)
	}

	// TODO: channel status = open

	return m, nil
}
