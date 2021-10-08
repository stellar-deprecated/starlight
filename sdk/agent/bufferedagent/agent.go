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
)

var ErrQueueFull = errors.New("queue full")

type Config struct {
	Agent       *agent.Agent
	AgentEvents <-chan agent.Event

	MaxQueueSize int

	LogWriter io.Writer

	Events chan<- interface{}
}

func NewAgent(c Config) *Agent {
	agent := &Agent{
		agent:       c.Agent,
		agentEvents: c.AgentEvents,

		maxQueueSize: c.MaxQueueSize,

		logWriter: c.LogWriter,

		queueReady:   make(chan struct{}, 1),
		sendingReady: make(chan struct{}, 1),
		idle:         make(chan struct{}),

		events: c.Events,
	}
	agent.resetQueue()
	agent.sendingReady <- struct{}{}
	go agent.flushLoop()
	return agent
}

// BufferedAgent coordinates a payment channel over a TCP connection, and
// buffers payments by collapsing them down into single payments while it waits
// for a change to make a payment itself.
type Agent struct {
	maxQueueSize int

	logWriter io.Writer

	agentEvents <-chan agent.Event
	events      chan<- interface{}

	// mu is a lock for the mutable fields of this type. It should be locked
	// when reading or writing any of the mutable fields. The mutable fields are
	// listed below. If pushing to a chan, such as Events, it is unnecessary to
	// lock.
	mu sync.Mutex

	agent *agent.Agent

	queueID          string
	queue            []int64
	queueTotalAmount int64
	queueReady       chan struct{}
	sendingReady     chan struct{}
	idle             chan struct{}
}

func (a *Agent) Open() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Open()
}

// Payment queues a payment which will be paid in the next settlement. The
// identifier for the settlement is returned.
func (a *Agent) Payment(paymentAmount int64) (queueID string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.maxQueueSize != 0 && len(a.queue) == a.maxQueueSize {
		return "", ErrQueueFull
	}
	if paymentAmount > math.MaxInt64-a.queueTotalAmount {
		return "", ErrQueueFull
	}
	a.queue = append(a.queue, paymentAmount)
	a.queueTotalAmount += paymentAmount
	queueID = a.queueID
	select {
	case a.queueReady <- struct{}{}:
	default:
	}
	return
}

// Wait waits for sending to complete and the queue to be empty.
func (a *Agent) Wait() {
	<-a.idle
}

func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.queueReady)
	return a.agent.DeclareClose()
}

func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.queueReady)
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
		switch e := ae.(type) {
		case agent.PaymentReceivedEvent:
			memo, err := parseSettlementMemo(e.CloseAgreement.Envelope.Details.Memo)
			if err != nil {
				a.events <- agent.ErrorEvent{Err: err}
				continue
			}
			a.events <- SettlementReceivedEvent{
				CloseAgreement: e.CloseAgreement,
				ID:             memo.ID,
				Amounts:        memo.Amounts,
			}
		case agent.PaymentSentEvent:
			a.sendingReady <- struct{}{}
			memo, err := parseSettlementMemo(e.CloseAgreement.Envelope.Details.Memo)
			if err != nil {
				a.events <- agent.ErrorEvent{Err: err}
				continue
			}
			a.events <- SettlementSentEvent{
				CloseAgreement: e.CloseAgreement,
				ID:             memo.ID,
				Amounts:        memo.Amounts,
			}
		default:
			a.events <- e
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
		case _, open = <-a.queueReady:
			if !open {
				return
			}
			a.flush()
		default:
			select {
			case _, open = <-a.queueReady:
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
	var queueID string
	var queue []int64
	var queueTotalAmount int64

	func() {
		a.mu.Lock()
		defer a.mu.Unlock()

		queueID = a.queueID
		queue = a.queue
		queueTotalAmount = a.queueTotalAmount
		a.resetQueue()
	}()

	if len(queue) == 0 {
		a.sendingReady <- struct{}{}
		return
	}

	memo := settlementMemo{
		ID:      queueID,
		Amounts: queue,
	}

	err := a.agent.PaymentWithMemo(queueTotalAmount, memo.String())
	if err != nil {
		a.events <- agent.ErrorEvent{Err: err}
		a.sendingReady <- struct{}{}
		return
	}
}

func (a *Agent) resetQueue() {
	a.queueID = uuid.NewString()
	a.queue = nil
	a.queueTotalAmount = 0
}
