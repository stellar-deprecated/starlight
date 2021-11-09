package bufferedagent

// BufferedPayment contains the details of a payment that is buffered and
// transmitted in the memo of an agreement on the payment channel.
type BufferedPayment struct {
	Amount int64
	Memo   string
}
