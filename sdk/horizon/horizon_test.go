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
)

func TestHorizon_StreamTx_longBlock(t *testing.T) {
	client := &horizonclient.MockClient{}
	h := Horizon{HorizonClient: client}

	accountA := keypair.MustRandom()
	client.On(
		"StreamTransactions",
		mock.Anything,
		horizonclient.TransactionRequest{},
		mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		ctx := args[0].(context.Context)
		handler := args[2].(horizonclient.TransactionHandler)
		handler(horizon.Transaction{
			EnvelopeXdr:   "a-txxdr",
			ResultXdr:     "a-resultxdr",
			ResultMetaXdr: "a-resultmetaxdr",
		})
		// Simulate long block on new data from Horizon.
		<-ctx.Done()
	})

	t.Log("Streaming...")
	txsCh, cancel := h.StreamTx("", accountA.FromAddress())

	// Pull streamed transactions into slice.
	t.Log("Pulling some transactions from stream...")
	txs := []agent.StreamedTransaction{}
	txs = append(txs, <-txsCh)

	// Check that the streamed transactions has transactions from A and B.
	assert.ElementsMatch(
		t,
		[]agent.StreamedTransaction{
			{
				TransactionXDR: "a-txxdr",
				ResultXDR:      "a-resultxdr",
				ResultMetaXDR:  "a-resultmetaxdr",
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

func TestHorizon_StreamTx_manyTxs(t *testing.T) {
	client := &horizonclient.MockClient{}
	h := Horizon{HorizonClient: client}

	accountB := keypair.MustRandom()
	client.On(
		"StreamTransactions",
		mock.Anything,
		horizonclient.TransactionRequest{},
		mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		ctx := args[0].(context.Context)
		handler := args[2].(horizonclient.TransactionHandler)
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
	txsCh, cancel := h.StreamTx("", accountB.FromAddress())

	// Pull streamed transactions into slice.
	t.Log("Pulling some transactions from stream...")
	txs := []agent.StreamedTransaction{}
	txs = append(txs, <-txsCh, <-txsCh)

	// Check that the streamed transactions has transactions from A and B.
	assert.ElementsMatch(
		t,
		[]agent.StreamedTransaction{
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
