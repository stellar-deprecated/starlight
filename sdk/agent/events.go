package agent

import "github.com/stellar/experimental-payment-channels/sdk/state"

// ErrorEvent occurs when an error has occurred, and contains the error
// occurred.
type ErrorEvent struct {
	Err error
}

// ConnectedEvent occurs when the agent is connected to another participant.
type ConnectedEvent struct{}

// OpenedEvent occurs when the channel has been opened.
type OpenedEvent struct{}

// PaymentReceivedEvent occurs when a payment is received and the balance it
// agrees to would be the resulting disbursements from the channel if closed.
type PaymentReceivedEvent struct {
	CloseAgreement state.CloseAgreement
}

// PaymentSentEvent occurs when a payment is sent and the other participant has
// confirmed it such that the balance the agreement agrees to would be the
// resulting disbursements from the channel if closed.
type PaymentSentEvent struct {
	CloseAgreement state.CloseAgreement
}

// ClosingEvent occurs when the channel is closing and no new payments should be
// proposed or confirmed.
type ClosingEvent struct{}

// ClosingWithOutdatedStateEvent occurs when the channel is closing and no new payments should be
// proposed or confirmed, and the state it is closing in is not the latest known state.
type ClosingWithOutdatedStateEvent struct{}

// ClosedEvent occurs when the channel is successfully closed.
type ClosedEvent struct{}
