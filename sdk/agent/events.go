package agent

import "github.com/stellar/experimental-payment-channels/sdk/state"

type Event interface {
	event()
}

// ErrorEvent occurs when an error has occurred, and contains the error
// occurred.
type ErrorEvent struct {
	Err error
}

func (e ErrorEvent) event() {}

// ConnectedEvent occurs when the agent is connected to another participant.
type ConnectedEvent struct{}

func (e ConnectedEvent) event() {}

// OpenedEvent occurs when the channel has been opened.
type OpenedEvent struct{}

func (e OpenedEvent) event() {}

// PaymentReceivedEvent occurs when a payment is received and the balance it
// agrees to would be the resulting disbursements from the channel if closed.
type PaymentReceivedEvent struct {
	CloseAgreement state.CloseEnvelope
}

func (e PaymentReceivedEvent) event() {}

// PaymentSentEvent occurs when a payment is sent and the other participant has
// confirmed it such that the balance the agreement agrees to would be the
// resulting disbursements from the channel if closed.
type PaymentSentEvent struct {
	CloseAgreement state.CloseEnvelope
}

func (e PaymentSentEvent) event() {}

// ClosingEvent occurs when the channel is closing and no new payments should be
// proposed or confirmed.
type ClosingEvent struct{}

func (e ClosingEvent) event() {}

// ClosingWithOutdatedStateEvent occurs when the channel is closing and no new payments should be
// proposed or confirmed, and the state it is closing in is not the latest known state.
type ClosingWithOutdatedStateEvent struct{}

func (e ClosingWithOutdatedStateEvent) event() {}

// ClosedEvent occurs when the channel is successfully closed.
type ClosedEvent struct{}

func (e ClosedEvent) event() {}
