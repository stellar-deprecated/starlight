package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/amount"
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
	fs.StringVar(&horizonURL, "horizon", horizonURL, "Horizon URL")
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
	if accountKeyStr == "" || accountKeyStr == "G..." {
		return fmt.Errorf("-account required")
	}
	if signerKeyStr == "" || accountKeyStr == "S..." {
		return fmt.Errorf("-signer required")
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

	account, err := horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountKey.Address()})
	if horizonclient.IsNotFoundError(err) {
		fmt.Fprintf(os.Stderr, "account %s does not exist, attempting to create using network root key\n", accountKey.Address())
		err = createAccountWithRoot(horizonClient, networkDetails.NetworkPassphrase, accountKey)
	}
	if err != nil {
		return err
	}
	accountSeqNum, err := account.GetSequenceNumber()
	if err != nil {
		return err
	}

	submitter := &Submitter{
		HorizonClient:     horizonClient,
		NetworkPassphrase: networkDetails.NetworkPassphrase,
		BaseFee:           txnbuild.MinBaseFee,
		FeeAccount:        accountKey,
		FeeAccountSigners: []*keypair.Full{signerKey},
	}

	escrowAccountKey := keypair.MustRandom()
	agent := &Agent{
		HorizonClient:       horizonClient,
		NetworkPassphrase:   networkDetails.NetworkPassphrase,
		EscrowAccountKey:    escrowAccountKey.FromAddress(),
		EscrowAccountSigner: signerKey,
		LogWriter:           os.Stderr,
	}

	fmt.Fprintln(os.Stdout, "escrow account:", escrowAccountKey.Address())
	tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
		Creator:        accountKey.FromAddress(),
		Escrow:         escrowAccountKey.FromAddress(),
		SequenceNumber: accountSeqNum + 1,
		Asset:          txnbuild.NativeAsset{},
	})
	if err != nil {
		return fmt.Errorf("creating escrow account tx: %w", err)
	}
	tx, err = tx.Sign(networkDetails.NetworkPassphrase, signerKey, escrowAccountKey)
	if err != nil {
		return fmt.Errorf("signing tx to create escrow account: %w", err)
	}
	err = submitter.SubmitFeeBumpTx(tx)
	if err != nil {
		return fmt.Errorf("submitting tx to create escrow account: %w", err)
	}

	br := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		line, err := br.ReadString('\n')
		if err == io.EOF {
			fmt.Fprintf(os.Stderr, "connection terminated\n")
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %#v\n", err)
			continue
		}
		params := strings.Fields(line)
		if len(params) == 0 {
			continue
		}
		switch params[0] {
		case "help":
			fmt.Fprintf(os.Stderr, "listen [addr]:<port> - listen for a peer to connect\n")
			fmt.Fprintf(os.Stderr, "connect <addr>:<port> - connect to a peer\n")
			fmt.Fprintf(os.Stderr, "open - open a channel with asset\n")
			fmt.Fprintf(os.Stderr, "deposit <amount> - deposit asset into escrow account\n")
			fmt.Fprintf(os.Stderr, "pay <amount> - pay amount of asset to peer\n")
			fmt.Fprintf(os.Stderr, "close - close the channel\n")
			fmt.Fprintf(os.Stderr, "exit - exit the application\n")
		case "listen":
			err := agent.Listen(params[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "connected to %v\n", agent.Conn().RemoteAddr())
		case "connect":
			err := agent.Connect(params[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "connected to %v\n", agent.Conn().RemoteAddr())
		case "intro":
			if agent.Conn() == nil {
				fmt.Fprintf(os.Stderr, "error: not connected to a peer\n")
				continue
			}
			enc := json.NewEncoder(io.MultiWriter(agent.Conn(), io.Discard))
			err = enc.Encode(message{
				Introduction: &introduction{
					EscrowAccount: escrowAccountKey.Address(),
					Signer:        accountKey.Address(),
				},
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}
		case "open":
			if agent.Conn() == nil {
				fmt.Fprintf(os.Stderr, "error: not connected to a peer\n")
				continue
			}
			if agent.channel == nil {
				fmt.Fprintf(os.Stderr, "error: not introduced to peer\n")
				continue
			}
			open, err := agent.channel.ProposeOpen(state.OpenParams{
				ObservationPeriodTime:      observationPeriodTime,
				ObservationPeriodLedgerGap: observationPeriodLedgerGap,
				Asset:                      "native",
				ExpiresAt:                  time.Now().Add(openExpiry),
			})
			if err != nil {
				return fmt.Errorf("proposing open: %w", err)
			}
			enc := json.NewEncoder(io.MultiWriter(agent.Conn(), io.Discard))
			err = enc.Encode(message{Open: &open})
			if err != nil {
				return fmt.Errorf("sending open: %w", err)
			}
		case "formate":
			err = ChannelSubmitter{Submitter: submitter, Channel: agent.channel}.SubmitFormationTx()
			if err != nil {
				return fmt.Errorf("submitting formation: %w", err)
			}
			fmt.Fprintf(os.Stdout, "formation submitted\n")
		case "deposit":
			depositAmountStr := params[1]
			depositAmountInt, err := amount.ParseInt64(depositAmountStr)
			if err != nil {
				return fmt.Errorf("parsing deposit amount: %w", err)
			}
			account, err := horizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountKey.Address()})
			if err != nil {
				return fmt.Errorf("getting state of local escrow account: %w", err)
			}
			tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
				SourceAccount:        &account,
				IncrementSequenceNum: true,
				BaseFee:              txnbuild.MinBaseFee,
				Timebounds:           txnbuild.NewTimeout(300),
				Operations: []txnbuild.Operation{
					&txnbuild.Payment{Destination: escrowAccountKey.Address(), Asset: txnbuild.NativeAsset{}, Amount: depositAmountStr},
				},
			})
			if err != nil {
				return fmt.Errorf("building deposit payment tx: %w", err)
			}
			tx, err = tx.Sign(networkDetails.NetworkPassphrase, signerKey)
			if err != nil {
				return fmt.Errorf("signing deposit payment tx: %w", err)
			}
			_, err = horizonClient.SubmitTransaction(tx)
			if err != nil {
				return fmt.Errorf("submitting deposit payment tx: %w", err)
			}
			newBalance := agent.channel.LocalEscrowAccountBalance() + depositAmountInt
			agent.channel.UpdateLocalEscrowAccountBalance(newBalance)
			fmt.Println("new balance of", newBalance)
		case "pay":
			if agent.Conn() == nil || agent.channel == nil {
				fmt.Fprintf(os.Stderr, "error: not connected to a peer\n")
				continue
			}
			amountValue, err := amount.ParseInt64(params[1])
			if err != nil {
				return fmt.Errorf("parsing the amount: %w", err)
			}
			ca, err := agent.channel.ProposePayment(amountValue)
			if err != nil {
				return fmt.Errorf("proposing the payment: %w", err)
			}
			enc := json.NewEncoder(io.MultiWriter(agent.Conn(), io.Discard))
			err = enc.Encode(message{Update: &ca})
			if err != nil {
				return fmt.Errorf("sending the payment: %w", err)
			}
		case "close":
			if agent.Conn() == nil || agent.channel == nil {
				fmt.Fprintf(os.Stderr, "error: not connected to a peer\n")
				continue
			}
			// Submit declaration tx
			err = ChannelSubmitter{Submitter: submitter, Channel: agent.channel}.SubmitLatestDeclarationTx()
			if err != nil {
				return fmt.Errorf("submitting tx to decl the channel: %w", err)
			}
			// Revising agreement to close early
			ca, err := agent.channel.ProposeClose()
			if err != nil {
				return fmt.Errorf("proposing the close: %w", err)
			}
			enc := json.NewEncoder(io.MultiWriter(agent.Conn(), os.Stdout))
			dec := json.NewDecoder(io.TeeReader(agent.Conn(), os.Stdout))
			err = enc.Encode(message{Close: &ca})
			if err != nil {
				return fmt.Errorf("sending the payment: %w", err)
			}
			err = agent.Conn().SetReadDeadline(time.Now().Add(observationPeriodTime))
			if err != nil {
				return fmt.Errorf("setting read deadline of conn: %w", err)
			}
			timerStart := time.Now()
			authorized := false
			m := message{}
			err = dec.Decode(&m)
			if errors.Is(err, os.ErrDeadlineExceeded) {
			} else {
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: decoding response: %v\n", err)
					break
				}
				_, authorized, err = agent.channel.ConfirmClose(*m.Close)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: confirming close: %v\n", err)
					break
				}
			}
			if authorized {
				fmt.Fprintln(os.Stderr, "close ready")
			} else {
				fmt.Fprintf(os.Stderr, "close not authorized, waiting observation period then closing...")
				time.Sleep(observationPeriodTime*2 - time.Since(timerStart))
			}
			err = ChannelSubmitter{Submitter: submitter, Channel: agent.channel}.SubmitLatestCloseTx()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: submitting close: %v\n", err)
				break
			}
		case "exit":
			return nil
		default:
			fmt.Fprintf(os.Stderr, "error: unrecognized command\n")
		}
	}
	return nil
}
