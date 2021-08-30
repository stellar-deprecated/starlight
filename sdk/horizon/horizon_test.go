package horizon

import (
	"context"
	"testing"

	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHorizon_StreamTx(t *testing.T) {
	client := &horizonclient.MockClient{}
	h := Horizon{HorizonClient: client}

	accountA := keypair.MustRandom()
	client.On(
		"StreamTransactions",
		mock.Anything,
		horizonclient.TransactionRequest{ForAccount: accountA.Address()},
		mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		ctx := args[0].(context.Context)
		req := args[1].(horizonclient.TransactionRequest)
		require.Equal(t, accountA.Address(), req.ForAccount)
		handler := args[2].(horizonclient.TransactionHandler)
		handler(horizon.Transaction{
			EnvelopeXdr:   "a-txxdr",
			ResultXdr:     "a-resultxdr",
			ResultMetaXdr: "a-resultmetaxdr",
		})
		// Simulate long block on new data from Horizon.
		<-ctx.Done()
	})

	accountBStart := make(chan struct{})
	accountB := keypair.MustRandom()
	client.On(
		"StreamTransactions",
		mock.Anything,
		horizonclient.TransactionRequest{ForAccount: accountB.Address()},
		mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		ctx := args[0].(context.Context)
		req := args[1].(horizonclient.TransactionRequest)
		require.Equal(t, accountB.Address(), req.ForAccount)
		handler := args[2].(horizonclient.TransactionHandler)
		// Wait starting until signaled.
		<-accountBStart
		// Simulate many transactions coming from Horizon.
		for {
			select {
			case <-ctx.Done():
				return
			default:
				handler(horizon.Transaction{
					EnvelopeXdr:   "b-txxdr",
					ResultXdr:     "b-resultxdr",
					ResultMetaXdr: "b-resultmetaxdr",
				})
			}
		}
	})

	t.Log("Streaming...")
	txsCh, cancel := h.StreamTx([]*keypair.FromAddress{accountA.FromAddress(), accountB.FromAddress()})

	// Pull streamed transactions into slice.
	t.Log("Pulling some transactions from stream...")
	txs := []agent.StreamedTransaction{}
	txs = append(txs, <-txsCh)

	// Signal to accountB's client that it can start producing too and pull more.
	close(accountBStart)
	txs = append(txs, <-txsCh, <-txsCh)

	// Check that the streamed transactions has transactions from A and B.
	assert.ElementsMatch(
		t,
		[]agent.StreamedTransaction{
			{
				TransactionXDR: "a-txxdr",
				ResultXDR:      "a-resultxdr",
				ResultMetaXDR:  "a-resultmetaxdr",
			},
			{
				TransactionXDR: "b-txxdr",
				ResultXDR:      "b-resultxdr",
				ResultMetaXDR:  "b-resultmetaxdr",
			},
			{
				TransactionXDR: "b-txxdr",
				ResultXDR:      "b-resultxdr",
				ResultMetaXDR:  "b-resultmetaxdr",
			},
		},
		txs,
	)

	// Cancel streaming, and check that multiple cancels are okay.
	t.Log("Canceling...")
	cancel()
	cancel()

	// Check that the transaction stream channel is closed. It may still be
	// producing transactions for a short period of time.
	open := true
	for open {
		_, open = <-txsCh
		t.Log("Still open, waiting for cancel...")
	}
	assert.False(t, open, "txs channel not closed but should be after cancel called")
}
