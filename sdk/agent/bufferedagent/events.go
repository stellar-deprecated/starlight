package bufferedagent

import "github.com/stellar/starlight/sdk/agent"

// BufferedPaymentReceivedEvent occurs when a payment is received that was
// buffered.
type BufferedPaymentsReceivedEvent struct {
	agent.PaymentReceivedEvent
	BufferID       string
	BufferByteSize int
	Payments       []BufferedPayment
}

// BufferedPaymentSentEvent occurs when a payment is sent that was buffered.
type BufferedPaymentsSentEvent struct {
	agent.PaymentSentEvent
	BufferID       string
	BufferByteSize int
	Payments       []BufferedPayment
}
