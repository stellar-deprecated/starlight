package agent

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
)

func getSeqNum(client horizonclient.ClientInterface, accountID string) (int64, error) {
	account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: accountID})
	if err != nil {
		return 0, fmt.Errorf("getting account %s: %w", accountID, err)
	}
	seqNum, err := account.GetSequenceNumber()
	if err != nil {
		return 0, fmt.Errorf("getting sequence number of account %s: %w", accountID, err)
	}
	return seqNum, nil
}
