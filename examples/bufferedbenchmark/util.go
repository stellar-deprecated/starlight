package main

import (
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func createAccountWithRoot(client horizonclient.ClientInterface, networkPassphrase string, accountKey *keypair.Full) error {
	rootKey := keypair.Root(networkPassphrase)
	root, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: rootKey.Address()})
	if err != nil {
		return err
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        &root,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		Operations: []txnbuild.Operation{
			&txnbuild.CreateAccount{
				Destination: accountKey.Address(),
				Amount:      "1000000",
			},
		},
	}
	tx, err := txnbuild.NewTransaction(txParams)
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

func createAccountWithSignerWithRoot(client horizonclient.ClientInterface, networkPassphrase string, accountKey *keypair.Full, signerKey *keypair.FromAddress) error {
	rootKey := keypair.Root(networkPassphrase)
	root, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: rootKey.Address()})
	if err != nil {
		return err
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        &root,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		Operations: []txnbuild.Operation{
			&txnbuild.CreateAccount{
				Destination: accountKey.Address(),
				Amount:      "1000000",
			},
			&txnbuild.SetOptions{
				SourceAccount: accountKey.Address(),
				MasterWeight:  txnbuild.NewThreshold(0),
				Signer:        &txnbuild.Signer{Address: signerKey.Address(), Weight: 1},
			},
		},
	}
	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(networkPassphrase, rootKey, accountKey)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}
