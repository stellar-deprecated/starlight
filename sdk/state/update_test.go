package state

import (
	"math/rand"
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastConfirmedPayment(t *testing.T) {
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
	sendingChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
	})

	// latest close agreement should be set during open steps
	sendingChannel.latestAuthorizedCloseAgreement.Balance = Amount{Asset: NativeAsset{}}
	receiverChannel.latestAuthorizedCloseAgreement.Balance = Amount{Asset: NativeAsset{}}

	ca, err := sendingChannel.ProposePayment(Amount{
		Asset:  NativeAsset{},
		Amount: 200,
	})
	require.NoError(t, err)

	ca, fullySigned, err := receiverChannel.ConfirmPayment(ca)
	assert.False(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, ca, receiverChannel.latestUnauthorizedCloseAgreement)

	// Confirming a close agreement with same sequence number but different Amount should error
	caDifferent := CloseAgreement{
		IterationNumber: 1,
		Balance: Amount{
			Asset:  txnbuild.NativeAsset{},
			Amount: 400,
		},
		CloseSignatures: ca.CloseSignatures,
	}
	_, fullySigned, err = receiverChannel.ConfirmPayment(caDifferent)
	assert.False(t, fullySigned)
	require.EqualError(t, err, "close agreement does not match the close agreement already in progress")
	assert.Equal(t, CloseAgreement{Balance: Amount{Asset: NativeAsset{}}}, receiverChannel.LatestCloseAgreement())

	// Confirming a payment with same sequence number and same amount should pass
	ca, fullySigned, err = sendingChannel.ConfirmPayment(ca)
	assert.True(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, CloseAgreement{}, sendingChannel.latestUnauthorizedCloseAgreement)

	ca, fullySigned, err = receiverChannel.ConfirmPayment(ca)
	assert.True(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, CloseAgreement{}, receiverChannel.latestUnauthorizedCloseAgreement)
}

func TestAppendNewSignature(t *testing.T) {
	closeSignatures := []xdr.DecoratedSignature{
		{Signature: randomByteArray(t, 10)},
		{Signature: randomByteArray(t, 10)},
	}

	closeSignaturesToAppend := []xdr.DecoratedSignature{
		closeSignatures[0], // A duplicate signature is included.
		{Signature: randomByteArray(t, 10)},
	}

	newCloseSignatures := appendNewSignatures(closeSignatures, closeSignaturesToAppend)

	// Check that the final slice of signatures does not contain the duplicate.
	assert.ElementsMatch(
		t,
		newCloseSignatures,
		[]xdr.DecoratedSignature{
			closeSignatures[0],
			closeSignatures[1],
			closeSignaturesToAppend[1],
		},
	)

	// Check existing signatures are not lost.
	newCloseSignatures = appendNewSignatures(closeSignatures, []xdr.DecoratedSignature{})

	assert.ElementsMatch(
		t,
		newCloseSignatures,
		[]xdr.DecoratedSignature{
			closeSignatures[0],
			closeSignatures[1],
		},
	)
}

func randomByteArray(t *testing.T, length int) []byte {
	arr := make([]byte, length)
	_, err := rand.Read(arr)
	require.NoError(t, err)
	return arr
}
