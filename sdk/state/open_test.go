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
	_, err := sendingChannel.ProposeOpen(OpenParams{Asset: native})
	require.NoError(t, err)

	invalidCredit := CreditAsset{}
	_, err = sendingChannel.ProposeOpen(OpenParams{Asset: invalidCredit, AssetLimit: 100})
	require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset code length must be between 1 and 12 characters`)

	validCredit := CreditAsset{Code: "ABCD", Issuer: "GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P"}
	_, err = sendingChannel.ProposeOpen(OpenParams{Asset: validCredit, AssetLimit: 100})
	require.NoError(t, err)
}

func TestProposeOpen_multipleAssets(t *testing.T) {
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
		Asset:      txnbuild.CreditAsset{Code: "ABC", Issuer: "abcIssuer"},
		AssetLimit: 200,
	}
	p = OpenParams{
		Assets: []Trustline{ca1, ca2},
	}
	oa, err := channel.ProposeOpen(p)
	require.NoError(t, err)

	wantDetails := OpenAgreementDetails{
		Assets: []Trustline{ca1, ca2},
	}
	assert.Equal(t, wantDetails, oa.Details)
	assert.Len(t, 1, oa.CloseSignatures)

}
