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
			ConfirmingSigner:           remoteSigner.FromAddress(),
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
