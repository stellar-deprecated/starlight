package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

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
	showHelp := false
	horizonURL := "http://localhost:8000"
	accountKeyStr := "G..."
	signerKeyStr := "S..."

	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&showHelp, "h", showHelp, "Show this help")
	fs.StringVar(&horizonURL, "horizon-url", horizonURL, "Horizon URL")
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
		fmt.Fprintf(os.Stderr, "account %s does not exist, attempting to create using network root key\n", accountKey.Address())
		err = fund(client, networkDetails.NetworkPassphrase, accountKey)
	}
	if err != nil {
		return err
	}

	conn := net.Conn(nil)

	br := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		line, err := br.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %#v", err)
			continue
		}
		params := strings.Fields(line)
		if len(params) == 0 {
			continue
		}
		switch params[0] {
		case "listen":
			if conn != nil {
				fmt.Fprintf(os.Stderr, "error: already connected to a peer")
				continue
			}
			ln, err := net.Listen("tcp", params[1])
			if err != nil {
				return err
			}
			conn, err := ln.Accept()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: accepting incoming conn: %v", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "connected to %v\n", conn.RemoteAddr())
		case "connect":
			if conn != nil {
				fmt.Fprintf(os.Stderr, "error: already connected to a peer")
				continue
			}
			outgoingConn, err := connect(params[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "connected to %v\n", outgoingConn.RemoteAddr())
			conn = outgoingConn
		case "open":
		case "close":
		case "exit":
			os.Exit(0)
		}
	}

	return nil
}

func connect(peerAddr string) (net.Conn, error) {
	return net.Dial("tcp", peerAddr)
}

func open(networkPassphrase string) error {
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
