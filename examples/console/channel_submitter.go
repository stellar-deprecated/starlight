package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
)

// Submitter submits fee-less transactions to the network via Horizon by
// wrapping them in a fee bump transaction.
//
// Submitter is intended for use with transactions that have a zero fee and were
// not intended for submitting on their own.
//
// Submitter is intended for use with payment channels and will submit
// transactions to execute a channel's agreements.
//
// The BaseFee is the base fee that will be used for any submission.
type ChannelSubmitter struct {
	Channel   *state.Channel
	Submitter *Submitter
}

// SubmitLatestDeclarationTx submits the latest authorized declaration
// transaction for a channel wrapped in a fee bump transaction with the
// Submitter's FeeAccount as the fee account.
func (s ChannelSubmitter) SubmitLatestDeclarationTx() error {
	return s.SubmitDeclarationTx(s.Channel.LatestCloseAgreement())
}

// SubmitDeclarationTx submits the declaration transaction for a channel's close
// agreement wrapped in a fee bump transaction with the Submitter's FeeAccount
// as the fee account.
func (s ChannelSubmitter) SubmitDeclarationTx(closeAgreement state.CloseAgreement) error {
	// Get declaration transaction.
	declTx, _, err := s.Channel.CloseTxs(closeAgreement.Details)
	if err != nil {
		return fmt.Errorf("building declaration tx: %w", err)
	}

	// Attach signatures to declaration transaction.
	declTx, err = declTx.AddSignatureDecorated(closeAgreement.DeclarationSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the declaration tx: %w", err)
	}

	// Submit fee bump transaction that wraps the declaration transaction.
	err = s.Submitter.SubmitFeeBumpTx(declTx)
	if err != nil {
		return fmt.Errorf("submitting declaration tx: %w", err)
	}

	return nil
}

// SubmitLatestCloseTx submits the latest authorized close transaction for a
// channel wrapped in a fee bump transaction with the Submitter's FeeAccount as
// the fee account.
func (s ChannelSubmitter) SubmitLatestCloseTx() error {
	return s.SubmitCloseTx(s.Channel.LatestCloseAgreement())
}

// SubmitCloseTx submits the close transaction for a channel's close agreement
// wrapped in a fee bump transaction with the Submitter's FeeAccount as the fee
// account.
func (s ChannelSubmitter) SubmitCloseTx(closeAgreement state.CloseAgreement) error {
	// Get close transaction.
	_, closeTx, err := s.Channel.CloseTxs(closeAgreement.Details)
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}

	// Attach signatures to close transaction.
	closeTx, err = closeTx.AddSignatureDecorated(closeAgreement.CloseSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the close tx: %w", err)
	}

	// Submit fee bump transaction that wraps the close transaction.
	err = s.Submitter.SubmitFeeBumpTx(closeTx)
	if err != nil {
		return fmt.Errorf("submitting close tx: %w", err)
	}

	return nil
}

// SubmitFormationTx submits the formation transaction for a channel wrapped in
// a fee bump transaction with the Submitter's FeeAccount as the fee account.
func (s ChannelSubmitter) SubmitFormationTx() error {
	oa := s.Channel.OpenAgreement()

	// Get formation transaction.
	_, _, formationTx, err := s.Channel.OpenTxs(oa.Details)
	if err != nil {
		return fmt.Errorf("building formation tx: %w", err)
	}

	// Attach signatures to formation	 transaction.
	formationTx, err = formationTx.AddSignatureDecorated(oa.FormationSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the formation tx: %w", err)
	}

	// Submit fee bump transaction that wraps the formation	 transaction.
	err = s.Submitter.SubmitFeeBumpTx(formationTx)
	if err != nil {
		return fmt.Errorf("submitting formation tx: %w", err)
	}

	return nil
}
