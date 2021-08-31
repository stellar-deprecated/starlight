package horizon

import (
	"context"
	"fmt"
	"sync"

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

// StreamTx streams transactions that affect the given accounts, sending each
// transaction to the txs channel returned. StreamTx can be stopped by calling
// the cancel function returned. If multiple accounts are given the same
// transaction it may be broadcasted in duplicate if the transaction affects more
// than one account being monitored. The given cursor suppors resuming a
// previous stream.
//
// TODO: Improve StreamTx so that it only streams transactions that affect the
// given accounts. At the moment, to reduce complexity and due to limitations in
// Horizon, it streams all network transactions. See
// https://github.com/stellar/go/issues/3874.
func (h *Horizon) StreamTx(cursor string, accounts ...*keypair.FromAddress) (txs <-chan agent.StreamedTransaction, cancel func()) {
	// txsCh is the channel that streamed transactions will be written to.
	txsCh := make(chan agent.StreamedTransaction)

	// cancelCh will be used to signal the streamer to stop.
	cancelCh := make(chan struct{})

	// Start a streamer that will write txs and stop when
	// signaled to cancel.
	go func() {
		defer close(txsCh)
		h.streamTx(cursor, txsCh, cancelCh)
	}()

	cancelOnce := sync.Once{}
	cancel = func() {
		cancelOnce.Do(func() {
			close(cancelCh)
		})
	}
	return txsCh, cancel
}

func (h *Horizon) streamTx(cursor string, txs chan<- agent.StreamedTransaction, cancel <-chan struct{}) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	go func() {
		<-cancel
		ctxCancel()
	}()
	req := horizonclient.TransactionRequest{
		Cursor: cursor,
	}
	err := h.HorizonClient.StreamTransactions(ctx, req, func(tx horizon.Transaction) {
		streamedTx := agent.StreamedTransaction{
			Cursor:         tx.PagingToken(),
			TransactionXDR: tx.EnvelopeXdr,
			ResultXDR:      tx.ResultXdr,
			ResultMetaXDR:  tx.ResultMetaXdr,
		}
		select {
		case <-cancel:
			ctxCancel()
		case txs <- streamedTx:
		}
	})
	if err != nil {
		// TODO: Handle errors.
		panic(err)
	}
}
