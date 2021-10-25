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
	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/state"
)

var ErrBufferFull = errors.New("buffer full")

type Config struct {
	Agent       *agent.Agent
	AgentEvents <-chan interface{}

	MaxBufferSize int

	LogWriter io.Writer

	Events chan<- interface{}
}

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

// BufferedAgent coordinates a payment channel over a TCP connection, and
// buffers payments by collapsing them down into single payments while it waits
// for a change to make a payment itself.
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

func (a *Agent) MaxBufferSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.maxbufferSize
}

func (a *Agent) SetMaxBufferSize(maxbufferSize int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.maxbufferSize = maxbufferSize
}

func (a *Agent) Open(asset state.Asset) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Open(asset)
}

// PaymentWithMemo buffers a payment which will be paid in the next settlement.
// The identifier for the settlement is returned.
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

// Wait waits for sending to complete and the buffer to be empty.
func (a *Agent) Wait() {
	<-a.idle
}

func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.bufferReady)
	return a.agent.DeclareClose()
}

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
			memo, err := parseBufferedPaymentMemo(e.CloseAgreement.Envelope.Details.Memo)
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
			memo, err := parseBufferedPaymentMemo(e.CloseAgreement.Envelope.Details.Memo)
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

	err := a.agent.PaymentWithMemo(bufferTotalAmount, memo.String())
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
