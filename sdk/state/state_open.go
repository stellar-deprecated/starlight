package state

import (
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
)

type Open struct {
	CloseSignatures       []string
	DeclarationSignatures []string
	FormationSignatures   []string
}

func (c *Channel) openTxs() (close, decl, formation *txnbuild.Transaction, err error) {
	close, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.observationPeriodTime,
		ObservationPeriodLedgerGap: c.observationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            0,
		AmountToInitiator:          0,
		AmountToResponder:          0,
	})
	if err != nil {
		return
	}
	decl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         0,
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

func (c *Channel) openTxHashes() (closeHash, declHash, formationHash [32]byte, err error) {
	var close, decl, formation *txnbuild.Transaction
	close, decl, formation, err = c.openTxs()
	if err != nil {
		return
	}
	closeHash, err = close.Hash(c.networkPassphrase)
	if err != nil {
		return
	}
	declHash, err = decl.Hash(c.networkPassphrase)
	if err != nil {
		return
	}
	formationHash, err = formation.Hash(c.networkPassphrase)
	return
}

// OpenPropose proposes the open of the channel, it is called by the participant
// initiating the channel.
func (c *Channel) OpenPropose() (Open, error) {
	closeHash, _, _, err := c.openTxHashes()
	closeSig, err := c.localSigner.SignBase64(closeHash[:])
	if err != nil {
		return Open{}, err
	}
	open := Open{
		CloseSignatures: []string{closeSig},
	}
	return open, nil
}

// OpenConfirm confirms an open that was proposed. It is called by both
// participants as they both participate in the open process.
//
// If there are no sigs on the open, the local participant will only add a close
// signature.
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
	// TODO: if no remote close sig
	// TODO:   add close sig
	// TODO:   return incomplete

	// TODO: if no local close sig
	// TODO:   add close sig
	// TODO: if no local decl sig
	// TODO:   add decl sig
	// TODO:   return incomplete

	// TODO: if no remote decl sig
	// TODO:   return incomplete

	// TODO: if no local formation sig
	// TODO:   add local formation sig
	// TODO: if no remote formation sig
	// TODO:   return incomplete

	// TODO: channel status = open
	// TODO: return success
	return Open{}, nil
}
