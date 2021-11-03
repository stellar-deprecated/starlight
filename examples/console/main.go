package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/abiosoft/ishell"
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

var (
	asset              = state.Asset("")
	otherEscrowAccount = (*keypair.FromAddress)(nil)
	closeAgreements    = []state.CloseAgreement{}
)

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
				otherEscrowAccount = e.EscrowAccount
				fmt.Fprintf(os.Stderr, "connected\n")
			case agentpkg.OpenedEvent:
				asset = e.OpenAgreement.Envelope.Details.Asset
				fmt.Fprintf(os.Stderr, "channel opened for asset %v\n", asset)

			case agentpkg.PaymentReceivedEvent:
				closeAgreements = append(closeAgreements, e.CloseAgreement)
				// As this example uses the buffered agent, each
				// PaymentReceivedEvent, is a new CloseAgreement containing many
				// buffered payments. The BufferedPaymentsReceivedEvent will
				// also be triggered and will contain the buffered payments.
				stats.AddAgreementsReceived(1)
			case agentpkg.PaymentSentEvent:
				closeAgreements = append(closeAgreements, e.CloseAgreement)
				// As this example uses the buffered agent, each
				// PaymentSentEvent, is a new CloseAgreement containing many
				// buffered payments. The BufferedPaymentsSentEvent will also be
				// triggered and will contain the buffered payments.
				stats.AddAgreementsSent(1)

			case bufferedagent.BufferedPaymentsReceivedEvent:
				stats.AddBufferedPaymentsReceived(len(e.Payments))
				stats.AddBufferByteSize(e.BufferByteSize)
			case bufferedagent.BufferedPaymentsSentEvent:
				stats.AddBufferedPaymentsSent(len(e.Payments))
				stats.AddBufferByteSize(e.BufferByteSize)

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
		fmt.Fprintln(os.Stdout, "waiting before creating escrow")
		time.Sleep(5 * time.Second)
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
		err = retry(10, func() error {
			return submitter.SubmitTx(tx)
		})
		if err != nil {
			return fmt.Errorf("submitting tx to create escrow account: %w", err)
		}
		fmt.Fprintln(os.Stdout, "escrow created")
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
			err = http.ListenAndServe(":"+httpPort, cors.Default().Handler(&mux))
			if err != nil {
				fmt.Fprintf(os.Stdout, "error: http listening: %#v\n", err)
			}
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

	err = runShell(agent, stats, submitter, horizonClient, networkDetails.NetworkPassphrase, accountKey, escrowAccountKey, signerKey)
	if err != nil {
		fmt.Fprintf(os.Stdout, "error: %#v\n", err)
	}

	return nil
}

func runShell(agent *bufferedagent.Agent, stats *stats, submitter agentpkg.Submitter, horizonClient horizonclient.ClientInterface, networkPassphrase string, account, escrowAccount *keypair.FromAddress, signer *keypair.Full) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	shell := ishell.New()
	shell.AddCmd(&ishell.Cmd{
		Name: "listen",
		Help: "listen [addr]:<port> - listen for a peer to connect",
		Func: func(c *ishell.Context) {
			err := agent.ServeTCP(c.Args[0])
			if err != nil {
				c.Err(err)
			}
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "connect",
		Help: "connect <addr>:<port> - connect to a peer",
		Func: func(c *ishell.Context) {
			c.Err(agent.ConnectTCP(c.Args[0]))
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "open",
		Help: "open [asset-code] - open a channel with optional asset",
		Func: func(c *ishell.Context) {

			assetCode := ""
			if len(c.Args) >= 1 {
				assetCode = c.Args[0]
			}
			asset := state.NativeAsset
			if assetCode != "" && assetCode != "native" {
				asset = state.Asset(assetCode + ":" + signer.Address())
			}
			c.Err(agent.Open(asset))
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "deposit",
		Help: "deposit <amount> - deposit asset into escrow account",
		Func: func(c *ishell.Context) {
			depositAmountStr := c.Args[0]
			destination := escrowAccount
			if len(c.Args) >= 3 && c.Args[1] == "other" {
				destination = otherEscrowAccount
			}
			account, err := horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: account.Address()})
			if err != nil {
				c.Err(fmt.Errorf("getting state of local escrow account: %w", err))
			}
			tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
				SourceAccount:        &account,
				IncrementSequenceNum: true,
				BaseFee:              txnbuild.MinBaseFee,
				Timebounds:           txnbuild.NewTimeout(300),
				Operations: []txnbuild.Operation{
					&txnbuild.Payment{Destination: destination.Address(), Asset: asset.Asset(), Amount: depositAmountStr},
				},
			})
			if err != nil {
				c.Err(fmt.Errorf("building deposit payment tx: %w", err))
			}
			tx, err = tx.Sign(networkPassphrase, signer)
			if err != nil {
				c.Err(fmt.Errorf("signing deposit payment tx: %w", err))
			}
			_, err = horizonClient.SubmitTransaction(tx)
			if err != nil {
				c.Err(fmt.Errorf("submitting deposit payment tx: %w", err))
			}
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "pay",
		Help: "pay <amount> - pay amount of asset to peer",
		Func: func(c *ishell.Context) {
			amt, err := amount.ParseInt64(c.Args[0])
			if err != nil {
				c.Err(err)
			}
			stats.Reset()
			_, err = agent.Payment(amt)
			c.Err(err)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "payx",
		Help: "payx <amount> <number of times> - pay an amount a certain number of times",
		Func: func(c *ishell.Context) {
			fmt.Fprintf(os.Stdout, "sending %s payment %s times\n", c.Args[0], c.Args[1])
			amt, err := amount.ParseInt64(c.Args[0])
			if err != nil {
				c.Err(err)
			}
			x, err := strconv.Atoi(c.Args[1])
			if err != nil {
				c.Err(err)
			}
			bufferSize, err := strconv.Atoi(c.Args[2])
			if err != nil {
				c.Err(err)
			}
			stats.Reset()
			agent.SetMaxBufferSize(bufferSize)
			stats.MarkStart()
			amtRange := int64(bufferSize)
			if amtRange > amt {
				amtRange = amt
			}
			for i := 0; i < x; i++ {
				memo := "tx-" + strconv.Itoa(i)
				for {
					amt := amt - (int64(i) % amtRange)
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
			c.Err(err)
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "declareclose",
		Help: "declareclose - declare to close the channel",
		Func: func(c *ishell.Context) {
			c.Err(agent.DeclareClose())
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "close",
		Help: "close - close the channel",
		Func: func(c *ishell.Context) {
			c.Err(agent.Close())
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "listagreements",
		Help: "listagreements - list agreements/payments",
		Func: func(c *ishell.Context) {
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
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "declarecloseidx",
		Help: "declarecloseidx <idx> - declare to close the channel with a specific previous declaration tx",
		Func: func(c *ishell.Context) {
			idx, err := strconv.Atoi(c.Args[0])
			if err != nil {
				c.Err(err)
			}
			if idx >= len(closeAgreements) {
				c.Err(fmt.Errorf("invalid index, got %d must be between %d and %d", idx, 0, len(closeAgreements)-1))
			}
			tx := closeAgreements[idx].SignedTransactions().Declaration
			err = submitter.SubmitTx(tx)
			if err != nil {
				c.Err(err)
			}
		},
	})
	shell.AddCmd(&ishell.Cmd{
		Name: "closeidx",
		Help: "closeidx <idx> - close the channel with a specific previous close tx",
		Func: func(c *ishell.Context) {
			idx, err := strconv.Atoi(c.Args[0])
			if err != nil {
				c.Err(err)
			}
			tx := closeAgreements[idx].SignedTransactions().Close
			err = submitter.SubmitTx(tx)
			if err != nil {
				c.Err(err)
			}
		},
	})

	shell.Run()
	return
}
