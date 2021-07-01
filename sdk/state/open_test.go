package state

import (
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
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

	native := NativeAsset{}
	_, err := sendingChannel.ProposeOpen(OpenParams{
		Trustlines: []Trustline{Trustline{
			Asset: native},
		},
	})
	require.NoError(t, err)

	invalidCredit := CreditAsset{}
	_, err = sendingChannel.ProposeOpen(OpenParams{
		Trustlines: []Trustline{
			Trustline{Asset: invalidCredit, AssetLimit: 100},
		},
	})
	require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset code length must be between 1 and 12 characters`)

	validCredit := CreditAsset{Code: "ABCD", Issuer: "GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P"}
	_, err = sendingChannel.ProposeOpen(OpenParams{
		Trustlines: []Trustline{
			Trustline{Asset: validCredit, AssetLimit: 100},
		},
	})
	require.NoError(t, err)
}

func TestProposeOpen_multipleAssets(t *testing.T) {
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
			Trustlines: []Trustline{
				Trustline{Asset: NativeAsset{}},
			},
		},
	}

	// invalid open params
	p := OpenParams{}
	_, err := channel.ProposeOpen(p)
	assert.EqualError(t, err, `invalid open params: trying to open a channel with no assets`)

	// valid open params
	ca1 := Trustline{
		Asset:      txnbuild.NativeAsset{},
		AssetLimit: 100,
	}
	ca2 := Trustline{
		Asset:      txnbuild.CreditAsset{Code: "ABC", Issuer: "GB3A5VJGUIXQ4X35NZQMVLO5DKSOTUFF6SLHLY7BWJLJYVQVCJM4IEAI"},
		AssetLimit: 200,
	}
	p = OpenParams{
		Trustlines: []Trustline{ca1, ca2},
	}
	oa, err := channel.ProposeOpen(p)
	require.NoError(t, err)

	wantDetails := OpenAgreementDetails{
		Trustlines: []Trustline{ca1, ca2},
	}
	assert.Equal(t, wantDetails, oa.Details)
	assert.Len(t, oa.CloseSignatures, 1)
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
			Trustlines: []Trustline{
				Trustline{Asset: NativeAsset{}},
			},
		},
	}

	oa := OpenAgreementDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Trustlines: []Trustline{
			Trustline{Asset: NativeAsset{}},
		},
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
		// invalid new asset
		d := oa
		d.Trustlines = append(d.Trustlines, Trustline{Asset: CreditAsset{Code: "abc"}})
		_, authorized, err := channel.ConfirmOpen(OpenAgreement{Details: d})
		require.False(t, authorized)
		require.EqualError(t, err, "input open agreement details do not match the saved open agreement details")
	}

}
