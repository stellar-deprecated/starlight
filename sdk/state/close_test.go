package state

import (
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestChannel_CloseTx(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	channel.openAgreement = OpenAgreement{
		Details: OpenAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now(),
		},
	}
	channel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 2,
			IterationNumber:            3,
			Balance:                    4,
		},
		DeclarationSignatures: []xdr.DecoratedSignature{{Hint: [4]byte{0, 0, 0, 0}, Signature: []byte{0}}},
		CloseSignatures:       []xdr.DecoratedSignature{{Hint: [4]byte{1, 1, 1, 1}, Signature: []byte{1}}},
	}

	declTx, closeTx, err := channel.CloseTxs()
	require.NoError(t, err)
	// TODO: Compare the non-signature parts of the txs with the result of
	// channel.closeTxs() when there is an practical way of doing that added to
	// txnbuild.
	assert.Equal(t, []xdr.DecoratedSignature{{Hint: [4]byte{0, 0, 0, 0}, Signature: []byte{0}}}, declTx.Signatures())
	assert.Equal(t, []xdr.DecoratedSignature{{Hint: [4]byte{1, 1, 1, 1}, Signature: []byte{1}}}, closeTx.Signatures())
}

func TestChannel_ConfirmClose_checksForExtraSignatures(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}

	senderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
	})

	ca, err := senderChannel.ProposeClose()
	require.NoError(t, err)

	// Adding extra signature should cause error
	ca.CloseSignatures = append(ca.CloseSignatures, xdr.DecoratedSignature{Signature: randomByteArray(t, 10)})
	_, authorized, err := responderChannel.ConfirmClose(ca)
	require.EqualError(t, err, "close agreement has too many signatures, has declaration: 0, close: 3, max of 2 allowed for each")
	assert.False(t, authorized)

	// Remove extra signature, now should succeed
	ca.CloseSignatures = ca.CloseSignatures[0:1]
	_, authorized, err = responderChannel.ConfirmClose(ca)
	require.NoError(t, err)
	require.True(t, authorized)
}
