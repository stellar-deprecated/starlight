package main

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
)

const networkPassphrase = "Standalone Network ; February 2017"

var root = func() *keypair.Full {
	// TODO: Add this logic as a root key function to the network package.
	kp, err := keypair.FromRawSeed(network.ID(networkPassphrase))
	if err != nil {
		panic(err)
	}
	return kp
}()

const horizonURL = "http://localhost:8000"
var client = horizonclient.Client{HorizonURL: horizonURL}

func main() {
	var err error

	initiator := keypair.MustRandom()
	err = friendbot(initiator.FromAddress(), "10000.0")
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}

	ei := keypair.MustRandom()
	err = setupEscrowAccount(initiator, ei, "1000.0")
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}

	responder := keypair.MustRandom()
	err = friendbot(responder.FromAddress(), "10000.0")
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}

	er := keypair.MustRandom()
	err = setupEscrowAccount(responder, er, "2000.0")
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}

	// s := 0 // TODO: set to E's seq
	// i := 0
	// e := 0

	f, err := buildFormationTx(initiator.FromAddress(), responder.FromAddress(), ei.FromAddress(), er.FromAddress())
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
	f, err = f.Sign(networkPassphrase, initiator, responder)
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
	_, err = client.SubmitTransaction(f)
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
}

func buildFormationTx(i *keypair.FromAddress, r *keypair.FromAddress, ei *keypair.FromAddress, er *keypair.FromAddress) (*txnbuild.Transaction, error) {
	var err error

	eia, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: ei.Address()})
	if err != nil {
		return nil, err
	}
	tp := txnbuild.TransactionParams{
		SourceAccount:        &eia,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
	}
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: i.Address(), SponsoredID: ei.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   ei.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: r.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: ei.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: r.Address(), SponsoredID: er.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   er.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: i.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: er.Address()})
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func setupEscrowAccount(creator *keypair.Full, account *keypair.Full, initialContribution string) error {
	var err error

	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: creator.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.BeginSponsoringFutureReserves{
					SponsoredID: account.Address(),
				},
				&txnbuild.CreateAccount{
					Destination: account.Address(),
					Amount:      initialContribution,
				},
				&txnbuild.SetOptions{
					SourceAccount:   account.Address(),
					MasterWeight:    txnbuild.NewThreshold(0),
					LowThreshold:    txnbuild.NewThreshold(1),
					MediumThreshold: txnbuild.NewThreshold(1),
					HighThreshold:   txnbuild.NewThreshold(1),
					Signer:          &txnbuild.Signer{Address: creator.Address(), Weight: 1},
				},
				&txnbuild.EndSponsoringFutureReserves{
					SourceAccount: account.Address(),
				},
			},
		},
	)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(networkPassphrase, creator, account)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}

func friendbot(account *keypair.FromAddress, startingBalance string) error {
	var err error

	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: root.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.CreateAccount{
					Destination: account.Address(),
					Amount:      startingBalance,
				},
			},
		},
	)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(networkPassphrase, root)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}
