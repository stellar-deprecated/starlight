package state

import (
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stretchr/testify/require"
)

func TestProposeOpen_validAsset(t *testing.T) {
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

	_, err := sendingChannel.ProposeOpen(OpenParams{
		Asset: NativeAsset,
	})
	require.NoError(t, err)

	_, err = sendingChannel.ProposeOpen(OpenParams{
		Asset: ":GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P",
	})
	require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset code length must be between 1 and 12 characters`)

	_, err = sendingChannel.ProposeOpen(OpenParams{
		Asset: "ABCD:GABCD:AB",
	})
	require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset issuer: GABCD:AB is not a valid stellar public key`)

	_, err = sendingChannel.ProposeOpen(OpenParams{
		Asset: "ABCD:GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P",
	})
	require.NoError(t, err)
}

func TestConfirmOpen_rejectsDifferentOpenAgreements(t *testing.T) {
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
		},
	}

	oa := OpenAgreementDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      NativeAsset,
	}

	{
		// invalid ObservationPeriodTime
		d := oa
		d.ObservationPeriodTime = 0
		_, authorized, err := channel.ConfirmOpen(OpenAgreement{Details: d})
		require.False(t, authorized)
		require.EqualError(t, err, "input open agreement details do not match the saved open agreement details")
	}

	{
		// invalid different asset
		d := oa
		d.Asset = "ABC:GCDFU7RNY6HTYQKP7PYHBMXXKXZ4HET6LMJ5CDO7YL5NMYH4T2BSZCPZ"
		_, authorized, err := channel.ConfirmOpen(OpenAgreement{Details: d})
		require.False(t, authorized)
		require.EqualError(t, err, "input open agreement details do not match the saved open agreement details")
	}
}
