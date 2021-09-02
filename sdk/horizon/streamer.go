package horizon

import (
	"context"
	"sync"

	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
)

type Streamer struct {
	HorizonClient horizonclient.ClientInterface
	ErrorHandler  func(error)
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
func (h *Streamer) StreamTx(cursor string, accounts ...*keypair.FromAddress) (txs <-chan agent.StreamedTransaction, cancel func()) {
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

func (h *Streamer) streamTx(cursor string, txs chan<- agent.StreamedTransaction, cancel <-chan struct{}) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	go func() {
		<-cancel
		ctxCancel()
	}()
	req := horizonclient.TransactionRequest{
		Cursor: cursor,
	}
	for {
		err := h.HorizonClient.StreamTransactions(ctx, req, func(tx horizon.Transaction) {
			cursor = tx.PagingToken()
			streamedTx := agent.StreamedTransaction{
				Cursor:         cursor,
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
		if err == nil {
			break
		}
		if h.ErrorHandler != nil {
			h.ErrorHandler(err)
		}
	}
}
