// Package agent contains a rudimentary and experimental implementation of an
// agent that coordinates a TCP network connection, initial handshake, and
// channel opens, payments, and closes.
//
// The agent is intended for use in examples only at this point and is not
// intended to be stable or reliable.
package agent

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/msg"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

// BalanceCollector gets the balance of an asset for an account.
type BalanceCollector interface {
	GetBalance(account *keypair.FromAddress, asset state.Asset) (int64, error)
}

// SequenceNumberCollector gets the sequence number for an account.
type SequenceNumberCollector interface {
	GetSequenceNumber(account *keypair.FromAddress) (int64, error)
}

// Submitter submits a transaction to the network.
type Submitter interface {
	SubmitTx(tx *txnbuild.Transaction) error
}

// Streamer streams transactions that affect a set of accounts.
type Streamer interface {
	StreamTx(cursor string, accounts ...*keypair.FromAddress) (transactions <-chan StreamedTransaction, cancel func())
}

// StreamedTransaction is a transaction that has been seen by the
// Streamer.
type StreamedTransaction struct {
	// Cursor is a cursor that can be used to resume streaming.
	Cursor string

	// TransactionOrderID is an identifier that orders transactions in the order
	// they were executed on the Stellar network.
	TransactionOrderID int64

	TransactionXDR string
	ResultXDR      string
	ResultMetaXDR  string
}

// Snapshotter is given a snapshot of the agent and its dependencies whenever
// its meaningful state changes. Snapshots can be restore using
// NewAgentFromSnapshot.
type Snapshotter interface {
	Snapshot(a *Agent, s Snapshot)
}

type Config struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	MaxOpenExpiry              time.Duration
	NetworkPassphrase          string

	SequenceNumberCollector SequenceNumberCollector
	BalanceCollector        BalanceCollector
	Submitter               Submitter
	Streamer                Streamer
	Snapshotter             Snapshotter

	EscrowAccountKey    *keypair.FromAddress
	EscrowAccountSigner *keypair.Full

	LogWriter io.Writer

	Events chan<- Event
}

func NewAgent(c Config) *Agent {
	agent := &Agent{
		observationPeriodTime:      c.ObservationPeriodTime,
		observationPeriodLedgerGap: c.ObservationPeriodLedgerGap,
		maxOpenExpiry:              c.MaxOpenExpiry,
		networkPassphrase:          c.NetworkPassphrase,

		sequenceNumberCollector: c.SequenceNumberCollector,
		balanceCollector:        c.BalanceCollector,
		submitter:               c.Submitter,
		streamer:                c.Streamer,
		snapshotter:             c.Snapshotter,

		escrowAccountKey:    c.EscrowAccountKey,
		escrowAccountSigner: c.EscrowAccountSigner,

		logWriter: c.LogWriter,

		events: c.Events,
	}
	return agent
}

// Snapshot is a snapshot of the agent and its dependencies excluding any fields
// provided in the Config when instantiating an agent. A Snapshot can be
// restored into an Agent using NewAgentWithSnapshot.
type Snapshot struct {
	OtherEscrowAccount       *keypair.FromAddress
	OtherEscrowAccountSigner *keypair.FromAddress
	StreamerCursor           string
	State                    *struct {
		Initiator bool
		Snapshot  state.Snapshot
	}
}

// NewAgentFromSnapshot creates an agent using a previously generated snapshot
// so that the new agent has the same state as the previous agent. To restore
// the channel to its identical state the same config should be provided that
// was in use when the snapshot was created.
func NewAgentFromSnapshot(c Config, s Snapshot) *Agent {
	agent := NewAgent(c)
	agent.otherEscrowAccount = s.OtherEscrowAccount
	agent.otherEscrowAccountSigner = s.OtherEscrowAccountSigner
	agent.streamerCursor = s.StreamerCursor
	if s.State != nil {
		agent.initChannel(s.State.Initiator, &s.State.Snapshot)
	}
	return agent
}

// Agent coordinates a payment channel over a TCP connection.
type Agent struct {
	observationPeriodTime      time.Duration
	observationPeriodLedgerGap int64
	maxOpenExpiry              time.Duration
	networkPassphrase          string

	sequenceNumberCollector SequenceNumberCollector
	balanceCollector        BalanceCollector
	submitter               Submitter
	streamer                Streamer
	snapshotter             Snapshotter

	escrowAccountKey    *keypair.FromAddress
	escrowAccountSigner *keypair.Full

	logWriter io.Writer

	events chan<- Event

	// mu is a lock for the mutable fields of this type. It should be locked
	// when reading or writing any of the mutable fields. The mutable fields are
	// listed below. If pushing to a chan, such as Events, it is unnecessary to
	// lock.
	mu sync.Mutex

	conn                     io.ReadWriter
	otherEscrowAccount       *keypair.FromAddress
	otherEscrowAccountSigner *keypair.FromAddress
	channel                  *state.Channel
	streamerTransactions     <-chan StreamedTransaction
	streamerCursor           string
	streamerCancel           func()
}

func (a *Agent) snapshot() {
	if a.snapshotter == nil {
		return
	}
	snapshot := Snapshot{
		OtherEscrowAccount:       a.otherEscrowAccount,
		OtherEscrowAccountSigner: a.otherEscrowAccountSigner,
		StreamerCursor:           a.streamerCursor,
	}
	if a.channel != nil {
		snapshot.State = &struct {
			Initiator bool
			Snapshot  state.Snapshot
		}{
			Initiator: a.channel.IsInitiator(),
			Snapshot:  a.channel.Snapshot(),
		}
	}
	a.snapshotter.Snapshot(a, snapshot)
}

// hello sends a hello message to the remote participant over the connection.
func (a *Agent) hello() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	enc := msg.NewEncoder(io.MultiWriter(a.conn, a.logWriter))
	err := enc.Encode(msg.Message{
		Type: msg.TypeHello,
		Hello: &msg.Hello{
			EscrowAccount: *a.escrowAccountKey,
			Signer:        *a.escrowAccountSigner.FromAddress(),
		},
	})
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	return nil
}

func (a *Agent) initChannel(initiator bool, snapshot *state.Snapshot) {
	config := state.Config{
		NetworkPassphrase:   a.networkPassphrase,
		MaxOpenExpiry:       a.maxOpenExpiry,
		Initiator:           initiator,
		LocalEscrowAccount:  a.escrowAccountKey,
		RemoteEscrowAccount: a.otherEscrowAccount,
		LocalSigner:         a.escrowAccountSigner,
		RemoteSigner:        a.otherEscrowAccountSigner,
	}
	if snapshot == nil {
		a.channel = state.NewChannel(config)
	} else {
		a.channel = state.NewChannelFromSnapshot(config, *snapshot)
	}
	a.streamerTransactions, a.streamerCancel = a.streamer.StreamTx(a.streamerCursor)
	go a.ingestLoop()
}

// Open kicks off the open process which will continue after the function
// returns.
func (a *Agent) Open() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel != nil {
		return fmt.Errorf("channel already exists")
	}

	seqNum, err := a.sequenceNumberCollector.GetSequenceNumber(a.escrowAccountKey)
	if err != nil {
		return fmt.Errorf("getting sequence number of escrow account: %w", err)
	}

	defer a.snapshot()

	a.initChannel(true, nil)

	open, err := a.channel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      a.observationPeriodTime,
		ObservationPeriodLedgerGap: a.observationPeriodLedgerGap,
		Asset:                      "native",
		ExpiresAt:                  time.Now().Add(a.maxOpenExpiry),
		StartingSequence:           seqNum + 1,
	})
	if err != nil {
		return fmt.Errorf("proposing open: %w", err)
	}
	enc := msg.NewEncoder(io.MultiWriter(a.conn, a.logWriter))
	err = enc.Encode(msg.Message{
		Type:        msg.TypeOpenRequest,
		OpenRequest: &open.Envelope,
	})
	if err != nil {
		return fmt.Errorf("sending open: %w", err)
	}

	return nil
}

// Payment makes a payment of the payment amount to the remote participant using
// the open channel. The process is asynchronous and the function returns
// immediately after the payment is signed and sent to the remote participant.
// The payment is not authorized until the remote participant signs the payment
// and returns the payment.
func (a *Agent) Payment(paymentAmount string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("no channel")
	}
	amountValue, err := amount.ParseInt64(paymentAmount)
	if err != nil {
		return fmt.Errorf("parsing amount %s: %w", paymentAmount, err)
	}

	defer a.snapshot()

	ca, err := a.channel.ProposePayment(amountValue)
	if errors.Is(err, state.ErrUnderfunded) {
		fmt.Fprintf(a.logWriter, "local is underfunded for this payment based on cached account balances, checking escrow account...\n")
		var balance int64
		balance, err = a.balanceCollector.GetBalance(a.channel.LocalEscrowAccount().Address, a.channel.OpenAgreement().Envelope.Details.Asset)
		if err != nil {
			return err
		}
		a.channel.UpdateLocalEscrowAccountBalance(balance)
		ca, err = a.channel.ProposePayment(amountValue)
	}
	if err != nil {
		return fmt.Errorf("proposing payment %d: %w", amountValue, err)
	}
	enc := msg.NewEncoder(io.MultiWriter(a.conn, a.logWriter))
	err = enc.Encode(msg.Message{
		Type:           msg.TypePaymentRequest,
		PaymentRequest: &ca.Envelope,
	})
	if err != nil {
		return fmt.Errorf("sending payment: %w", err)
	}

	return nil
}

// DeclareClose kicks off the close process by submitting a tx to the network to
// begin the close process, then asynchronously coordinating with the remote
// participant to coordinate the close. If the participant responds the agent
// will automatically submit the final close tx that can be submitted
// immediately. If no closed notification occurs before the observation period,
// manually submit the close by calling Close.
func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	// Submit declaration tx.
	declTx, _, err := a.channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building declaration tx: %w", err)
	}
	declHash, err := declTx.HashHex(a.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing decl tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "submitting declaration:", declHash)
	err = a.submitter.SubmitTx(declTx)
	if err != nil {
		return fmt.Errorf("submitting declaration tx: %w", err)
	}

	defer a.snapshot()

	// Attempt revising the close agreement to close early.
	fmt.Fprintln(a.logWriter, "proposing a revised close for immediate submission")
	ca, err := a.channel.ProposeClose()
	if err != nil {
		return fmt.Errorf("proposing the close: %w", err)
	}
	enc := msg.NewEncoder(io.MultiWriter(a.conn, a.logWriter))
	err = enc.Encode(msg.Message{
		Type:         msg.TypeCloseRequest,
		CloseRequest: &ca.Envelope,
	})
	if err != nil {
		return fmt.Errorf("error: sending the close proposal: %w", err)
	}

	return nil
}

// Close closes the channel. The close must have been declared first either by
// calling DeclareClose or by the other participant. If the close fails it may
// be because the channel is already closed, or the participant has submitted
// the same close which is already queued but not yet processed, or the
// observation period has not yet passed since the close was declared.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	_, closeTx, err := a.channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}
	closeHash, err := closeTx.HashHex(a.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "submitting close tx:", closeHash)
	err = a.submitter.SubmitTx(closeTx)
	if err != nil {
		fmt.Fprintln(a.logWriter, "error submitting close tx:", closeHash, ",", err)
		return fmt.Errorf("submitting close tx %s: %w", closeHash, err)
	}
	fmt.Fprintln(a.logWriter, "submitted close tx:", closeHash)
	return nil
}

func (a *Agent) receive() error {
	recv := msg.NewDecoder(io.TeeReader(a.conn, a.logWriter))
	send := msg.NewEncoder(io.MultiWriter(a.conn, a.logWriter))
	m := msg.Message{}
	err := recv.Decode(&m)
	if err == io.EOF {
		return err
	}
	if err != nil {
		return fmt.Errorf("reading and decoding: %v", err)
	}
	err = a.handle(m, send)
	if err != nil {
		return fmt.Errorf("handling message: %v", err)
	}
	return nil
}

func (a *Agent) receiveLoop() {
	for {
		err := a.receive()
		if err == io.EOF {
			fmt.Fprintln(a.logWriter, "error receiving: EOF, stopping receiving")
			break
		}
		if err != nil {
			fmt.Fprintf(a.logWriter, "error receiving: %v\n", err)
		}
	}
}

func (a *Agent) handle(m msg.Message, send *msg.Encoder) error {
	fmt.Fprintf(a.logWriter, "handling %v\n", m.Type)
	handler := handlerMap[m.Type]
	if handler == nil {
		err := fmt.Errorf("handling message %d: unrecognized message type", m.Type)
		if a.events != nil {
			a.events <- ErrorEvent{Err: err}
		}
		return err
	}
	err := handler(a, m, send)
	if err != nil {
		err = fmt.Errorf("handling message %d: %w", m.Type, err)
		if a.events != nil {
			a.events <- ErrorEvent{Err: err}
		}
		return err
	}
	return nil
}

var handlerMap = map[msg.Type]func(*Agent, msg.Message, *msg.Encoder) error{
	msg.TypeHello:           (*Agent).handleHello,
	msg.TypeOpenRequest:     (*Agent).handleOpenRequest,
	msg.TypeOpenResponse:    (*Agent).handleOpenResponse,
	msg.TypePaymentRequest:  (*Agent).handlePaymentRequest,
	msg.TypePaymentResponse: (*Agent).handlePaymentResponse,
	msg.TypeCloseRequest:    (*Agent).handleCloseRequest,
	msg.TypeCloseResponse:   (*Agent).handleCloseResponse,
}

func (a *Agent) handleHello(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	defer a.snapshot()

	h := m.Hello

	if a.otherEscrowAccount != nil && !a.otherEscrowAccount.Equal(&h.EscrowAccount) {
		return fmt.Errorf("hello received with unexpected escrow account: %s expected: %s", h.EscrowAccount.Address(), a.otherEscrowAccount.Address())
	}
	if a.otherEscrowAccountSigner != nil && !a.otherEscrowAccountSigner.Equal(&h.Signer) {
		return fmt.Errorf("hello received with unexpected signer: %s expected: %s", h.Signer.Address(), a.otherEscrowAccountSigner.Address())
	}

	a.otherEscrowAccount = &h.EscrowAccount
	a.otherEscrowAccountSigner = &h.Signer

	fmt.Fprintf(a.logWriter, "other's escrow account: %v\n", a.otherEscrowAccount.Address())
	fmt.Fprintf(a.logWriter, "other's signer: %v\n", a.otherEscrowAccountSigner.Address())

	if a.events != nil {
		a.events <- ConnectedEvent{}
	}

	return nil
}

func (a *Agent) handleOpenRequest(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	defer a.snapshot()

	if a.channel != nil {
		return fmt.Errorf("channel already exists")
	}

	a.initChannel(false, nil)

	openIn := *m.OpenRequest
	open, err := a.channel.ConfirmOpen(openIn)
	if err != nil {
		return fmt.Errorf("confirming open: %w", err)
	}
	fmt.Fprintf(a.logWriter, "open authorized\n")
	err = send.Encode(msg.Message{
		Type:         msg.TypeOpenResponse,
		OpenResponse: &open.Envelope,
	})
	if err != nil {
		return fmt.Errorf("encoding open to send back: %w", err)
	}
	return nil
}

func (a *Agent) handleOpenResponse(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	defer a.snapshot()

	openIn := *m.OpenResponse
	_, err := a.channel.ConfirmOpen(openIn)
	if err != nil {
		return fmt.Errorf("confirming open: %w", err)
	}
	fmt.Fprintf(a.logWriter, "open authorized\n")
	formationTx, err := a.channel.OpenTx()
	if err != nil {
		return fmt.Errorf("building formation tx: %w", err)
	}
	err = a.submitter.SubmitTx(formationTx)
	if err != nil {
		return fmt.Errorf("submitting formation tx: %w", err)
	}
	return nil
}

func (a *Agent) handlePaymentRequest(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	defer a.snapshot()

	paymentIn := *m.PaymentRequest
	payment, err := a.channel.ConfirmPayment(paymentIn)
	if errors.Is(err, state.ErrUnderfunded) {
		fmt.Fprintf(a.logWriter, "remote is underfunded for this payment based on cached account balances, checking their escrow account...\n")
		var balance int64
		balance, err = a.balanceCollector.GetBalance(a.channel.RemoteEscrowAccount().Address, a.channel.OpenAgreement().Envelope.Details.Asset)
		if err != nil {
			return err
		}
		a.channel.UpdateRemoteEscrowAccountBalance(balance)
		payment, err = a.channel.ConfirmPayment(paymentIn)
	}
	if err != nil {
		return fmt.Errorf("confirming payment: %w", err)
	}
	fmt.Fprintf(a.logWriter, "payment authorized\n")
	err = send.Encode(msg.Message{Type: msg.TypePaymentResponse, PaymentResponse: &payment.Envelope})
	if a.events != nil {
		a.events <- PaymentReceivedEvent{CloseAgreement: payment}
	}
	if err != nil {
		return fmt.Errorf("encoding payment to send back: %w", err)
	}
	return nil
}

func (a *Agent) handlePaymentResponse(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	defer a.snapshot()

	paymentIn := *m.PaymentResponse
	payment, err := a.channel.ConfirmPayment(paymentIn)
	if err != nil {
		return fmt.Errorf("confirming payment: %w", err)
	}
	fmt.Fprintf(a.logWriter, "payment authorized\n")
	if a.events != nil {
		a.events <- PaymentSentEvent{CloseAgreement: payment}
	}
	return nil
}

func (a *Agent) handleCloseRequest(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	defer a.snapshot()

	// Agree to the close and send it back to requesting participant.
	closeIn := *m.CloseRequest
	close, err := a.channel.ConfirmClose(closeIn)
	if err != nil {
		return fmt.Errorf("confirming close: %v\n", err)
	}
	err = send.Encode(msg.Message{
		Type:          msg.TypeCloseResponse,
		CloseResponse: &close.Envelope,
	})
	if err != nil {
		return fmt.Errorf("encoding close to send back: %v\n", err)
	}
	fmt.Fprintln(a.logWriter, "close ready")

	// Submit the close immediately since it is valid immediately.
	_, closeTx, err := a.channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}
	hash, err := closeTx.HashHex(a.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "submitting close", hash)
	err = a.submitter.SubmitTx(closeTx)
	if err != nil {
		return fmt.Errorf("submitting close tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "close successful")
	return nil
}

func (a *Agent) handleCloseResponse(m msg.Message, send *msg.Encoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.channel == nil {
		return fmt.Errorf("no channel")
	}

	defer a.snapshot()

	// Store updated agreement from other participant.
	closeIn := *m.CloseResponse
	_, err := a.channel.ConfirmClose(closeIn)
	if err != nil {
		return fmt.Errorf("confirming close: %v\n", err)
	}
	fmt.Fprintln(a.logWriter, "close ready")

	// Submit the close immediately since it is valid immediately.
	_, closeTx, err := a.channel.CloseTxs()
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}
	hash, err := closeTx.HashHex(a.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing close tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "submitting close", hash)
	err = a.submitter.SubmitTx(closeTx)
	if err != nil {
		return fmt.Errorf("submitting close tx: %w", err)
	}
	fmt.Fprintln(a.logWriter, "close successful")
	return nil
}
