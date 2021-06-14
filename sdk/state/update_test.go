package state

import (
	"crypto/rand"
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
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
			Asset:  NativeAsset{},
			Amount: 400,
		},
		CloseSignatures: p.CloseSignatures,
	}
	_, fullySigned, err = receiverChannel.ConfirmPayment(pDifferent)
	assert.False(t, fullySigned)
	require.Error(t, err)
	require.Equal(t, "a different unconfirmed payment exists", err.Error())
	assert.Equal(t, p, receiverChannel.latestUnconfirmedPayment)
	assert.Equal(t, CloseAgreement{}, receiverChannel.LatestCloseAgreement())

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
	// TODO - put this code somewhere for re-use (use the integration function code?)
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

	rand1, err := randomByteArray(10)
	require.NoError(t, err)
	rand2, err := randomByteArray(10)
	require.NoError(t, err)
	newCloseSignatures := []xdr.DecoratedSignature{
		xdr.DecoratedSignature{
			Signature: rand1,
		},
		xdr.DecoratedSignature{
			Signature: rand2,
		},
	}

	sendingChannel.latestUnconfirmedPayment.CloseSignatures = appendNewSignatures(sendingChannel.latestUnconfirmedPayment.CloseSignatures, newCloseSignatures)
	assert.Equal(t, 2, len(sendingChannel.latestUnconfirmedPayment.CloseSignatures))
	assert.Equal(t, newCloseSignatures[0], sendingChannel.latestUnconfirmedPayment.CloseSignatures[0])
	assert.Equal(t, newCloseSignatures[1], sendingChannel.latestUnconfirmedPayment.CloseSignatures[1])

	// 1 new signature is introduced
	rand3, err := randomByteArray(10)
	require.NoError(t, err)
	newCloseSignatures = append(newCloseSignatures, xdr.DecoratedSignature{
		Signature: rand3,
	})
	sendingChannel.latestUnconfirmedPayment.CloseSignatures = appendNewSignatures(sendingChannel.latestUnconfirmedPayment.CloseSignatures, newCloseSignatures)
	assert.Equal(t, 3, len(sendingChannel.latestUnconfirmedPayment.CloseSignatures))
	for i, ncs := range newCloseSignatures {
		assert.Equal(t, ncs, sendingChannel.latestUnconfirmedPayment.CloseSignatures[i])
	}
}

// TODO - move to another folder for helper methods?
func randomByteArray(length int) ([]byte, error) {
	arr := make([]byte, length)
	_, err := rand.Read(arr)
	if err != nil {
		return nil, err
	}
	return arr, nil
}
