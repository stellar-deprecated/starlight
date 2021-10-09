package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/rs/cors"
	agentpkg "github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/agent/agenthttp"
	"github.com/stellar/experimental-payment-channels/sdk/agent/bufferedagent"
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
	maxOpenExpiry              = 5 * time.Hour
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
	signerKeyStr := "S..."
	filename := ""
	httpPort := ""
	listenPort := ""
	connectAddr := ""
	cpuProfileFile := ""

	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.BoolVar(&showHelp, "h", showHelp, "Show this help")
	fs.StringVar(&horizonURL, "horizon", horizonURL, "Horizon URL")
	fs.StringVar(&httpPort, "port", httpPort, "Port to serve API on")
	fs.StringVar(&signerKeyStr, "signer", signerKeyStr, "Account S signer")
	fs.StringVar(&filename, "f", filename, "File to write and load channel state")
	fs.StringVar(&listenPort, "listen-port", listenPort, "Listen on port")
	fs.StringVar(&connectAddr, "connect-addr", connectAddr, "Address to connect to")
	fs.StringVar(&cpuProfileFile, "cpuprofile", cpuProfileFile, "Write cpu profile to `file`")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if showHelp {
		fs.Usage()
		return nil
	}

	if cpuProfileFile != "" {
		f, err := os.Create(cpuProfileFile)
		if err != nil {
			return fmt.Errorf("error creating cpu profile file: %w", err)
		}
		defer f.Close()
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return fmt.Errorf("error starting cpu profile: %w", err)
		}
		defer pprof.StopCPUProfile()
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

	var signerKey *keypair.Full
	if signerKeyStr != "" && signerKeyStr != "S..." {
		signerKey, err = keypair.ParseFull(signerKeyStr)
		if err != nil {
			return fmt.Errorf("cannot parse -signer: %w", err)
		}
	} else {
		signerKey = keypair.MustRandom()
	}
	accountKey := signerKey.FromAddress()

	horizonClient := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := horizonClient.Root()
	if err != nil {
		return err
	}

	stats := &stats{}

	events := make(chan interface{})
	go func() {
		for {
			switch e := (<-events).(type) {
			case agentpkg.ErrorEvent:
				fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
			case agentpkg.ConnectedEvent:
				fmt.Fprintf(os.Stderr, "connected\n")
			case agentpkg.OpenedEvent:
				fmt.Fprintf(os.Stderr, "channel opened\n")

			case agentpkg.PaymentReceivedEvent:
				closeAgreements = append(closeAgreements, e.CloseAgreement)
				stats.AddPaymentsReceived(1)
			case agentpkg.PaymentSentEvent:
				closeAgreements = append(closeAgreements, e.CloseAgreement)
				stats.AddPaymentsSent(1)

			case bufferedagent.BufferedPaymentsReceivedEvent:
				stats.AddBufferedPaymentsReceived(len(e.Payments))
			case bufferedagent.BufferedPaymentsSentEvent:
				stats.AddBufferedPaymentsSent(len(e.Payments))

			case agentpkg.ClosingEvent:
				fmt.Fprintf(os.Stderr, "channel closing\n")
			case agentpkg.ClosedEvent:
				fmt.Fprintf(os.Stderr, "channel closed\n")
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
	var underlyingAgent *agentpkg.Agent
	underlyingEvents := make(chan interface{})
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
			LogWriter:                  io.Discard,
			Events:                     underlyingEvents,
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
		underlyingAgent = agentpkg.NewAgent(config)

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
			LogWriter:           io.Discard,
			Events:              underlyingEvents,
		}
		underlyingAgent = agentpkg.NewAgentFromSnapshot(config, file.Snapshot)
	}
	bufferedConfig := bufferedagent.Config{
		Agent:         underlyingAgent,
		AgentEvents:   underlyingEvents,
		MaxBufferSize: 1,
		LogWriter:     io.Discard,
		Events:        events,
	}
	agent := bufferedagent.NewAgent(bufferedConfig)

	if httpPort != "" {
		agentHandler := agenthttp.New(underlyingAgent)
		fmt.Fprintf(os.Stdout, "agent http served on :%s\n", httpPort)
		mux := http.ServeMux{}
		mux.Handle("/", agentHandler)
		mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(stats)
			if err != nil {
				panic(err)
			}
		})
		go func() {
			_ = http.ListenAndServe(":"+httpPort, cors.Default().Handler(&mux))
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
		err = prompt(agent, stats, submitter, horizonClient, networkDetails.NetworkPassphrase, accountKey, escrowAccountKey, signerKey, params)
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

func prompt(agent *bufferedagent.Agent, stats *stats, submitter agentpkg.Submitter, horizonClient horizonclient.ClientInterface, networkPassphrase string, account, escrowAccount *keypair.FromAddress, signer *keypair.Full, params []string) (err error) {
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
		stats.Reset()
		_, err = agent.Payment(amt)
		return err
	case "payx":
		fmt.Fprintf(os.Stdout, "sending %s payment %s times\n", params[1], params[2])
		amt, err := amount.ParseInt64(params[1])
		if err != nil {
			return err
		}
		x, err := strconv.Atoi(params[2])
		if err != nil {
			return err
		}
		bufferSize, err := strconv.Atoi(params[3])
		if err != nil {
			return err
		}
		stats.Reset()
		agent.SetMaxBufferSize(bufferSize)
		stats.MarkStart()
		for i := 0; i < x; i++ {
			memo := "tx-" + strconv.Itoa(i)
			for {
				_, err = agent.PaymentWithMemo(amt, memo)
				if err != nil {
					continue
				}
				break
			}
		}
		agent.Wait()
		for stats.bufferedPaymentsSent != int64(x) {
			// Wait for all the buffered payments to have been sent.
		}
		stats.MarkFinish()
		fmt.Println(stats.Summary())
		agent.SetMaxBufferSize(1)
		return err
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
