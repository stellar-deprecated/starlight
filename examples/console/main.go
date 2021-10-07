package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	agentpkg "github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/agent/agenthttp"
	"github.com/stellar/experimental-payment-channels/sdk/horizon"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/submit"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

const (
	observationPeriodTime      = 10 * time.Second
	observationPeriodLedgerGap = 1
	maxOpenExpiry              = 5 * time.Minute
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stdout, "error:", err)
	}
}

var closeAgreements = []state.CloseAgreement{}

func run() error {
	showHelp := false
	horizonURL := "http://localhost:8000"
	accountKeyStr := "G..."
	signerKeyStr := "S..."
	filename := ""
	httpPort := ""
	listenPort := ""
	connectAddr := ""

	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.BoolVar(&showHelp, "h", showHelp, "Show this help")
	fs.StringVar(&horizonURL, "horizon", horizonURL, "Horizon URL")
	fs.StringVar(&httpPort, "stats-port", httpPort, "Port to serve snapshots on")
	fs.StringVar(&accountKeyStr, "account", accountKeyStr, "Account G address")
	fs.StringVar(&signerKeyStr, "signer", signerKeyStr, "Account S signer")
	fs.StringVar(&filename, "f", filename, "File to write and load channel state")
	fs.StringVar(&listenPort, "listen-port", listenPort, "Listen on port")
	fs.StringVar(&connectAddr, "connect-addr", connectAddr, "Address to connect to")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if showHelp {
		fs.Usage()
		return nil
	}
	if accountKeyStr == "" || accountKeyStr == "G..." {
		return fmt.Errorf("-account required")
	}
	if signerKeyStr == "" || accountKeyStr == "S..." {
		return fmt.Errorf("-signer required")
	}

	var file *File
	if filename != "" {
		fmt.Printf("loading file: %s\n", filename)
		fileBytes, err := ioutil.ReadFile(filename)
		if os.IsNotExist(err) {
			fmt.Printf("file doesn't exist and will be created when saving state: %s\n", filename)
		} else {
			if err != nil {
				return fmt.Errorf("reading file %s: %w", filename, err)
			}
			file = &File{}
			err = json.Unmarshal(fileBytes, file)
			if err != nil {
				return fmt.Errorf("json decoding file %s: %w", filename, err)
			}
			fmt.Printf("loaded state from file: %s\n", filename)
		}
	}

	accountKey, err := keypair.ParseAddress(accountKeyStr)
	if err != nil {
		return fmt.Errorf("cannot parse -account: %w", err)
	}
	signerKey, err := keypair.ParseFull(signerKeyStr)
	if err != nil {
		return fmt.Errorf("cannot parse -signer: %w", err)
	}

	horizonClient := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := horizonClient.Root()
	if err != nil {
		return err
	}

	events := make(chan agentpkg.Event)
	go func() {
		for {
			switch e := (<-events).(type) {
			case agentpkg.ErrorEvent:
				fmt.Fprintf(os.Stderr, "agent error: %v\n", e.Err)
			case agentpkg.ConnectedEvent:
				fmt.Fprintf(os.Stderr, "agent connected\n")
			case agentpkg.OpenedEvent:
				fmt.Fprintf(os.Stderr, "agent channel opened\n")
			case agentpkg.PaymentReceivedEvent:
				fmt.Fprintf(os.Stderr, "agent channel received payment: iteration=%d balance=%s\n", e.CloseAgreement.Envelope.Details.IterationNumber, amount.StringFromInt64(e.CloseAgreement.Envelope.Details.Balance))
				closeAgreements = append(closeAgreements, e.CloseAgreement)
			case agentpkg.PaymentSentEvent:
				fmt.Fprintf(os.Stderr, "agent channel sent payment and other participant confirmed: iteration=%d balance=%s\n", e.CloseAgreement.Envelope.Details.IterationNumber, amount.StringFromInt64(e.CloseAgreement.Envelope.Details.Balance))
				closeAgreements = append(closeAgreements, e.CloseAgreement)
			case agentpkg.ClosingEvent:
				fmt.Fprintf(os.Stderr, "agent channel closing\n")
			case agentpkg.ClosedEvent:
				fmt.Fprintf(os.Stderr, "agent channel closed\n")
			}
		}
	}()

	streamer := &horizon.Streamer{
		HorizonClient: horizonClient,
		ErrorHandler: func(err error) {
			fmt.Fprintf(os.Stderr, "horizon streamer error: %v\n", err)
		},
	}
	balanceCollector := &horizon.BalanceCollector{HorizonClient: horizonClient}
	sequenceNumberCollector := &horizon.SequenceNumberCollector{HorizonClient: horizonClient}
	submitter := &submit.Submitter{
		SubmitTxer:        &horizon.Submitter{HorizonClient: horizonClient},
		NetworkPassphrase: networkDetails.NetworkPassphrase,
		BaseFee:           txnbuild.MinBaseFee,
		FeeAccount:        accountKey,
		FeeAccountSigners: []*keypair.Full{signerKey},
	}

	var escrowAccountKey *keypair.FromAddress
	var agent *agentpkg.Agent
	if file == nil {
		account, err := horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountKey.Address()})
		if horizonclient.IsNotFoundError(err) {
			fmt.Fprintf(os.Stdout, "account %s does not exist, attempting to create using network root key\n", accountKey.Address())
			err = createAccountWithRoot(horizonClient, networkDetails.NetworkPassphrase, accountKey)
			if err != nil {
				return err
			}
			account, err = horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountKey.Address()})
		}
		if err != nil {
			return err
		}
		accountSeqNum, err := account.GetSequenceNumber()
		if err != nil {
			return err
		}

		escrowAccountKeyFull := keypair.MustRandom()
		escrowAccountKey = escrowAccountKeyFull.FromAddress()
		fmt.Fprintln(os.Stdout, "escrow account:", escrowAccountKey.Address())

		config := agentpkg.Config{
			ObservationPeriodTime:      observationPeriodTime,
			ObservationPeriodLedgerGap: observationPeriodLedgerGap,
			MaxOpenExpiry:              maxOpenExpiry,
			NetworkPassphrase:          networkDetails.NetworkPassphrase,
			SequenceNumberCollector:    sequenceNumberCollector,
			BalanceCollector:           balanceCollector,
			Submitter:                  submitter,
			Streamer:                   streamer,
			EscrowAccountKey:           escrowAccountKey,
			EscrowAccountSigner:        signerKey,
			LogWriter:                  os.Stderr,
			Events:                     events,
		}
		if filename != "" {
			config.Snapshotter = JSONFileSnapshotter{
				Filename:                   filename,
				ObservationPeriodTime:      observationPeriodTime,
				ObservationPeriodLedgerGap: observationPeriodLedgerGap,
				MaxOpenExpiry:              maxOpenExpiry,
				EscrowAccountKey:           escrowAccountKey,
			}
		}
		agent = agentpkg.NewAgent(config)

		tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
			Creator:        accountKey.FromAddress(),
			Escrow:         escrowAccountKey.FromAddress(),
			SequenceNumber: accountSeqNum + 1,
			Asset:          txnbuild.NativeAsset{},
		})
		if err != nil {
			return fmt.Errorf("creating escrow account tx: %w", err)
		}
		tx, err = tx.Sign(networkDetails.NetworkPassphrase, signerKey, escrowAccountKeyFull)
		if err != nil {
			return fmt.Errorf("signing tx to create escrow account: %w", err)
		}
		err = submitter.SubmitTx(tx)
		if err != nil {
			return fmt.Errorf("submitting tx to create escrow account: %w", err)
		}
	} else {
		escrowAccountKey = file.EscrowAccountKey
		config := agentpkg.Config{
			ObservationPeriodTime:      file.ObservationPeriodTime,
			ObservationPeriodLedgerGap: file.ObservationPeriodLedgerGap,
			MaxOpenExpiry:              file.MaxOpenExpiry,
			NetworkPassphrase:          networkDetails.NetworkPassphrase,
			SequenceNumberCollector:    sequenceNumberCollector,
			BalanceCollector:           balanceCollector,
			Submitter:                  submitter,
			Streamer:                   streamer,
			Snapshotter: JSONFileSnapshotter{
				Filename:                   filename,
				ObservationPeriodTime:      file.ObservationPeriodTime,
				ObservationPeriodLedgerGap: file.ObservationPeriodLedgerGap,
				MaxOpenExpiry:              file.MaxOpenExpiry,
				EscrowAccountKey:           escrowAccountKey,
			},
			EscrowAccountKey:    escrowAccountKey,
			EscrowAccountSigner: signerKey,
			LogWriter:           os.Stderr,
			Events:              events,
		}
		agent = agentpkg.NewAgentFromSnapshot(config, file.Snapshot)
	}

	if httpPort != "" {
		agentHandler := agenthttp.New(agent)
		fmt.Fprintf(os.Stdout, "agent http served on :%s\n", httpPort)
		go func() {
			_ = http.ListenAndServe(":"+httpPort, agentHandler)
		}()
	}

	if listenPort != "" {
		err := agent.ServeTCP(":" + listenPort)
		if err != nil {
			fmt.Fprintf(os.Stdout, "error: %#v\n", err)
		}
	}
	if connectAddr != "" {
		err := agent.ConnectTCP(connectAddr)
		if err != nil {
			fmt.Fprintf(os.Stdout, "error: %#v\n", err)
		}
	}

	br := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		line, err := br.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stdout, "error: %#v\n", err)
			continue
		}
		params := strings.Fields(line)
		if len(params) == 0 {
			continue
		}
		err = prompt(agent, submitter, horizonClient, networkDetails.NetworkPassphrase, accountKey, escrowAccountKey, signerKey, params)
		if errors.Is(err, errExit) {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stdout, "error: %#v\n", err)
			continue
		}
	}

	return nil
}

var errExit = errors.New("exit")

func prompt(agent *agentpkg.Agent, submitter agentpkg.Submitter, horizonClient horizonclient.ClientInterface, networkPassphrase string, account, escrowAccount *keypair.FromAddress, signer *keypair.Full, params []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	switch params[0] {
	case "help":
		fmt.Fprintf(os.Stdout, "listen [addr]:<port> - listen for a peer to connect\n")
		fmt.Fprintf(os.Stdout, "connect <addr>:<port> - connect to a peer\n")
		fmt.Fprintf(os.Stdout, "open - open a channel with asset\n")
		fmt.Fprintf(os.Stdout, "deposit <amount> - deposit asset into escrow account\n")
		fmt.Fprintf(os.Stdout, "pay <amount> - pay amount of asset to peer\n")
		fmt.Fprintf(os.Stdout, "declareclose - declare to close the channel\n")
		fmt.Fprintf(os.Stdout, "close - close the channel\n")
		fmt.Fprintf(os.Stdout, "listagreements - list agreements/payments\n")
		fmt.Fprintf(os.Stdout, "declarecloseidx - declare to close the channel with a specific previous declaration tx\n")
		fmt.Fprintf(os.Stdout, "closeidx - close the channel with a specific previous close tx\n")
		fmt.Fprintf(os.Stdout, "exit - exit the application\n")
		return nil
	case "listen":
		return agent.ServeTCP(params[1])
	case "connect":
		return agent.ConnectTCP(params[1])
	case "open":
		return agent.Open()
	case "pay":
		amt, err := amount.ParseInt64(params[1])
		if err != nil {
			return err
		}
		return agent.Payment(amt)
	case "declareclose":
		return agent.DeclareClose()
	case "close":
		return agent.Close()
	case "listagreements":
		for i, a := range closeAgreements {
			var sender string
			if signer.FromAddress().Equal(a.Envelope.Details.ProposingSigner) {
				sender = "me"
			} else {
				sender = "them"
			}
			var receiver string
			if signer.FromAddress().Equal(a.Envelope.Details.ConfirmingSigner) {
				receiver = "me"
			} else {
				receiver = "them"
			}
			payment := amount.StringFromInt64(a.Envelope.Details.PaymentAmount)
			balance := amount.StringFromInt64(a.Envelope.Details.Balance)
			fmt.Fprintf(os.Stdout, "%d: amount=%s %s=>%s balance=%s\n", i, payment, sender, receiver, balance)
		}
		return nil
	case "declarecloseidx":
		idx, err := strconv.Atoi(params[1])
		if err != nil {
			return err
		}
		if idx >= len(closeAgreements) {
			return fmt.Errorf("invalid index, got %d must be between %d and %d", idx, 0, len(closeAgreements)-1)
		}
		tx := closeAgreements[idx].SignedTransactions().Declaration
		err = submitter.SubmitTx(tx)
		if err != nil {
			return err
		}
		return nil
	case "closeidx":
		idx, err := strconv.Atoi(params[1])
		if err != nil {
			return err
		}
		tx := closeAgreements[idx].SignedTransactions().Close
		err = submitter.SubmitTx(tx)
		if err != nil {
			return err
		}
		return nil
	case "deposit":
		depositAmountStr := params[1]
		account, err := horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: account.Address()})
		if err != nil {
			return fmt.Errorf("getting state of local escrow account: %w", err)
		}
		tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
			SourceAccount:        &account,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{Destination: escrowAccount.Address(), Asset: txnbuild.NativeAsset{}, Amount: depositAmountStr},
			},
		})
		if err != nil {
			return fmt.Errorf("building deposit payment tx: %w", err)
		}
		tx, err = tx.Sign(networkPassphrase, signer)
		if err != nil {
			return fmt.Errorf("signing deposit payment tx: %w", err)
		}
		_, err = horizonClient.SubmitTransaction(tx)
		if err != nil {
			return fmt.Errorf("submitting deposit payment tx: %w", err)
		}
		return nil
	case "exit":
		return errExit
	default:
		return fmt.Errorf("error: unrecognized command")
	}
}
