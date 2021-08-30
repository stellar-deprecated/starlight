package horizon

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
)

type Horizon struct {
	HorizonClient horizonclient.ClientInterface
}

func (h *Horizon) GetBalance(accountID *keypair.FromAddress, asset state.Asset) (int64, error) {
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

func (h *Horizon) GetSequenceNumber(accountID *keypair.FromAddress) (int64, error) {
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

func (h *Horizon) SubmitTx(xdr string) error {
	_, err := h.HorizonClient.SubmitTransactionXDR(xdr)
	if err != nil {
		return fmt.Errorf("submitting tx %s: %w", xdr, buildErr(err))
	}
	return nil
}

func buildErr(err error) error {
	if hErr := horizonclient.GetError(err); hErr != nil {
		resultString, rErr := hErr.ResultString()
		if rErr != nil {
			resultString = "<error getting result string: " + rErr.Error() + ">"
		}
		return fmt.Errorf("%w (%v)", err, resultString)
	}
	return err
}

func (h *Horizon) StreamTransactions(accounts []*keypair.FromAddress, transactions chan<- agent.StreamedTransaction) (cancel func()) {
	// TODO: Implement streaming of transactions for the accounts.
	return nil
}
