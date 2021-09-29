// Package bufferedagent contains a rudimentary and experimental implementation
// of an agent that coordinates a TCP network connection, initial handshake, and
// channel opens, payments, and closes, and buffers outgoing payments,
// collapsing them down to a single payment.
//
// The agent is intended for use in examples only at this point and is not
// intended to be stable or reliable.
package bufferedagent

import (
	"io"
	"sync"

	"github.com/stellar/experimental-payment-channels/sdk/agent"
)

type Config struct {
	Agent       *agent.Agent
	AgentEvents <-chan agent.Event

	LogWriter io.Writer

	Events chan<- agent.Event
}

func NewBufferedAgent(c Config) *Agent {
	agent := &Agent{
		agent:       c.Agent,
		agentEvents: c.AgentEvents,

		logWriter: c.LogWriter,

		events: c.Events,
	}
	return agent
}

// BufferedAgent coordinates a payment channel over a TCP connection, and
// buffers payments by collapsing them down into single payments while it waits
// for a change to make a payment itself.
type Agent struct {
	logWriter io.Writer

	agentEvents <-chan agent.Event
	events      chan<- agent.Event

	// mu is a lock for the mutable fields of this type. It should be locked
	// when reading or writing any of the mutable fields. The mutable fields are
	// listed below. If pushing to a chan, such as Events, it is unnecessary to
	// lock.
	mu sync.Mutex

	queue []int64
	agent *agent.Agent
}

func (a *Agent) Open() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Open()
}

func (a *Agent) Payment(paymentAmount int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.queue = append(a.queue, paymentAmount)
	return nil
}

func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.DeclareClose()
}

func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Close()
}

func (a *Agent) eventLoop() {
	defer close(a.events)
	for {
		switch e := (<-a.agentEvents).(type) {
		default:
			a.events <- e
		case agent.PaymentSentEvent:
			a.flushQueue()
		}
		// TODO: Handle case where channel closing but payments still in queue.
	}
}

func (a *Agent) flushQueue() {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := a.agent.Payment(0)
}
