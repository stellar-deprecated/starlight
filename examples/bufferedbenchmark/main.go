package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	agentpkg "github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/agent/agenthttp"
	"github.com/stellar/experimental-payment-channels/sdk/agent/bufferedagent"
	"github.com/stellar/experimental-payment-channels/sdk/horizon"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/submit"
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

func run() error {
	showHelp := false
	horizonURL := "http://localhost:8000"
	listenAddr := ""
	connectAddr := ""
	cpuProfileFile := ""
	paymentsToSend := 50_000_000
	maxBufferSize := 20_000_000
	httpPort := ""

	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.BoolVar(&showHelp, "h", showHelp, "Show this help")
	fs.StringVar(&horizonURL, "horizon", horizonURL, "Horizon URL")
	fs.StringVar(&listenAddr, "listen", listenAddr, "Address to listen on in listen mode")
	fs.StringVar(&connectAddr, "connect", connectAddr, "Address to connect to in connect mode")
	fs.StringVar(&cpuProfileFile, "cpuprofile", cpuProfileFile, "Write cpu profile to `file`")
	fs.IntVar(&paymentsToSend, "count", paymentsToSend, "Number of payments to send")
	fs.IntVar(&maxBufferSize, "max-buffer", maxBufferSize, "Max buffer size")
	fs.StringVar(&httpPort, "port", httpPort, "Port to serve API on")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if showHelp {
		fs.Usage()
		return nil
	}
	if listenAddr == "" && connectAddr == "" {
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

	accountKey := keypair.MustRandom()
	multisigAccountKey := keypair.MustRandom()

	horizonClient := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := horizonClient.Root()
	if err != nil {
		return err
	}

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
		FeeAccount:        accountKey.FromAddress(),
		FeeAccountSigners: []*keypair.Full{accountKey},
	}

	fmt.Fprintf(os.Stdout, "creating main account %s with network root\n", accountKey.Address())
	err = createAccountWithRoot(horizonClient, networkDetails.NetworkPassphrase, accountKey)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "creating multisig account %s with network root\n", multisigAccountKey.Address())
	err = createAccountWithSignerWithRoot(horizonClient, networkDetails.NetworkPassphrase, multisigAccountKey, accountKey.FromAddress())
	if err != nil {
		return err
	}

	// Wait for state of multisig accounts to be ingested by Horizon.
	time.Sleep(2 * time.Second)

	underlyingEvents := make(chan interface{})
	config := agentpkg.Config{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		MaxOpenExpiry:              maxOpenExpiry,
		NetworkPassphrase:          networkDetails.NetworkPassphrase,
		SequenceNumberCollector:    sequenceNumberCollector,
		BalanceCollector:           balanceCollector,
		Submitter:                  submitter,
		Streamer:                   streamer,
		MultiSigAccountKey:         multisigAccountKey.FromAddress(),
		MultiSigAccountSigner:      accountKey,
		LogWriter:                  io.Discard,
		Events:                     underlyingEvents,
	}
	underlyingAgent := agentpkg.NewAgent(config)
	events := make(chan interface{})
	bufferedConfig := bufferedagent.Config{
		Agent:         underlyingAgent,
		AgentEvents:   underlyingEvents,
		MaxBufferSize: maxBufferSize,
		LogWriter:     io.Discard,
		Events:        events,
	}
	agent := bufferedagent.NewAgent(bufferedConfig)

	if httpPort != "" {
		agentHandler := agenthttp.New(underlyingAgent)
		fmt.Fprintf(os.Stdout, "agent http served on :%s\n", httpPort)
		go func() {
			_ = http.ListenAndServe(":"+httpPort, agentHandler)
		}()
	}

	var timeStarted, timeFinished time.Time
	bufferedPaymentsSent := 0
	bufferedPaymentsSentConfirmed := 0
	bufferedPaymentsReceived := 0
	paymentsSent := 0
	paymentsReceived := 0

	done := make(chan struct{})
	open := make(chan struct{})
	go func() {
		defer close(done)
		for {
			switch e := (<-events).(type) {
			case agentpkg.ErrorEvent:
				fmt.Fprintf(os.Stderr, "%v\n", e.Err)
			case agentpkg.ConnectedEvent:
				fmt.Fprintf(os.Stderr, "connected\n")
				if connectAddr != "" {
					_ = agent.Open(state.NativeAsset)
				}
			case agentpkg.OpenedEvent:
				fmt.Fprintf(os.Stderr, "opened\n")
				close(open)
			case bufferedagent.BufferedPaymentsReceivedEvent:
				if timeStarted.IsZero() {
					timeStarted = time.Now()
				}
				paymentsReceived++
				bufferedPaymentsReceived += len(e.Payments)
				timeFinished = time.Now()
			case bufferedagent.BufferedPaymentsSentEvent:
				paymentsSent++
				bufferedPaymentsSentConfirmed += len(e.Payments)
			case agentpkg.ClosingEvent:
				fmt.Fprintf(os.Stderr, "closing\n")
			case agentpkg.ClosedEvent:
				fmt.Fprintf(os.Stderr, "closed\n")
				return
			}
		}
	}()

	if listenAddr != "" {
		fmt.Fprintf(os.Stdout, "listening on %s\n", listenAddr)
		err = agent.ServeTCP(listenAddr)
		if err != nil {
			return err
		}
	}
	if connectAddr != "" {
		fmt.Fprintf(os.Stdout, "connecting to %s\n", connectAddr)
		err = agent.ConnectTCP(connectAddr)
		if err != nil {
			return err
		}
	}

	<-open

	if connectAddr != "" {
		time.Sleep(2 * time.Second)

		timeStarted = time.Now()

		for i := 0; i < paymentsToSend; i++ {
			for {
				_, err := agent.Payment(1)
				if err != nil {
					agent.Wait()
					continue
				}
				break
			}
			bufferedPaymentsSent++
		}

		fmt.Printf("waiting for all payments to finish\n")
		agent.Wait()

		timeFinished = time.Now()

		err = agent.DeclareClose()
		if err != nil {
			panic(err)
		}
	}

	<-done

	timeSpent := timeFinished.Sub(timeStarted)
	fmt.Fprintf(os.Stderr, "time spent: %v\n", timeSpent)
	fmt.Fprintf(os.Stderr, "buffered payments sent: %d\n", bufferedPaymentsSent)
	fmt.Fprintf(os.Stderr, "buffered payments received: %d\n", bufferedPaymentsReceived)
	fmt.Fprintf(os.Stderr, "buffered payments tps: %.3f\n", float64(bufferedPaymentsSent+bufferedPaymentsReceived)/timeSpent.Seconds())
	fmt.Fprintf(os.Stderr, "payments sent: %d\n", paymentsSent)
	fmt.Fprintf(os.Stderr, "payments received: %d\n", paymentsReceived)
	fmt.Fprintf(os.Stderr, "payments tps: %.3f\n", float64(paymentsSent+paymentsReceived)/timeSpent.Seconds())

	fmt.Print("Press 'Enter' to exit...")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')

	return nil
}
