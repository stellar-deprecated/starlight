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
	sendingChannel.latestCloseAgreement.Balance = Amount{Asset: NativeAsset{}}
	receiverChannel.latestCloseAgreement.Balance = Amount{Asset: NativeAsset{}}

	p, err := sendingChannel.ProposePayment(Amount{
		Asset:  NativeAsset{},
		Amount: 200,
	})
	require.NoError(t, err)

	p, fullySigned, err := receiverChannel.ConfirmPayment(p)
	assert.False(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, p, receiverChannel.latestUnconfirmedPayment)

	// Confirming a payment with same sequence number but different Amount should error
	pDifferent := Payment{
		IterationNumber: 1,
		Amount: Amount{
			Asset:  txnbuild.NativeAsset{},
			Amount: 400,
		},
		CloseSignatures: p.CloseSignatures,
	}
	_, fullySigned, err = receiverChannel.ConfirmPayment(pDifferent)
	assert.False(t, fullySigned)
	require.Error(t, err)
	require.Equal(t, "a different unconfirmed payment exists", err.Error())
	assert.Equal(t, p, receiverChannel.latestUnconfirmedPayment)
	assert.Equal(t, CloseAgreement{Balance: Amount{Asset: NativeAsset{}}}, receiverChannel.LatestCloseAgreement())

	// Confirming a payment with same sequence number and same amount should pass
	p, fullySigned, err = sendingChannel.ConfirmPayment(p)
	assert.True(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, Payment{}, sendingChannel.latestUnconfirmedPayment)

	p, fullySigned, err = receiverChannel.ConfirmPayment(p)
	assert.True(t, fullySigned)
	require.NoError(t, err)
	assert.Equal(t, Payment{}, receiverChannel.latestUnconfirmedPayment)
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
