package txbuild

import (
	"encoding/base64"
	"math"
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclaration_size(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom()

	closeTxHash := [32]byte{}
	tx, err := Declaration(DeclarationParams{
		InitiatorChannelAccount: initiatorChannelAccount.FromAddress(),
		StartSequence:           101,
		IterationNumber:         1,
		CloseTxHash:             closeTxHash,
		ConfirmingSigner:        initiatorSigner.FromAddress(),
	})
	require.NoError(t, err)

	// Test the size without signers.
	{
		txb, err := tx.MarshalBinary()
		require.NoError(t, err)
		t.Log("unsigned:", base64.StdEncoding.EncodeToString(txb))
		assert.Len(t, txb, 212)
	}

	// Test the size with signers.
	{
		tx, err := tx.Sign("test", initiatorSigner, responderSigner)
		require.NoError(t, err)
		signedPayloadSig, err := responderSigner.SignPayloadDecorated(closeTxHash[:])
		require.NoError(t, err)
		tx, err = tx.AddSignatureDecorated(signedPayloadSig)
		require.NoError(t, err)
		txb, err := tx.MarshalBinary()
		require.NoError(t, err)
		t.Log("signed:", base64.StdEncoding.EncodeToString(txb))
		assert.Len(t, txb, 428)
	}
}

func TestDeclaration_iterationNumber_checkNonNegative(t *testing.T) {
	_, err := Declaration(DeclarationParams{
		StartSequence:   101,
		IterationNumber: -1,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
	_, err = Declaration(DeclarationParams{
		StartSequence:   -1,
		IterationNumber: 5,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
}

func TestDeclaration_startSequenceOfIteration_checkNonNegative(t *testing.T) {
	_, err := Declaration(DeclarationParams{
		IterationNumber: 1,
		StartSequence:   math.MaxInt64,
	})
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
}
