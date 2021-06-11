package integration

import (
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	stellarAmount "github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/require"
)

// functions to be used in the state_test integration tests

func initAccounts(t *testing.T) (initiator Participant, responder Participant) {
	initiator = Participant{
		Name:         "Initiator",
		KP:           keypair.MustRandom(),
		Escrow:       keypair.MustRandom(),
		Contribution: 1_000_0000000,
	}
	t.Log("Initiator:", initiator.KP.Address())
	t.Log("Initiator Escrow:", initiator.Escrow.Address())
	{
		err := retry(2, func() error { return fund(client, initiator.KP.FromAddress(), 10_000_0000000) })
		require.NoError(t, err)
		account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: initiator.KP.Address()})
		require.NoError(t, err)
		seqNum, err := account.GetSequenceNumber()
		require.NoError(t, err)
		tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
			Creator:             initiator.KP.FromAddress(),
			Escrow:              initiator.Escrow.FromAddress(),
			SequenceNumber:      seqNum + 1,
			InitialContribution: initiator.Contribution,
		})
		require.NoError(t, err)
		tx, err = tx.Sign(networkPassphrase, initiator.KP, initiator.Escrow)
		require.NoError(t, err)
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		txResp, err := client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
		initiator.EscrowSequenceNumber = int64(txResp.Ledger) << 32
	}
	t.Log("Initiator Escrow Sequence Number:", initiator.EscrowSequenceNumber)
	t.Log("Initiator Contribution:", initiator.Contribution)

	// Setup responder.
	responder = Participant{
		Name:         "Responder",
		KP:           keypair.MustRandom(),
		Escrow:       keypair.MustRandom(),
		Contribution: 1_000_0000000,
	}
	t.Log("Responder:", responder.KP.Address())
	t.Log("Responder Escrow:", responder.Escrow.Address())
	{
		err := retry(2, func() error { return fund(client, responder.KP.FromAddress(), 10_000_0000000) })
		require.NoError(t, err)
		account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: responder.KP.Address()})
		require.NoError(t, err)
		seqNum, err := account.GetSequenceNumber()
		require.NoError(t, err)
		tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
			Creator:             responder.KP.FromAddress(),
			Escrow:              responder.Escrow.FromAddress(),
			SequenceNumber:      seqNum + 1,
			InitialContribution: responder.Contribution,
		})
		require.NoError(t, err)
		tx, err = tx.Sign(networkPassphrase, responder.KP, responder.Escrow)
		require.NoError(t, err)
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: responder.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
		require.NoError(t, err)
		txResp, err := client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
		responder.EscrowSequenceNumber = int64(txResp.Ledger) << 32
	}
	t.Log("Responder Escrow Sequence Number:", responder.EscrowSequenceNumber)
	t.Log("Responder Contribution:", responder.Contribution)
	return initiator, responder
}

func initChannels(t *testing.T, initiator Participant, responder Participant) (initiatorChannel *state.Channel, responderChannel *state.Channel) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

	initiatorChannel = state.NewChannel(state.Config{
		NetworkPassphrase:          networkPassphrase,
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Initiator:                  true,
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: initiator.Contribution},
			},
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: responder.Contribution},
			},
		},
		LocalSigner:  initiator.KP,
		RemoteSigner: responder.KP.FromAddress(),
	})
	responderChannel = state.NewChannel(state.Config{
		NetworkPassphrase:          networkPassphrase,
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Initiator:                  false,
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: responder.Contribution},
			},
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: initiator.Contribution},
			},
		},
		LocalSigner:  responder.KP,
		RemoteSigner: initiator.KP.FromAddress(),
	})
	return initiatorChannel, responderChannel
}

func randomBool(t *testing.T) bool {
	t.Helper()
	b := [1]byte{}
	_, err := rand.Read(b[:])
	require.NoError(t, err)
	return b[0]%2 == 0
}

func randomPositiveInt64(t *testing.T, max int64) int64 {
	t.Helper()
	var i uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &i)
	require.NoError(t, err)
	return int64(i) % max
}

func retry(maxAttempts int, f func() error) (err error) {
	for i := 0; i < maxAttempts; i++ {
		err = f()
		if err == nil {
			return
		}
	}
	return err
}

func fund(client horizonclient.ClientInterface, account *keypair.FromAddress, startingBalance int64) error {
	rootResp, err := client.Root()
	if err != nil {
		return err
	}
	root := keypair.Master(rootResp.NetworkPassphrase).(*keypair.Full)
	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: root.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.CreateAccount{
					Destination: account.Address(),
					Amount:      stellarAmount.StringFromInt64(startingBalance),
				},
			},
		},
	)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(rootResp.NetworkPassphrase, root)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}

func txSeqs(txs []*txnbuild.Transaction) []int64 {
	seqs := make([]int64, len(txs))
	for i := range txs {
		seqs[i] = txs[i].SequenceNumber()
	}
	return seqs
}
