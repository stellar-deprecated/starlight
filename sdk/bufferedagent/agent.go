// Package bufferedagent contains a rudimentary and experimental implementation
// of an agent that coordinates a TCP network connection, initial handshake, and
// channel opens, payments, and closes, and buffers outgoing payments,
// collapsing them down to a single payment.
//
// The agent is intended for use in examples only at this point and is not
// intended to be stable or reliable.
package bufferedagent

import (
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/stellar/experimental-payment-channels/sdk/agent"
)

type Config struct {
	Agent       *agent.Agent
	AgentEvents <-chan agent.Event

	LogWriter io.Writer

	Events chan<- interface{}
}

func NewAgent(c Config) *Agent {
	agent := &Agent{
		agent:       c.Agent,
		agentEvents: c.AgentEvents,

		logWriter: c.LogWriter,

		events: c.Events,

		settlementID: uuid.NewString(),
	}
	return agent
}

// BufferedAgent coordinates a payment channel over a TCP connection, and
// buffers payments by collapsing them down into single payments while it waits
// for a change to make a payment itself.
type Agent struct {
	logWriter io.Writer

	agentEvents <-chan agent.Event
	events      chan<- interface{}

	// mu is a lock for the mutable fields of this type. It should be locked
	// when reading or writing any of the mutable fields. The mutable fields are
	// listed below. If pushing to a chan, such as Events, it is unnecessary to
	// lock.
	mu sync.Mutex

	waitingConfirmation bool
	settlementID        string
	queue               []int64
	agent               *agent.Agent
}

func (a *Agent) QueueLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	queueLen := len(a.queue)
	if queueLen == 0 && a.waitingConfirmation {
		return 1
	}
	return queueLen
}

func (a *Agent) Open() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.agent.Open()
}

// Payment queues a payment which will be paid in the next settlement. The
// identifier for the settlement is returned.
func (a *Agent) Payment(paymentAmount int64) (settlementID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.queue = append(a.queue, paymentAmount)
	settlementID = a.settlementID
	if !a.waitingConfirmation {
		a.flushQueue()
	}
	return
}

func (a *Agent) flushQueue() {
	if a.waitingConfirmation {
		return
	}

	if len(a.queue) == 0 {
		return
	}

	memo := settlementMemo{
		ID: a.settlementID,
	}
	a.settlementID = uuid.NewString()

	sum := int64(0)
	for _, paymentAmount := range a.queue {
		// TODO: Handle overflow.
		sum += paymentAmount
		memo.Amounts = append(memo.Amounts, paymentAmount)
	}

	err := a.agent.PaymentWithMemo(sum, memo.String())
	if err != nil {
		a.events <- agent.ErrorEvent{Err: err}
		return
	}
	a.waitingConfirmation = true
	a.queue = a.queue[:0]
}

func (a *Agent) DeclareClose() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// TODO: Handle channel closing but payments still in queue.
	return a.agent.DeclareClose()
}

func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// TODO: Handle channel closing but payments still in queue.
	return a.agent.Close()
}

func (a *Agent) eventLoop() {
	defer close(a.events)
	fmt.Fprintf(a.logWriter, "event loop started\n")
	for {
		switch e := (<-a.agentEvents).(type) {
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
			a.handlePaymentSent()
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
			// TODO: Handle channel closing but payments still in queue.
			a.events <- e
		}
		// TODO: Handle exiting.
	}
}

func (a *Agent) handlePaymentSent() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waitingConfirmation = false
	a.flushQueue()
}
