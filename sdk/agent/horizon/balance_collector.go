package horizon

import (
	"fmt"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/starlight/sdk/state"
)

type BalanceCollector struct {
	HorizonClient horizonclient.ClientInterface
}

func (h *BalanceCollector) GetBalance(accountID *keypair.FromAddress, asset state.Asset) (int64, error) {
	var account horizon.Account
	account, err := h.HorizonClient.AccountDetail(horizonclient.AccountRequest{AccountID: accountID.Address()})
	if err != nil {
		return 0, fmt.Errorf("getting account details of %s: %w", accountID, err)
	}
	for _, b := range account.Balances {
		if b.Asset.Code == asset.Code() || b.Asset.Issuer == asset.Issuer() {
			balance, err := amount.ParseInt64(account.Balances[0].Balance)
			if err != nil {
				return 0, fmt.Errorf("parsing %s balance of %s: %w", asset, accountID, err)
			}
			return balance, nil
		}
	}
	return 0, nil
}
