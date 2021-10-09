package main

import (
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func createAccountWithRoot(client horizonclient.ClientInterface, networkPassphrase string, accountKey *keypair.FromAddress) error {
	rootKey := keypair.Root(networkPassphrase)
	root, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: rootKey.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &root,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		Operations: []txnbuild.Operation{
			&txnbuild.CreateAccount{
				Destination: accountKey.Address(),
				Amount:      "10000000",
			},
		},
	})
	if err != nil {
		return err
	}
	tx, err = tx.Sign(networkPassphrase, rootKey)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}
