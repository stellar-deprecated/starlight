package main

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func SubmitFeeBumpTx(client horizonclient.ClientInterface, networkPassphrase string, tx *txnbuild.Transaction, feeAccount *keypair.FromAddress, feeAccountSigner *keypair.Full) error {
	feeBumpTx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		BaseFee:    txnbuild.MinBaseFee,
		FeeAccount: feeAccount.Address(),
	})
	if err != nil {
		return fmt.Errorf("building fee bump tx: %w", err)
	}
	feeBumpTx, err = feeBumpTx.Sign(networkPassphrase, feeAccountSigner)
	if err != nil {
		return fmt.Errorf("signing fee bump tx: %w", err)
	}
	_, err = client.SubmitFeeBumpTransaction(feeBumpTx)
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
