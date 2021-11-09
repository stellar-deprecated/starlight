package horizon

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/starlight/sdk/agent"
)

var _ agent.SequenceNumberCollector = &SequenceNumberCollector{}

// SequenceNumberCollector implements an agent's interface for collecting the
// current sequence number by querying Horizon's accounts endpoint.
type SequenceNumberCollector struct {
	HorizonClient horizonclient.ClientInterface
}

// GetSequenceNumber queries Horizon for the balance of the given account.
func (h *SequenceNumberCollector) GetSequenceNumber(accountID *keypair.FromAddress) (int64, error) {
	account, err := h.HorizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountID.Address()})
	if err != nil {
		return 0, fmt.Errorf("getting account details of %s: %w", accountID, err)
	}
	seqNum, err := account.GetSequenceNumber()
	if err != nil {
		return 0, fmt.Errorf("getting sequence number of account %s: %w", accountID, err)
	}
	return seqNum, nil
}
