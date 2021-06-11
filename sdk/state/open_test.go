package state

import (
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stretchr/testify/require"
)

func TestProposePayment_valid_asset(t *testing.T) {
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
	_, err := sendingChannel.ProposeOpen(native, "")
	require.NoError(t, err)

	invalidCredit := CreditAsset{}
	_, err = sendingChannel.ProposeOpen(invalidCredit, "")
	require.Error(t, err)

	validCredit := CreditAsset{Code: "ABCD", Issuer: "GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P"}
	_, err = sendingChannel.ProposeOpen(validCredit, "")
	require.Error(t, err)
	require.Equal(t, "proposing asset limit for non native asset: strconv.Atoi: parsing \"\": invalid syntax", err.Error())

	validCredit = CreditAsset{Code: "ABCD", Issuer: "GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P"}
	_, err = sendingChannel.ProposeOpen(validCredit, "100")
	require.NoError(t, err)
}
