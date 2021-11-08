package horizon

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
)

type SequenceNumberCollector struct {
	HorizonClient horizonclient.ClientInterface
}

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
