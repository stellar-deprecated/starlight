package main

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

// Submitter submits fee-less transactions to the network via Horizon by
// wrapping them in a fee bump transaction.
//
// Submitter is intended for use with transactions that have a zero fee and were
// not intended for submitting on their own.
//
// The BaseFee is the base fee that will be used for any submission.
type Submitter struct {
	HorizonClient     horizonclient.ClientInterface
	NetworkPassphrase string
	BaseFee           int64
	FeeAccount        *keypair.FromAddress
	FeeAccountSigners []*keypair.Full
}

// SubmitFeeBumpTx submits the transaction wrapped in a fee bump transaction
// with the Submitter's FeeAccount as the fee account.
func (s *Submitter) SubmitFeeBumpTx(tx *txnbuild.Transaction) error {
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
	_, err = s.HorizonClient.SubmitFeeBumpTransaction(feeBumpTx)
	if err != nil {
		resultString := "<->"
		if hErr := horizonclient.GetError(err); hErr != nil {
			var err error
			resultString, err = hErr.ResultString()
			if err != nil {
				resultString = "<error getting result string: " + err.Error() + ">"
			}
		}
		return fmt.Errorf("submitting fee bump tx: %w (%v)", err, resultString)
	}

	return nil
}
