// Package bufferedagent contains a rudimentary and experimental implementation
// of an agent that coordinates a TCP network connection, initial handshake, and
// channel opens, payments, and closes, and buffers outgoing payments,
// collapsing them down to a single payment.
//
// The agent is intended for use in examples only at this point and is not
// intended to be stable or reliable.
package bufferedagent

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/google/uuid"
	"github.com/stellar/starlight/sdk/agent"
	"github.com/stellar/starlight/sdk/state"
)

// ErrBufferFull indicates that the payment buffer has reached it's maximum size
// as configured when the buffered agent was created.
var ErrBufferFull = errors.New("buffer full")

// Config contains the information that can be supplied to configure the Agent
// at construction.
type Config struct {
	Agent       *agent.Agent
	AgentEvents <-chan interface{}

	MaxBufferSize int

	LogWriter io.Writer

	Events chan<- interface{}
}

// NewAgent constructs a new buffered agent with the given config.
func NewAgent(c Config) *Agent {
	agent := &Agent{
		agent:       c.Agent,
		agentEvents: c.AgentEvents,

		maxbufferSize: c.MaxBufferSize,

		logWriter: c.LogWriter,

		bufferReady:  make(chan struct{}, 1),
		sendingReady: make(chan struct{}, 1),
		idle:         make(chan struct{}),

		events: c.Events,
	}
	agent.resetbuffer()
	agent.sendingReady <- struct{}{}
	go agent.flushLoop()
	return agent
}

// Agent coordinates a payment channel over a TCP connection, and
// buffers payments by collapsing them down into single payments while it waits
// for a chance to make the next payment.
//
// All functions of the Agent are safe to call from multiple goroutines as they
// use an internal mutex.
type Agent struct {
	maxbufferSize int

	logWriter io.Writer

	agentEvents <-chan interface{}
	events      chan<- interface{}

	// mu is a lock for the mutable fields of this type. It should be locked
	// when reading or writing any of the mutable fields. The mutable fields are
	// listed below. If pushing to a chan, such as Events, it is unnecessary to
	// lock.
	mu sync.Mutex

	agent *agent.Agent

	bufferID          string
	buffer            []BufferedPayment
	bufferTotalAmount int64
	bufferReady       chan struct{}
	sendingReady      chan struct{}
	idle              chan struct{}
}

// MaxBufferSize returns the maximum buffer size that was configured at
// construction or changed with SetMaxBufferSize. The maximum buffer size is the
// maximum number of payments that can be buffered while waiting for the
// opportunity to include the buffered payments in an agreement.
func (a *Agent) MaxBufferSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.maxbufferSize
}

// SetMaxBufferSize sets and changes the maximum buffer size.
func (a *Agent) SetMaxBufferSize(maxbufferSize int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.maxbufferSize = maxbufferSize
}

// Open opens the channel for the given asset. The open is coordinated with the
// other participant. An immediate error may be indicated if the attempt to open
// was immediately unsuccessful. However, more likely any error will be returned
// on the events channel as the process involves the other participant.
func (a *Agent) Open(asset state.Asset) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Open(asset)
}

// PaymentWithMemo buffers a payment which will be paid in the next agreement.
// The identifier for the buffer is returned. An error may be returned
// immediately if the buffer is full. Any errors relating to the payment, and
// confirmation of the payment, will be returned asynchronously on the events
// channel.
func (a *Agent) PaymentWithMemo(paymentAmount int64, memo string) (bufferID string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.maxbufferSize != 0 && len(a.buffer) == a.maxbufferSize {
		return "", ErrBufferFull
	}
	if paymentAmount > math.MaxInt64-a.bufferTotalAmount {
		return "", ErrBufferFull
	}
	a.buffer = append(a.buffer, BufferedPayment{Amount: paymentAmount, Memo: memo})
	a.bufferTotalAmount += paymentAmount
	bufferID = a.bufferID
	select {
	case a.bufferReady <- struct{}{}:
	default:
	}
	return
}

// Payment is equivalent to calling PaymentWithMemo with an empty memo.
func (a *Agent) Payment(paymentAmount int64) (bufferID string, err error) {
	return a.PaymentWithMemo(paymentAmount, "")
}

// Wait waits for sending of all buffered payments to complete and the buffer to
// be empty. It can be called multiple times, and it can be called in between
// sends of new payments.
func (a *Agent) Wait() {
	<-a.idle
}

// DeclareClose starts the close process of the channel by submitting the latest
// declaration to the network, then coordinating an immediate close with the
// other participant. If an immediate close can be coordinated it will
// automatically occur, otherwise a participant must call Close after the
// observation period has passed to close the channel.
//
// It is not possible to make new payments once called.
func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.bufferReady)
	return a.agent.DeclareClose()
}

// Close submits the close transaction to the network. DeclareClose must have
// been called by one of the participants before hand.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.bufferReady)
	return a.agent.Close()
}

func (a *Agent) eventLoop() {
	defer close(a.events)
	defer close(a.sendingReady)
	defer fmt.Fprintf(a.logWriter, "event loop stopped\n")
	fmt.Fprintf(a.logWriter, "event loop started\n")
	for {
		ae, open := <-a.agentEvents
		if !open {
			break
		}

		// Pass all agent events up to the agent.
		a.events <- ae

		// Unpack payment received and sent events and create events for each
		// sub-payment that was bufferd within them.
		switch e := ae.(type) {
		case agent.PaymentReceivedEvent:
			memo := bufferedPaymentsMemo{}
			err := memo.UnmarshalBinary(e.CloseAgreement.Envelope.Details.Memo)
			if err != nil {
				a.events <- agent.ErrorEvent{Err: err}
				continue
			}
			a.events <- BufferedPaymentsReceivedEvent{
				BufferID:       memo.ID,
				BufferByteSize: len(e.CloseAgreement.Envelope.Details.Memo),
				Payments:       memo.Payments,
			}
		case agent.PaymentSentEvent:
			a.sendingReady <- struct{}{}
			memo := bufferedPaymentsMemo{}
			err := memo.UnmarshalBinary(e.CloseAgreement.Envelope.Details.Memo)
			if err != nil {
				a.events <- agent.ErrorEvent{Err: err}
				continue
			}
			a.events <- BufferedPaymentsSentEvent{
				BufferID:       memo.ID,
				BufferByteSize: len(e.CloseAgreement.Envelope.Details.Memo),
				Payments:       memo.Payments,
			}
		}
	}
}

func (a *Agent) flushLoop() {
	defer fmt.Fprintf(a.logWriter, "flush loop stopped\n")
	fmt.Fprintf(a.logWriter, "flush loop started\n")
	for {
		_, open := <-a.sendingReady
		if !open {
			return
		}
		select {
		case _, open = <-a.bufferReady:
			if !open {
				return
			}
			a.flush()
		default:
			select {
			case _, open = <-a.bufferReady:
				if !open {
					return
				}
				a.flush()
			case a.idle <- struct{}{}:
				a.sendingReady <- struct{}{}
			}
		}
	}
}

func (a *Agent) flush() {
	var bufferID string
	var buffer []BufferedPayment
	var bufferTotalAmount int64

	func() {
		a.mu.Lock()
		defer a.mu.Unlock()

		bufferID = a.bufferID
		buffer = a.buffer
		bufferTotalAmount = a.bufferTotalAmount
		a.resetbuffer()
	}()

	if len(buffer) == 0 {
		a.sendingReady <- struct{}{}
		return
	}

	memo := bufferedPaymentsMemo{
		ID:       bufferID,
		Payments: buffer,
	}
	memoBytes, err := memo.MarshalBinary()
	if err != nil {
		a.events <- agent.ErrorEvent{Err: err}
		a.sendingReady <- struct{}{}
		return
	}

	err = a.agent.PaymentWithMemo(bufferTotalAmount, memoBytes)
	if err != nil {
		a.events <- agent.ErrorEvent{Err: err}
		a.sendingReady <- struct{}{}
		return
	}
}

func (a *Agent) resetbuffer() {
	a.bufferID = uuid.NewString()
	a.buffer = nil
	a.bufferTotalAmount = 0
}
