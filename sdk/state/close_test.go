package state

import (
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/require"
)

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

	ca := CloseAgreement{
		CloseSignatures: []xdr.DecoratedSignature{
			{Signature: randomByteArray(t, 10)},
			{Signature: randomByteArray(t, 10)},
		},
		DeclarationSignatures: []xdr.DecoratedSignature{
			{Signature: randomByteArray(t, 10)},
			{Signature: randomByteArray(t, 10)},
			{Signature: randomByteArray(t, 10)},
		},
	}

	err := channel.validateClose(ca)
	require.EqualError(t, err, "close agreement has too many signatures, has declaration: 3, close: 2, max of 2 allowed for each")

	// Should pass check with 2 signatures each
	ca.DeclarationSignatures = ca.DeclarationSignatures[1:]
	err = channel.validateClose(ca)
	require.NoError(t, err)
}
