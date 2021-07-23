package state

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// The steps for a channel close are as follows:
// 1. A submits latest declaration tx
// 2. A calls ProposeClose to propose an immediate close by resigning the
//    current close tx
// 3. B calls ConfirmClose to sign and store result
// 4. A calls ConfirmClose to store result
// 5. A or B submit the new close tx
// 6. If A or B declines or is not responsive at any step, A or B may submit the
//    original close tx after the observation period.

func (c *Channel) closeTxs(oad OpenAgreementDetails, d CloseAgreementDetails) (txDecl *txnbuild.Transaction, txClose *txnbuild.Transaction, err error) {
	txClose, err = txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      d.ObservationPeriodTime,
		ObservationPeriodLedgerGap: d.ObservationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            d.IterationNumber,
		AmountToInitiator:          amountToInitiator(d.Balance),
		AmountToResponder:          amountToResponder(d.Balance),
		Asset:                      oad.Asset.Asset(),
	})
	if err != nil {
		return nil, nil, err
	}
	txCloseHash, err := txClose.Hash(c.networkPassphrase)
	if err != nil {
		return nil, nil, err
	}
	txDecl, err = txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         d.IterationNumber,
		IterationNumberExecuted: 0,
		ConfirmingSigner:        d.ConfirmingSigner,
		CloseTxHash:             txCloseHash,
	})
	if err != nil {
		return nil, nil, err
	}
	return txDecl, txClose, nil
}

// CloseTxs builds the declaration and close transactions used for closing the
// channel using the latest close agreement. The transaction are signed and
// ready to submit.
func (c *Channel) CloseTxs() (declTx *txnbuild.Transaction, closeTx *txnbuild.Transaction, err error) {
	closeAgreement := c.latestAuthorizedCloseAgreement
	declTx, closeTx, err = c.closeTxs(c.openAgreement.Details, closeAgreement.Details)
	if err != nil {
		return nil, nil, fmt.Errorf("building declaration and close txs for latest close agreement: %w", err)
	}

	declTx, err = declTx.AddSignatureDecorated(closeAgreement.DeclarationSignatures...)
	if err != nil {
		return nil, nil, fmt.Errorf("attaching signatures to declaration tx for latest close agreement: %w", err)
	}
	for _, s := range closeAgreement.CloseSignatures {
		var signed bool
		signed, err = c.verifySigned(closeTx, []xdr.DecoratedSignature{s}, closeAgreement.Details.ConfirmingSigner)
		if err != nil {
			return nil, nil, fmt.Errorf("finding signatures of confirming signer of close tx for declaration tx for latest close agreement: %w", err)
		}
		if signed {
			var closeTxHash [32]byte
			closeTxHash, err = closeTx.Hash(c.networkPassphrase)
			if err != nil {
				return nil, nil, fmt.Errorf("hashing close tx for including payload sig in declaration tx: %w", err)
			}
			payloadSig := xdr.NewDecoratedSignatureForPayload(s.Signature, s.Hint, closeTxHash[:])
			declTx, err = declTx.AddSignatureDecorated(payloadSig)
			if err != nil {
				return nil, nil, fmt.Errorf("attaching signatures to declaration tx for latest close agreement: %w", err)
			}
		}
	}

	closeTx, err = closeTx.AddSignatureDecorated(closeAgreement.CloseSignatures...)
	if err != nil {
		return nil, nil, fmt.Errorf("attaching signatures to close tx for latest close agreement: %w", err)
	}

	return
}

// TODO - rename? refactor CloseTxs()?
func (c *Channel) LatestUnauthorizedCloseTx() (*txnbuild.Transaction, error) {
	txClose, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodTime,
		ObservationPeriodLedgerGap: c.latestUnauthorizedCloseAgreement.Details.ObservationPeriodLedgerGap,
		InitiatorSigner:            c.initiatorSigner(),
		ResponderSigner:            c.responderSigner(),
		InitiatorEscrow:            c.initiatorEscrowAccount().Address,
		ResponderEscrow:            c.responderEscrowAccount().Address,
		StartSequence:              c.startingSequence,
		IterationNumber:            c.latestUnauthorizedCloseAgreement.Details.IterationNumber,
		AmountToInitiator:          amountToInitiator(c.latestUnauthorizedCloseAgreement.Details.Balance),
		AmountToResponder:          amountToResponder(c.latestUnauthorizedCloseAgreement.Details.Balance),
		Asset:                      c.OpenAgreement().Details.Asset.Asset(),
	})
	if err != nil {
		return nil, err
	}
	return txClose, nil
}

// ProposeClose proposes that the latest authorized close agreement be submitted
// without waiting the observation period. This should be used when participants
// are in agreement on the final close state, but would like to submit earlier
// than the original observation time.
func (c *Channel) ProposeClose() (CloseAgreement, error) {
	d := c.latestAuthorizedCloseAgreement.Details
	d.ObservationPeriodTime = 0
	d.ObservationPeriodLedgerGap = 0
	d.ConfirmingSigner = c.remoteSigner

	txDecl, txClose, err := c.closeTxs(c.openAgreement.Details, d)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("making declaration and close transactions: %w", err)
	}
	txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing declaration transaction: %w", err)
	}
	txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("signing close transaction: %w", err)
	}

	// Store the close agreement while participants iterate on signatures.
	c.latestUnauthorizedCloseAgreement = CloseAgreement{
		Details:               d,
		CloseSignatures:       txClose.Signatures(),
		DeclarationSignatures: txDecl.Signatures(),
	}
	return c.latestUnauthorizedCloseAgreement, nil
}

func (c *Channel) validateClose(ca CloseAgreement) error {
	if ca.Details.IterationNumber != c.latestAuthorizedCloseAgreement.Details.IterationNumber {
		return fmt.Errorf("close agreement iteration number does not match saved latest authorized close agreement")
	}
	if ca.Details.Balance != c.latestAuthorizedCloseAgreement.Details.Balance {
		return fmt.Errorf("close agreement balance does not match saved latest authorized close agreement")
	}
	if ca.Details.ObservationPeriodTime != 0 {
		return fmt.Errorf("close agreement observation period time is not zero")
	}
	if ca.Details.ObservationPeriodLedgerGap != 0 {
		return fmt.Errorf("close agreement observation period ledger gap is not zero")
	}
	if ca.Details.ConfirmingSigner != nil && ca.Details.ConfirmingSigner.Address() != c.localSigner.Address() &&
		ca.Details.ConfirmingSigner.Address() != c.remoteSigner.Address() {
		return fmt.Errorf("close agreement confirmer does not match a local or remote signer, got: %s", ca.Details.ConfirmingSigner.Address())
	}
	return nil
}

// ConfirmClose agrees to a close agreement to be submitted without waiting the
// observation period. The agreement will always be accepted if it is identical
// to the latest authorized close agreement, and it is signed by the participant
// proposing the close.
func (c *Channel) ConfirmClose(ca CloseAgreement) (closeAgreement CloseAgreement, err error) {
	err = c.validateClose(ca)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("validating close agreement: %w", err)
	}

	txDecl, txClose, err := c.closeTxs(c.openAgreement.Details, ca.Details)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("making close transactions: %w", err)
	}

	// If remote has not signed, error as is invalid.
	signed, err := c.verifySigned(txClose, ca.CloseSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying close signature with remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, fmt.Errorf("verifying close: not signed by remote")
	}
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.remoteSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signed by remote: %w", err)
	}
	if !signed {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signed by remote: not signed by remote")
	}

	// If local has not signed, sign.
	signed, err = c.verifySigned(txClose, ca.CloseSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying close signature with local: %w", err)
	}
	if !signed {
		txClose, err = txClose.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fmt.Errorf("signing close transaction: %w", err)
		}
		ca.CloseSignatures = append(ca.CloseSignatures, txClose.Signatures()...)
	}
	signed, err = c.verifySigned(txDecl, ca.DeclarationSignatures, c.localSigner)
	if err != nil {
		return CloseAgreement{}, fmt.Errorf("verifying declaration signature with local: %w", err)
	}
	if !signed {
		txDecl, err = txDecl.Sign(c.networkPassphrase, c.localSigner)
		if err != nil {
			return CloseAgreement{}, fmt.Errorf("signing declaration transaction: %w", err)
		}
		ca.DeclarationSignatures = append(ca.DeclarationSignatures, txDecl.Signatures()...)
	}

	// If the agreement has extra signatures, error.
	if len(ca.DeclarationSignatures) > 2 || len(ca.CloseSignatures) > 2 {
		return CloseAgreement{}, fmt.Errorf("close agreement has too many signatures,"+
			" has declaration: %d, close: %d, max of 2 allowed for each",
			len(ca.DeclarationSignatures), len(ca.CloseSignatures))
	}

	// The new close agreement is valid and authorized, store and promote it.
	c.latestAuthorizedCloseAgreement = CloseAgreement{
		Details:               ca.Details,
		CloseSignatures:       ca.CloseSignatures,
		DeclarationSignatures: ca.DeclarationSignatures,
	}
	c.latestUnauthorizedCloseAgreement = CloseAgreement{}
	return c.latestAuthorizedCloseAgreement, nil
}
