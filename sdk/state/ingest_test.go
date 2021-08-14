package state

import (
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannel_IngestTx_latestUnauthorizedDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalEscrowAccountBalance(100)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(100)
	responderChannel.UpdateLocalEscrowAccountBalance(100)
	responderChannel.UpdateRemoteEscrowAccountBalance(100)

	// Create a close agreement in initiator that will remain unauthorized in
	// initiator even though it is authorized in responder.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(close)
	require.NoError(t, err)
	assert.Equal(t, int64(0), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())

	// Pretend that responder broadcasts the declaration tx before returning
	// it to the initiator.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(declTx)
	require.NoError(t, err)
	require.Equal(t, CloseClosing, initiatorChannel.CloseState())

	// The initiator channel and responder channel should have the same close
	// agreements.
	assert.Equal(t, int64(8), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())
	assert.Equal(t, initiatorChannel.LatestCloseAgreement(), responderChannel.LatestCloseAgreement())

	// TODO - initiatorChannel should be in a close state now, so shouldn't be able to propose/confirm new payments
}

func TestChannel_IngestTx_latestAuthorizedDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	// Pretend responder broadcasts latest declTx, and
	// initiator ingests it.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(declTx)
	require.NoError(t, err)
	require.Equal(t, CloseClosing, initiatorChannel.CloseState())

	// TODO - initiator should not be able to propose/confirm new close agreements
}

func TestChannel_IngestTx_oldDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalEscrowAccountBalance(100)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(100)
	responderChannel.UpdateLocalEscrowAccountBalance(100)
	responderChannel.UpdateRemoteEscrowAccountBalance(100)

	oldDeclTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)

	// New payment.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	close, err = responderChannel.ConfirmPayment(close)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(close)
	require.NoError(t, err)

	// Pretend that responder broadcasts the old declTx, and
	// initiator ingests it.
	err = initiatorChannel.IngestTx(oldDeclTx)
	require.NoError(t, err)
	require.Equal(t, CloseNeedsClosing, initiatorChannel.CloseState())

	// TODO - initiator should not be able to propose/confirm new close agreements
}

func TestChannel_IngestTx_updateBalances(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	type TestCase struct {
		txSourceAccount   string
		operation         txnbuild.Operation
		wantLocalBalance  int64
		wantRemoteBalance int64
	}

	localAddress := initiatorChannel.localEscrowAccount.Address.Address()
	remoteAddress := initiatorChannel.remoteEscrowAccount.Address.Address()

	testCases := []TestCase{
		// Withdrawals.
		{
			txSourceAccount: localAddress,
			operation: &txnbuild.Payment{
				Destination: keypair.MustRandom().Address(),
				Amount:      "7",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  93_0000000,
			wantRemoteBalance: 100_0000000,
		},
		{
			txSourceAccount: keypair.MustRandom().Address(),
			operation: &txnbuild.Payment{
				Destination:   keypair.MustRandom().Address(),
				Amount:        "8",
				Asset:         txnbuild.NativeAsset{},
				SourceAccount: localAddress,
			},
			wantLocalBalance:  92_0000000,
			wantRemoteBalance: 100_0000000,
		},
		{
			txSourceAccount: remoteAddress,
			operation: &txnbuild.Payment{
				Destination: keypair.MustRandom().Address(),
				Amount:      "7",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  100_0000000,
			wantRemoteBalance: 93_0000000,
		},
		{
			txSourceAccount: keypair.MustRandom().Address(),
			operation: &txnbuild.Payment{
				Destination:   keypair.MustRandom().Address(),
				Amount:        "4",
				Asset:         txnbuild.NativeAsset{},
				SourceAccount: remoteAddress,
			},
			wantLocalBalance:  100_0000000,
			wantRemoteBalance: 96_0000000,
		},
		{
			txSourceAccount: localAddress,
			operation: &txnbuild.Payment{
				Destination:   keypair.MustRandom().Address(),
				Amount:        "11",
				Asset:         txnbuild.NativeAsset{},
				SourceAccount: localAddress,
			},
			wantLocalBalance:  89_0000000,
			wantRemoteBalance: 100_0000000,
		},

		// Deposits.
		{
			txSourceAccount: keypair.MustRandom().Address(),
			operation: &txnbuild.Payment{
				Destination: localAddress,
				Amount:      "7",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  107_0000000,
			wantRemoteBalance: 100_0000000,
		},
		{
			txSourceAccount: keypair.MustRandom().Address(),
			operation: &txnbuild.Payment{
				Destination: remoteAddress,
				Amount:      "7",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  100_0000000,
			wantRemoteBalance: 107_0000000,
		},

		// Deposit and withdrawal.
		{
			txSourceAccount: localAddress,
			operation: &txnbuild.Payment{
				Destination: remoteAddress,
				Amount:      "7",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  93_0000000,
			wantRemoteBalance: 107_0000000,
		},
		{
			txSourceAccount: remoteAddress,
			operation: &txnbuild.Payment{
				Destination: localAddress,
				Amount:      "5",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  105_0000000,
			wantRemoteBalance: 95_0000000,
		},

		// Neither deposit nor withdrawal.
		{
			txSourceAccount: keypair.MustRandom().Address(),
			operation: &txnbuild.Payment{
				Destination: keypair.MustRandom().Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
			wantLocalBalance:  100_0000000,
			wantRemoteBalance: 100_0000000,
		},
	}

	tp := txnbuild.TransactionParams{
		IncrementSequenceNum: true,
		BaseFee:              100000,
		Timebounds:           txnbuild.NewInfiniteTimeout(),
	}

	for _, testCase := range testCases {
		initiatorChannel.UpdateLocalEscrowAccountBalance(100_0000000)
		initiatorChannel.UpdateRemoteEscrowAccountBalance(100_0000000)

		tp.SourceAccount = &txnbuild.SimpleAccount{
			AccountID: testCase.txSourceAccount,
			Sequence:  1,
		}
		tp.Operations = []txnbuild.Operation{testCase.operation}
		tx, err := txnbuild.NewTransaction(tp)
		require.NoError(t, err)

		err = initiatorChannel.ingestTxToUpdateBalances(tx)
		require.NoError(t, err)

		assert.Equal(t, testCase.wantLocalBalance, initiatorChannel.localEscrowAccount.Balance)
		assert.Equal(t, testCase.wantRemoteBalance, initiatorChannel.remoteEscrowAccount.Balance)
	}
}
