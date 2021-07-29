package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/msg"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
)

const (
	observationPeriodTime      = 10 * time.Second
	observationPeriodLedgerGap = 1
	openExpiry                 = 5 * time.Minute
)

type Agent struct {
	NetworkPassphrase string
	HorizonClient     horizonclient.ClientInterface
	Submitter         *Submitter

	EscrowAccountKey    *keypair.FromAddress
	EscrowAccountSigner *keypair.Full

	LogWriter io.Writer

	Channel *state.Channel

	Conn net.Conn

	closeSignal chan struct{}
}

func (a *Agent) Listen(addr string) error {
	if a.Conn != nil {
		return fmt.Errorf("already connected to a peer")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	a.Conn, err = ln.Accept()
	if err != nil {
		return fmt.Errorf("accepting incoming connection: %w", err)
	}
	err = a.sendHello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.loop()
	return nil
}

func (a *Agent) Connect(addr string) error {
	if a.Conn != nil {
		return fmt.Errorf("already connected to a peer")
	}
	var err error
	a.Conn, err = net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	err = a.sendHello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.loop()
	return nil
}

func (a *Agent) sendHello() error {
	enc := json.NewEncoder(io.MultiWriter(a.Conn, a.LogWriter))
	err := enc.Encode(msg.Message{
		Type: msg.TypeHello,
		Hello: &msg.Hello{
			EscrowAccount: *a.EscrowAccountKey,
			Signer:        *a.EscrowAccountSigner.FromAddress(),
		},
	})
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	return nil
}

func (a *Agent) StartOpen() error {
	if a.Conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.Channel == nil {
		return fmt.Errorf("not introduced")
	}
	open, err := a.Channel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      "native",
		ExpiresAt:                  time.Now().Add(openExpiry),
	})
	if err != nil {
		return fmt.Errorf("proposing open: %w", err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.Conn, a.LogWriter))
	err = enc.Encode(msg.Message{
		Type:        msg.TypeOpenRequest,
		OpenRequest: &open,
	})
	if err != nil {
		return fmt.Errorf("sending open: %w", err)
	}
	return nil
}

func (a *Agent) StartPayment(paymentAmount string) error {
	if a.Conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.Channel == nil {
		return fmt.Errorf("not introduced")
	}
	amountValue, err := amount.ParseInt64(paymentAmount)
	if err != nil {
		return fmt.Errorf("parsing amount %s: %w", paymentAmount, err)
	}
	ca, err := a.Channel.ProposePayment(amountValue)
	if err != nil {
		return fmt.Errorf("proposing payment %d: %w", amountValue, err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.Conn, a.LogWriter))
	err = enc.Encode(msg.Message{
		Type:           msg.TypePaymentRequest,
		PaymentRequest: &ca,
	})
	if err != nil {
		return fmt.Errorf("sending payment: %w", err)
	}
	return nil
}

func (a *Agent) StartClose() error {
	if a.Conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.Channel == nil {
		return fmt.Errorf("not introduced")
	}
	a.closeSignal = make(chan struct{})
	// Submit declaration tx
	declTx, closeTx, err := a.Channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building declaration tx: %w", err)
	}
	declHash, err := declTx.HashHex(a.NetworkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.LogWriter, "submitting declaration", declHash)
	err = a.Submitter.SubmitFeeBumpTx(declTx)
	if err != nil {
		return fmt.Errorf("submitting declaration tx: %w", err)
	}
	// Revising agreement to close early
	fmt.Fprintln(a.LogWriter, "proposing a revised close for immediate submission")
	ca, err := a.Channel.ProposeClose()
	if err != nil {
		return fmt.Errorf("proposing the close: %w", err)
	}
	enc := json.NewEncoder(io.MultiWriter(a.Conn, a.LogWriter))
	err = enc.Encode(msg.Message{
		Type:         msg.TypeCloseRequest,
		CloseRequest: &ca,
	})
	if err != nil {
		return fmt.Errorf("error: sending the close proposal: %w\n", err)
	}
	closeHash, err := closeTx.HashHex(a.NetworkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.LogWriter, "waiting observation period to submit delayed close tx", closeHash)
	select {
	case <-a.closeSignal:
		fmt.Fprintln(a.LogWriter, "aborting sending delayed close tx", closeHash)
		return nil
	case <-time.After(observationPeriodTime):
	}
	fmt.Fprintln(a.LogWriter, "submitting delayed close tx", closeHash)
	err = a.Submitter.SubmitFeeBumpTx(closeTx)
	if err != nil {
		return fmt.Errorf("submitting declaration tx: %w", err)
	}
	return nil
}

func (a *Agent) loop() {
	var err error
	recv := json.NewDecoder(io.TeeReader(a.Conn, a.LogWriter))
	send := json.NewEncoder(io.MultiWriter(a.Conn, a.LogWriter))
	for {
		m := msg.Message{}
		err = recv.Decode(&m)
		if err != nil {
			fmt.Fprintf(a.LogWriter, "error decoding message: %v\n", err)
			break
		}
		err = a.handle(m, send)
		if err != nil {
			fmt.Fprintf(a.LogWriter, "error handling message: %v\n", err)
		}
	}
}

func (a *Agent) handle(m msg.Message, send *json.Encoder) error {
	fmt.Fprintf(a.LogWriter, "handling %v\n", m.Type)
	handler := handlerMap[m.Type]
	if handler == nil {
		return fmt.Errorf("unrecognized message type %v", m.Type)
	}
	err := handler(a, m, send)
	if err != nil {
		return fmt.Errorf("handling message type %v: %w", m.Type, err)
	}
	return nil
}

var handlerMap = map[msg.Type]func(*Agent, msg.Message, *json.Encoder) error{
	msg.TypeHello:           (*Agent).handleHello,
	msg.TypeOpenRequest:     (*Agent).handleOpenRequest,
	msg.TypeOpenResponse:    (*Agent).handleOpenResponse,
	msg.TypePaymentRequest:  (*Agent).handlePaymentRequest,
	msg.TypePaymentResponse: (*Agent).handlePaymentResponse,
	msg.TypeCloseRequest:    (*Agent).handleCloseRequest,
	msg.TypeCloseResponse:   (*Agent).handleCloseResponse,
}

func (a *Agent) handleHello(m msg.Message, send *json.Encoder) error {
	h := *m.Hello

	fmt.Fprintf(a.LogWriter, "other's signer: %v\n", h.Signer.Address())
	fmt.Fprintf(a.LogWriter, "other's escrow account: %v\n", h.EscrowAccount.Address())
	escrowAccountSeqNum, err := getSeqNum(a.HorizonClient, a.EscrowAccountKey.Address())
	if err != nil {
		return fmt.Errorf("getting sequence number of escrow account: %w", err)
	}
	otherEscrowAccountSeqNum, err := getSeqNum(a.HorizonClient, h.EscrowAccount.Address())
	if err != nil {
		return fmt.Errorf("getting sequence number of other's escrow account: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "escrow account seq: %v\n", escrowAccountSeqNum)
	fmt.Fprintf(a.LogWriter, "other's escrow account seq: %v\n", otherEscrowAccountSeqNum)
	a.Channel = state.NewChannel(state.Config{
		NetworkPassphrase: a.NetworkPassphrase,
		MaxOpenExpiry:     openExpiry,
		Initiator:         a.EscrowAccountKey.Address() > h.EscrowAccount.Address(),
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        a.EscrowAccountKey,
			SequenceNumber: escrowAccountSeqNum,
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        &h.EscrowAccount,
			SequenceNumber: otherEscrowAccountSeqNum,
		},
		LocalSigner:  a.EscrowAccountSigner,
		RemoteSigner: &h.Signer,
	})
	return nil
}

func (a *Agent) handleOpenRequest(m msg.Message, send *json.Encoder) error {
	openIn := *m.OpenRequest
	open, err := a.Channel.ConfirmOpen(openIn)
	if err != nil {
		return fmt.Errorf("confirming open: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "open authorized\n")
	err = send.Encode(msg.Message{
		Type:         msg.TypeOpenResponse,
		OpenResponse: &open,
	})
	if err != nil {
		return fmt.Errorf("encoding open to send back: %w", err)
	}
	return nil
}

func (a *Agent) handleOpenResponse(m msg.Message, send *json.Encoder) error {
	openIn := *m.OpenResponse
	_, err := a.Channel.ConfirmOpen(openIn)
	if err != nil {
		return fmt.Errorf("confirming open: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "open authorized\n")
	formationTx, err := a.Channel.OpenTx()
	if err != nil {
		return fmt.Errorf("building formation tx: %w", err)
	}
	err = a.Submitter.SubmitFeeBumpTx(formationTx)
	if err != nil {
		return fmt.Errorf("submitting formation tx: %w", err)
	}
	return nil
}

func (a *Agent) handlePaymentRequest(m msg.Message, send *json.Encoder) error {
	paymentIn := *m.PaymentRequest
	payment, err := a.Channel.ConfirmPayment(paymentIn)
	if errors.Is(err, state.ErrUnderfunded) {
		fmt.Fprintf(a.LogWriter, "remote is underfunded for this payment based on cached account balances, checking their escrow account...\n")
		var account horizon.Account
		account, err = a.HorizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: a.Channel.RemoteEscrowAccount().Address.Address()})
		if err != nil {
			return fmt.Errorf("getting state of remote escrow account: %w", err)
		}
		fmt.Fprintf(a.LogWriter, "updating remote escrow balance to: %s\n", account.Balances[0].Balance)
		var balance int64
		balance, err = amount.ParseInt64(account.Balances[0].Balance)
		if err != nil {
			return fmt.Errorf("parsing balance of remote escrow account: %w", err)
		}
		a.Channel.UpdateRemoteEscrowAccountBalance(balance)
		payment, err = a.Channel.ConfirmPayment(paymentIn)
	}
	if err != nil {
		return fmt.Errorf("confirming payment: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "payment authorized\n")
	err = send.Encode(msg.Message{Type: msg.TypePaymentResponse, PaymentResponse: &payment})
	if err != nil {
		return fmt.Errorf("encoding payment to send back: %w", err)
	}
	return nil
}

func (a *Agent) handlePaymentResponse(m msg.Message, send *json.Encoder) error {
	paymentIn := *m.PaymentResponse
	_, err := a.Channel.ConfirmPayment(paymentIn)
	if err != nil {
		return fmt.Errorf("confirming payment: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "payment authorized\n")
	return nil
}

func (a *Agent) handleCloseRequest(m msg.Message, send *json.Encoder) error {
	closeIn := *m.CloseRequest
	close, err := a.Channel.ConfirmClose(closeIn)
	if err != nil {
		return fmt.Errorf("confirming close: %v\n", err)
	}
	err = send.Encode(msg.Message{
		Type:          msg.TypeCloseResponse,
		CloseResponse: &close,
	})
	if err != nil {
		return fmt.Errorf("encoding close to send back: %v\n", err)
	}
	fmt.Fprintln(a.LogWriter, "close ready")
	return nil
}

func (a *Agent) handleCloseResponse(m msg.Message, send *json.Encoder) error {
	closeIn := *m.CloseResponse
	_, err := a.Channel.ConfirmClose(closeIn)
	if err != nil {
		return fmt.Errorf("confirming close: %v\n", err)
	}
	fmt.Fprintln(a.LogWriter, "close ready")
	_, closeTx, err := a.Channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}
	hash, err := closeTx.HashHex(a.NetworkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.LogWriter, "submitting close", hash)
	err = a.Submitter.SubmitFeeBumpTx(closeTx)
	if err != nil {
		return fmt.Errorf("submitting close tx: %w", err)
	}
	fmt.Fprintln(a.LogWriter, "close successful")
	if a.closeSignal != nil {
		close(a.closeSignal)
	}
	return nil
}
