package state

import (
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannel_IngestTx_updateUnauthorizedCloseAgreement(t *testing.T) {
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

	// Pretend that responder broadcasts the declaration tx from the authorized
	// agreement to the network, making it visible to initiator.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(declTx)
	require.NoError(t, err)

	// The initiator channel and responder channel should have the same close
	// agreements.
	initiatorChannel.Balance()
	assert.Equal(t, int64(8), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())
	assert.Equal(t, initiatorChannel.LatestCloseAgreement(), responderChannel.LatestCloseAgreement())
}
