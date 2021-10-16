package bufferedagent

import "github.com/stellar/experimental-payment-channels/sdk/agent"

// BufferedPaymentReceivedEvent occurs when a payment is received that was
// buffered.
type BufferedPaymentsReceivedEvent struct {
	agent.PaymentReceivedEvent
	BufferID string
	Payments []BufferedPayment
}

// BufferedPaymentSentEvent occurs when a payment is sent that was buffered.
type BufferedPaymentsSentEvent struct {
	agent.PaymentSentEvent
	BufferID string
	Payments []BufferedPayment
}
