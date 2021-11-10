package state

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stellar/starlight/sdk/txbuild/txbuildtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertChannelSnapshotsAndRestores(t *testing.T, config Config, channel *Channel) {
	t.Helper()

	snapshot := channel.Snapshot()
	snapshotJSON, err := json.Marshal(snapshot)
	require.NoError(t, err)
	restoredSnapshot := Snapshot{}
	err = json.Unmarshal(snapshotJSON, &restoredSnapshot)
	require.NoError(t, err)

	restoredChannel := NewChannelFromSnapshot(config, snapshot)

	require.Equal(t, channel, restoredChannel)
}

func TestNewChannelWithSnapshot(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localChannelAccount := keypair.MustRandom().FromAddress()
	remoteChannelAccount := keypair.MustRandom().FromAddress()

	localConfig := Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            true,
		LocalSigner:          localSigner,
		RemoteSigner:         remoteSigner.FromAddress(),
		LocalChannelAccount:  localChannelAccount,
		RemoteChannelAccount: remoteChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	}
	localChannel := NewChannel(localConfig)
	remoteConfig := Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            false,
		LocalSigner:          remoteSigner,
		RemoteSigner:         localSigner.FromAddress(),
		LocalChannelAccount:  remoteChannelAccount,
		RemoteChannelAccount: localChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	}
	remoteChannel := NewChannel(remoteConfig)

	// Check snapshot rehydrates the channel identically when new and not open.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Negotiate the open state.
	{
		open1, err := localChannel.ProposeOpen(OpenParams{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			ExpiresAt:                  time.Now().Add(time.Hour),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		open2, err := remoteChannel.ConfirmOpen(open1.Envelope)
		require.NoError(t, err)
		_, err = localChannel.ConfirmOpen(open2.Envelope)
		require.NoError(t, err)
	}

	// Check snapshot rehydrates the channel identically when open.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Put the channel into an open state by ingesting the open tx.
	{
		ftx, err := localChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:         localSigner.Address(),
			ResponderSigner:         remoteSigner.Address(),
			InitiatorChannelAccount: localChannelAccount.Address(),
			ResponderChannelAccount: remoteChannelAccount.Address(),
			StartSequence:           101,
			Asset:                   txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = localChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = remoteChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := localChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = remoteChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// Check snapshot rehydrates the channel identically when open.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Update balances.
	localChannel.UpdateLocalChannelAccountBalance(100)
	localChannel.UpdateRemoteChannelAccountBalance(200)
	remoteChannel.UpdateLocalChannelAccountBalance(300)
	remoteChannel.UpdateRemoteChannelAccountBalance(400)

	// Check snapshot rehydrates the channel identically when open and with
	// balances updated.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Propose a payment.
	ca, err := localChannel.ProposePayment(10)
	require.NoError(t, err)

	// Check snapshot rehydrates the channel identically when payment proposed.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Confirm the payment.
	ca, err = remoteChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
	_, err = localChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)

	// Check snapshot rehydrates the channel identically when payment confirmed.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Propose a close.
	ca, err = localChannel.ProposeClose()
	require.NoError(t, err)

	// Check snapshot rehydrates the channel identically when payment confirmed.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Put the channel into a closing state by ingesting the closing declaration tx.
	{
		dtx, _, err := localChannel.CloseTxs()
		require.NoError(t, err)
		dtxXDR, err := dtx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)

		resultMetaXDR, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
			{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(localChannelAccount.Address()),
					SeqNum:    102,
					Signers: []xdr.Signer{
						{Key: xdr.MustSigner(localSigner.Address()), Weight: 1},
						{Key: xdr.MustSigner(remoteSigner.Address()), Weight: 1},
					},
					Thresholds: xdr.Thresholds{0, 2, 2, 2},
				},
			},
			{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(remoteChannelAccount.Address()),
					SeqNum:    103,
					Signers: []xdr.Signer{
						{Key: xdr.MustSigner(remoteSigner.Address()), Weight: 1},
						{Key: xdr.MustSigner(localSigner.Address()), Weight: 1},
					},
					Thresholds: xdr.Thresholds{0, 2, 2, 2},
				},
			},
		})
		require.NoError(t, err)

		err = localChannel.IngestTx(1, dtxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = remoteChannel.IngestTx(1, dtxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := localChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosing, cs)

		cs, err = remoteChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosing, cs)
	}

	// Check snapshot rehydrates the channel identically when closing ingested.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Confirm the close.
	ca, err = remoteChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)
	_, err = localChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)

	// Check snapshot rehydrates the channel identically when payment confirmed.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)

	// Put the channel into a close state by ingesting the close tx.
	{
		_, ctx, err := localChannel.CloseTxs()
		require.NoError(t, err)
		ctxXDR, err := ctx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)

		resultMetaXDR, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{})
		require.NoError(t, err)

		err = localChannel.IngestTx(1, ctxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = remoteChannel.IngestTx(1, ctxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := localChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosed, cs)

		cs, err = remoteChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosed, cs)
	}

	// Check snapshot rehydrates the channel identically when payment confirmed.
	assertChannelSnapshotsAndRestores(t, localConfig, localChannel)
	assertChannelSnapshotsAndRestores(t, remoteConfig, remoteChannel)
}
