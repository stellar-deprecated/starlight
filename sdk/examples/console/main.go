package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}

func run() error {
	horizonURL := "http://localhost:8000"
	showHelp := false
	accountKeyStr := ""
	signerKeyStr := ""

	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&horizonURL, "horizon-url", horizonURL, "Horizon URL")
	fs.BoolVar(&showHelp, "h", showHelp, "Show this help")
	fs.StringVar(&accountKeyStr, "account", accountKeyStr, "Account G address")
	fs.StringVar(&signerKeyStr, "signer", signerKeyStr, "Account S signer")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if showHelp {
		fs.Usage()
		return nil
	}

	if accountKeyStr == "" {
		return fmt.Errorf("-account required")
	}
	if signerKeyStr == "" {
		return fmt.Errorf("-signer required")
	}

	accountKey, err := keypair.ParseAddress(accountKeyStr)
	if err != nil {
		return fmt.Errorf("cannot parse -account: %w", err)
	}
	_ /* signerKey */, err = keypair.ParseFull(signerKeyStr)
	if err != nil {
		return fmt.Errorf("cannot parse -signer: %w", err)
	}

	client := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := client.Root()
	if err != nil {
		return err
	}

	_, err = client.AccountDetail(horizonclient.AccountRequest{AccountID: accountKey.Address()})
	if horizonclient.IsNotFoundError(err) {
		err = fund(client, networkDetails.NetworkPassphrase, accountKey)
	}
	if err != nil {
		return err
	}

	for {
		// wait for incoming request to connect
		// or, wait for message typed in with instruction:
		//   connect
		//   open
		//   pay
		//   close
		time.Sleep(time.Second)
	}

	return nil
}

func fund(client horizonclient.ClientInterface, networkPassphrase string, accountKey *keypair.FromAddress) error {
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
				Amount:      "10000",
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

func connect(networkPassphrase string) error {
	c := state.Config{
		NetworkPassphrase:   networkPassphrase,
		Initiator:           true,
		LocalEscrowAccount:  &state.EscrowAccount{},
		RemoteEscrowAccount: &state.EscrowAccount{},
		// LocalSigner:         localSigner,
		// RemoteSigner:        remoteSigner,
	}
	state.NewChannel(c)
	return nil
}
