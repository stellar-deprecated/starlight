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

func TestChannel_ConfirmPayment_rejectsDifferentObservationPeriod(t *testing.T) {
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

	// Given a channel with observation periods set to 1, that is already open.
	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	channel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
		},
	}

	// A close agreement from the remote participant should be accepted if the
	// observation period matches the channels observation period.
	{
		_, txClose, err := channel.CloseTxs(CloseAgreementDetails{
			IterationNumber:            1,
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
		})
		require.NoError(t, err)
		txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		_, _, err = channel.ConfirmPayment(CloseAgreement{
			Details: CloseAgreementDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
			},
			CloseSignatures: txClose.Signatures(),
		})
		require.NoError(t, err)
	}

	// A close agreement from the remote participant should be rejected if the
	// observation period doesn't match the channels observation period.
	{
		_, txClose, err := channel.CloseTxs(CloseAgreementDetails{
			IterationNumber:            1,
			ObservationPeriodTime:      0,
			ObservationPeriodLedgerGap: 0,
		})
		require.NoError(t, err)
		txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		_, _, err = channel.ConfirmPayment(CloseAgreement{
			Details: CloseAgreementDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      0,
				ObservationPeriodLedgerGap: 0,
			},
			CloseSignatures: txClose.Signatures(),
		})
		require.EqualError(t, err, "invalid payment observation period: different than channel state")
	}
}

func TestChannel_ConfirmPayment_initiatorRejectsPaymentToRemote(t *testing.T) {
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

	// Given a channel with observation periods set to 1, that is already open.
	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	channel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber: 1,
			Balance: Amount{
				Asset:  NativeAsset{},
				Amount: 100, // Local (initiator) owes remote (responder) 100.
			},
		},
	}
	ca := CloseAgreementDetails{
		IterationNumber: 2,
		Balance: Amount{
			Asset:  NativeAsset{},
			Amount: 110, // Local (initiator) owes remote (responder) 110, payment of 10 from ❌ local to remote.
		},
	}
	_, txClose, err := channel.CloseTxs(ca)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, _, err = channel.ConfirmPayment(CloseAgreement{
		Details:         ca,
		CloseSignatures: txClose.Signatures(),
	})
	require.EqualError(t, err, "close agreement from remote changes the balance in their favor")
}

func TestChannel_ConfirmPayment_responderRejectsPaymentToRemote(t *testing.T) {
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

	// Given a channel with observation periods set to 1, that is already open.
	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	channel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber: 1,
			Balance: Amount{
				Asset:  NativeAsset{},
				Amount: 100, // Local (responder) owes remote (initiator) 100.
			},
		},
	}
	ca := CloseAgreementDetails{
		IterationNumber: 2,
		Balance: Amount{
			Asset:  NativeAsset{},
			Amount: 90, // Local (responder) owes remote (initiator) 110, payment of 10 from ❌ local to remote.
		},
	}
	_, txClose, err := channel.CloseTxs(ca)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, _, err = channel.ConfirmPayment(CloseAgreement{
		Details:         ca,
		CloseSignatures: txClose.Signatures(),
	})
	require.EqualError(t, err, "close agreement from remote changes the balance in their favor")
}

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
	sendingChannel.latestAuthorizedCloseAgreement.Details.Balance = Amount{Asset: NativeAsset{}}
	receiverChannel.latestAuthorizedCloseAgreement.Details.Balance = Amount{Asset: NativeAsset{}}

	ca, err := sendingChannel.ProposePayment(Amount{
		Asset:  NativeAsset{},
		Amount: 200,
	})
	require.NoError(t, err)

	ca, authorized, err := receiverChannel.ConfirmPayment(ca)
	assert.False(t, authorized)
	require.NoError(t, err)
	assert.Equal(t, ca, receiverChannel.latestUnauthorizedCloseAgreement)

	// Confirming a close agreement with same sequence number but different Amount should error
	caDifferent := CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber: 1,
			Balance: Amount{
				Asset:  txnbuild.NativeAsset{},
				Amount: 400,
			},
		},
		CloseSignatures: ca.CloseSignatures,
	}
	_, authorized, err = receiverChannel.ConfirmPayment(caDifferent)
	assert.False(t, authorized)
	require.EqualError(t, err, "close agreement does not match the close agreement already in progress")
	assert.Equal(t, CloseAgreement{Details: CloseAgreementDetails{Balance: Amount{Asset: NativeAsset{}}}}, receiverChannel.LatestCloseAgreement())

	// Confirming a payment with same sequence number and same amount should pass
	ca, authorized, err = sendingChannel.ConfirmPayment(ca)
	assert.True(t, authorized)
	require.NoError(t, err)
	assert.Equal(t, CloseAgreement{}, sendingChannel.latestUnauthorizedCloseAgreement)

	ca, authorized, err = receiverChannel.ConfirmPayment(ca)
	assert.True(t, authorized)
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
