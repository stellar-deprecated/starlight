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

	t.Log("Pulling some transactions from stream...")
	txs := []agent.StreamedTransaction{}
	txs = append(txs, <-txsCh)
	close(accountBStart) // Block accountB's client from producing until accountA produces at least one.
	txs = append(txs, <-txsCh, <-txsCh)

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
	t.Log("Canceling...")
	cancel()
	cancel() // Check that multiple cancels are okay.
	_, ok := <-txsCh
	assert.False(t, ok, "txs channel not closed but should be after cancel called")
}
