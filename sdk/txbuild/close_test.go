package txbuild

import (
	"encoding/base64"
	"math"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClose_size(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom()
	responderChannelAccount := keypair.MustRandom()

	tx, err := Close(CloseParams{
		ObservationPeriodTime:      5 * time.Minute,
		ObservationPeriodLedgerGap: 5,
		InitiatorSigner:            initiatorSigner.FromAddress(),
		ResponderSigner:            responderSigner.FromAddress(),
		InitiatorChannelAccount:    initiatorChannelAccount.FromAddress(),
		ResponderChannelAccount:    responderChannelAccount.FromAddress(),
		StartSequence:              101,
		IterationNumber:            1,
		AmountToInitiator:          100,
		AmountToResponder:          200,
		Asset:                      txnbuild.CreditAsset{Code: "ETH", Issuer: "GBTYEE5BTST64JCBUXVAEEPQJAY3TNV47A5JFUMQKNDWUJRRT6LUVEQH"},
	})
	require.NoError(t, err)

	// Test the size without signers.
	{
		txb, err := tx.MarshalBinary()
		require.NoError(t, err)
		t.Log("unsigned:", base64.StdEncoding.EncodeToString(txb))
		assert.Len(t, txb, 652)
	}

	// Test the size with signers.
	{
		tx, err := tx.Sign("test", initiatorSigner, responderSigner)
		require.NoError(t, err)
		txb, err := tx.MarshalBinary()
		require.NoError(t, err)
		t.Log("signed:", base64.StdEncoding.EncodeToString(txb))
		assert.Len(t, txb, 796)
	}
}

func TestClose_iterationNumber_checkNonNegative(t *testing.T) {
	_, err := Close(CloseParams{
		StartSequence:   101,
		IterationNumber: -1,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
	_, err = Close(CloseParams{
		StartSequence:   -1,
		IterationNumber: 5,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
}

func TestClose_startSequenceOfIteration_checkNonNegative(t *testing.T) {
	_, err := Close(CloseParams{
		IterationNumber: 0,
		StartSequence:   math.MaxInt64,
	})
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
}
