package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
)

const (
	observationPeriodTime      = 10 * time.Second
	observationPeriodLedgerGap = 1
	openExpiry                 = 30 * time.Second
)

type Agent struct {
	NetworkPassphrase string
	HorizonClient     horizonclient.ClientInterface
	Submitter         *Submitter

	EscrowAccountKey    *keypair.FromAddress
	EscrowAccountSigner *keypair.Full

	LogWriter io.Writer

	channel *state.Channel

	conn net.Conn
	// err is the last error that has occurred on the agent.
	err error
}

func (a *Agent) Conn() net.Conn {
	return a.conn
}

func (a *Agent) Error() error {
	return a.err
}

func (a *Agent) Listen(addr string) error {
	if a.conn != nil {
		return fmt.Errorf("already connected to a peer")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	a.conn, err = ln.Accept()
	if err != nil {
		return fmt.Errorf("accepting incoming connection: %w", err)
	}
	go a.loop()
	return nil
}

func (a *Agent) Connect(addr string) error {
	if a.conn != nil {
		return fmt.Errorf("already connected to a peer")
	}
	var err error
	a.conn, err = net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	go a.loop()
	return nil
}

func (a *Agent) StartIntro() error {
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	enc := json.NewEncoder(io.MultiWriter(a.conn, a.LogWriter))
	err := enc.Encode(message{
		Introduction: &introduction{
			EscrowAccount: a.EscrowAccountKey.Address(),
			Signer:        a.EscrowAccountSigner.Address(),
		},
	})
	if err != nil {
		return fmt.Errorf("sending introduction: %w", err)
	}
	return nil
}

func (a *Agent) StartOpen() error {
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("not introduced")
	}
	open, err := a.channel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      "native",
		ExpiresAt:                  time.Now().Add(openExpiry),
	})
	if err != nil {
		return fmt.Errorf("proposing open: %w", err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.conn, a.LogWriter))
	err = enc.Encode(message{Open: &open})
	if err != nil {
		return fmt.Errorf("sending open: %w", err)
	}
	return nil
}

func (a *Agent) StartFormate() error {
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("not introduced")
	}
	err := ChannelSubmitter{Submitter: a.Submitter, Channel: a.channel}.SubmitFormationTx()
	if err != nil {
		return fmt.Errorf("submitting formation: %w", err)
	}
	return nil
}

func (a *Agent) StartPayment(paymentAmount string) error {
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("not introduced")
	}
	amountValue, err := amount.ParseInt64(paymentAmount)
	if err != nil {
		return fmt.Errorf("parsing amount %s: %w", paymentAmount, err)
	}
	ca, err := a.channel.ProposePayment(amountValue)
	if err != nil {
		return fmt.Errorf("proposing payment %d: %w", amountValue, err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.conn, a.LogWriter))
	err = enc.Encode(message{Update: &ca})
	if err != nil {
		return fmt.Errorf("sending payment: %w", err)
	}
	return nil
}

func (a *Agent) StartClose() error {
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("not introduced")
	}
	// Submit declaration tx
	err := ChannelSubmitter{Submitter: a.Submitter, Channel: a.channel}.SubmitLatestDeclarationTx()
	if err != nil {
		return fmt.Errorf("submitting tx to decl the channel: %w", err)
	}
	// Revising agreement to close early
	ca, err := a.channel.ProposeClose()
	if err != nil {
		return fmt.Errorf("proposing the close: %w", err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.Conn(), a.LogWriter))
	dec := json.NewDecoder(io.TeeReader(a.Conn(), a.LogWriter))
	err = enc.Encode(message{Close: &ca})
	if err != nil {
		return fmt.Errorf("sending the payment: %w", err)
	}
	err = a.conn.SetReadDeadline(time.Now().Add(observationPeriodTime))
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
			return fmt.Errorf("decoding response: %w", err)
		}
		_, authorized, err = a.channel.ConfirmClose(*m.Close)
		if err != nil {
			return fmt.Errorf("confirming close response: %w", err)
		}
	}
	if authorized {
		fmt.Fprintln(a.LogWriter, "close ready")
	} else {
		fmt.Fprintf(a.LogWriter, "close not authorized, waiting observation period then closing...")
		time.Sleep(observationPeriodTime*2 - time.Since(timerStart))
	}
	err = ChannelSubmitter{Submitter: a.Submitter, Channel: a.channel}.SubmitLatestCloseTx()
	if err != nil {
		return fmt.Errorf("submitting close tx: %w", err)
	}
	return nil
}

func (a *Agent) loop() {
	var err error
	recv := json.NewDecoder(io.TeeReader(a.Conn(), a.LogWriter))
	send := json.NewEncoder(io.MultiWriter(a.Conn(), a.LogWriter))
	for {
		m := message{}
		err = recv.Decode(&m)
		if err != nil {
			a.err = err
			break
		}
		err = a.handle(m, send)
		if err != nil {
			fmt.Fprintf(a.LogWriter, "error: %v\n", err)
			a.err = err
		}
	}
}

func (a *Agent) handle(m message, send *json.Encoder) error {
	if m.Introduction != nil {
		return a.handleIntroduction(*m.Introduction, send)
	}
	if m.Open != nil {
		return a.handleOpen(*m.Open, send)
	}
	if m.Update != nil {
		return a.handleUpdate(*m.Update, send)
	}
	if m.Close != nil {
		return a.handleClose(*m.Close, send)
	}
	return nil
}

func (a *Agent) handleIntroduction(intro introduction, send *json.Encoder) error {
	if a.channel != nil {
		return nil
	}

	otherEscrowAccountKey, err := keypair.ParseAddress(intro.EscrowAccount)
	if err != nil {
		return fmt.Errorf("parsing other's escrow account: %w", err)
	}
	otherSignerKey, err := keypair.ParseAddress(intro.Signer)
	if err != nil {
		return fmt.Errorf("parsing other's signer: %v\n", err)
	}
	fmt.Fprintf(a.LogWriter, "other's signer: %v\n", otherSignerKey.Address())
	fmt.Fprintf(a.LogWriter, "other's escrow account: %v\n", otherEscrowAccountKey.Address())
	err = send.Encode(message{
		Introduction: &introduction{
			EscrowAccount: a.EscrowAccountKey.Address(),
			Signer:        a.EscrowAccountSigner.Address(),
		},
	})
	if err != nil {
		return fmt.Errorf("sending back introduction: %w", err)
	}
	escrowAccountSeqNum, err := getSeqNum(a.HorizonClient, a.EscrowAccountKey.Address())
	if err != nil {
		return fmt.Errorf("getting sequence number of escrow account: %w", err)
	}
	otherEscrowAccountSeqNum, err := getSeqNum(a.HorizonClient, otherEscrowAccountKey.Address())
	if err != nil {
		return fmt.Errorf("getting sequence number of other's escrow account: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "escrow account seq: %v\n", escrowAccountSeqNum)
	fmt.Fprintf(a.LogWriter, "other's escrow account seq: %v\n", otherEscrowAccountSeqNum)
	a.channel = state.NewChannel(state.Config{
		NetworkPassphrase: a.NetworkPassphrase,
		MaxOpenExpiry:     openExpiry,
		Initiator:         a.EscrowAccountKey.Address() > otherEscrowAccountKey.Address(),
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        a.EscrowAccountKey,
			SequenceNumber: escrowAccountSeqNum,
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        otherEscrowAccountKey,
			SequenceNumber: otherEscrowAccountSeqNum,
		},
		LocalSigner:  a.EscrowAccountSigner,
		RemoteSigner: otherSignerKey,
	})
	return nil
}

func (a *Agent) handleOpen(openIn state.OpenAgreement, send *json.Encoder) error {
	open, authorized, err := a.channel.ConfirmOpen(openIn)
	if err != nil {
		return fmt.Errorf("confirming open: %w", err)
	}
	if authorized {
		fmt.Fprintf(a.LogWriter, "open authorized\n")
	}
	if !open.Equal(openIn) {
		err = send.Encode(message{Open: &open})
		if err != nil {
			return fmt.Errorf("encoding open to send back: %w", err)
		}
	}
	return nil
}

func (a *Agent) handleUpdate(updateIn state.CloseAgreement, send *json.Encoder) error {
	update, authorized, err := a.channel.ConfirmPayment(updateIn)
	if errors.Is(err, state.ErrUnderfunded) {
		fmt.Fprintf(a.LogWriter, "remote is underfunded for this payment based on cached account balances, checking their escrow account...\n")
		var account horizon.Account
		account, err = a.HorizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: a.channel.RemoteEscrowAccount().Address.Address()})
		if err != nil {
			return fmt.Errorf("getting state of remote escrow account: %w", err)
		}
		balance, err := amount.ParseInt64(account.Balances[0].Balance)
		if err != nil {
			return fmt.Errorf("parsing balance of remote escrow account: %w", err)
		}
		fmt.Fprintf(a.LogWriter, "updating remote escrow balance to: %d\n", balance)
		a.channel.UpdateRemoteEscrowAccountBalance(balance)
		update, authorized, err = a.channel.ConfirmPayment(updateIn)
	}
	if err != nil {
		return fmt.Errorf("confirming payment: %w", err)
	}
	if !update.Equal(updateIn) {
		err = send.Encode(message{Update: &update})
		if err != nil {
			fmt.Errorf("encoding payment to send back: %w", err)
		}
	}
	if authorized {
		fmt.Fprintf(a.LogWriter, "payment successfully received\n")
	}
	return nil
}

func (a *Agent) handleClose(closeIn state.CloseAgreement, send *json.Encoder) error {
	close, authorized, err := a.channel.ConfirmClose(closeIn)
	if err != nil {
		return fmt.Errorf("confirming close: %v\n", err)
	}
	err = send.Encode(message{Close: &close})
	if err != nil {
		return fmt.Errorf("encoding close to send back: %v\n", err)
	}
	if authorized {
		fmt.Fprintln(a.LogWriter, "close ready")
	}
	return nil
}
