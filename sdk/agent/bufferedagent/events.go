package bufferedagent

import "github.com/stellar/experimental-payment-channels/sdk/agent"

// BufferedPaymentReceivedEvent occurs when a payment is received that was
// buffered.
type BufferedPaymentsReceivedEvent struct {
	agent.PaymentReceivedEvent
	BufferID string
	Amounts  []int64
}

// BufferedPaymentSentEvent occurs when a payment is sent that was buffered.
type BufferedPaymentsSentEvent struct {
	agent.PaymentSentEvent
	BufferID string
	Amounts  []int64
}
