package bufferedagent

import (
	"github.com/stellar/experimental-payment-channels/sdk/state"
)

// SettlementReceivedEvent occurs when a settlement is received and the balance
// it agrees to would be the resulting disbursements from the channel if closed.
type SettlementReceivedEvent struct {
	CloseAgreement state.CloseAgreement
	ID             string
	Amounts        []int64
}

// SettlementSentEvent occurs when a settlement is sent and the other
// participant has confirmed it such that the balance the agreement agrees to
// would be the resulting disbursements from the channel if closed.
type SettlementSentEvent struct {
	CloseAgreement state.CloseAgreement
	ID             string
	Amounts        []int64
}
