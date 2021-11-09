// Package submit contains a type and logic for submitting transactions that need to be fee bumped.
//
// The Submitter type can be used to wrap any submission logic, and it will
// check if the transactions fee is below a threshold that indicates it needs to
// be fee bumped, and if so, it will wrap it in a fee bump transaction before
// submission.
//
// This package is intended for use in example payment channel implementations.
package submit

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

// Submitter submits transactions to the network via Horizon. If a transaction
// has a base fee below the submitters base fee, the transaction is wrapped in a
// fee bump transaction. This means fee-less transactions are wrapped in fee
// bump transaction.
//
// The BaseFee is the base fee that will be used for any submission where the
// transaction has a lower base fee.
type Submitter struct {
	SubmitTxer        interface{ SubmitTx(xdr string) error }
	NetworkPassphrase string
	BaseFee           int64
	FeeAccount        *keypair.FromAddress
	FeeAccountSigners []*keypair.Full
}

// SubmitTx submits the transaction. If the transaction has a base fee that is
// lower than the submitters base fee it is wrapped in a fee bump transaction
// with the Submitter's FeeAccount as the fee account.
func (s *Submitter) SubmitTx(tx *txnbuild.Transaction) error {
	if tx.BaseFee() < s.BaseFee {
		return s.submitTxWithFeeBump(tx)
	}
	return s.submitTx(tx)
}

func (s *Submitter) submitTx(tx *txnbuild.Transaction) error {
	txeBase64, err := tx.Base64()
	if err != nil {
		return fmt.Errorf("encoding tx as base64: %w", err)
	}
	err = s.SubmitTxer.SubmitTx(txeBase64)
	if err != nil {
		return fmt.Errorf("submitting tx: %w", buildErr(err))
	}
	return nil
}

func (s *Submitter) submitTxWithFeeBump(tx *txnbuild.Transaction) error {
	feeBumpTx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		BaseFee:    s.BaseFee,
		FeeAccount: s.FeeAccount.Address(),
	})
	if err != nil {
		return fmt.Errorf("building fee bump tx: %w", err)
	}
	feeBumpTx, err = feeBumpTx.Sign(s.NetworkPassphrase, s.FeeAccountSigners...)
	if err != nil {
		return fmt.Errorf("signing fee bump tx: %w", err)
	}
	txeBase64, err := feeBumpTx.Base64()
	if err != nil {
		return fmt.Errorf("encoding fee bump tx as base64: %w", err)
	}
	err = s.SubmitTxer.SubmitTx(txeBase64)
	if err != nil {
		return fmt.Errorf("submitting fee bump tx: %w", buildErr(err))
	}

	return nil
}

func buildErr(err error) error {
	if hErr := horizonclient.GetError(err); hErr != nil {
		resultString, rErr := hErr.ResultString()
		if rErr != nil {
			resultString = "<error getting result string: " + rErr.Error() + ">"
		}
		return fmt.Errorf("%w (%v)", err, resultString)
	}
	return err
}
