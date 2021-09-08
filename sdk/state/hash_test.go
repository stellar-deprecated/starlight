package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash_String(t *testing.T) {
	h := TransactionHash{}
	assert.Equal(
		t,
		"0000000000000000000000000000000000000000000000000000000000000000",
		h.String(),
	)
	h = TransactionHash{0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01}
	assert.Equal(
		t,
		"0123456789012345678901234567890101234567890123456789012345678901",
		h.String(),
	)
}

func TestHash_MarshalText(t *testing.T) {
	h := TransactionHash{}
	b, err := h.MarshalText()
	require.NoError(t, err)
	wantB := []byte("0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, wantB, b)

	h = TransactionHash{0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01}
	b, err = h.MarshalText()
	require.NoError(t, err)
	wantB = []byte("0123456789012345678901234567890101234567890123456789012345678901")
	assert.Equal(t, wantB, b)
}

func TestHash_UnmarshalText(t *testing.T) {
	// Zero.
	s := "0000000000000000000000000000000000000000000000000000000000000000"
	h := TransactionHash{}
	err := h.UnmarshalText([]byte(s))
	require.NoError(t, err)
	wantH := TransactionHash{}
	assert.Equal(t, wantH, h)

	// Valid.
	s = "0123456789012345678901234567890101234567890123456789012345678901"
	h = TransactionHash{}
	err = h.UnmarshalText([]byte(s))
	require.NoError(t, err)
	wantH = TransactionHash{0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45, 0x67, 0x89, 0x01}
	assert.Equal(t, wantH, h)

	// Invalid: too long by one character / a nibble.
	s = "01234567890123456789012345678901012345678901234567890123456789000"
	h = TransactionHash{}
	err = h.UnmarshalText([]byte(s))
	assert.EqualError(t, err, "unmarshaling transaction hash: input length 65 expected 64")

	// Invalid: too long by two characters / a byte.
	s = "012345678901234567890123456789010123456789012345678901234567890000"
	h = TransactionHash{}
	err = h.UnmarshalText([]byte(s))
	assert.EqualError(t, err, "unmarshaling transaction hash: input length 66 expected 64")

	// Invalid: too short by one character / a nibble.
	s = "012345678901234567890123456789010123456789012345678901234567890"
	h = TransactionHash{}
	err = h.UnmarshalText([]byte(s))
	assert.EqualError(t, err, "unmarshaling transaction hash: input length 63 expected 64")

	// Invalid: too short by two characters / a byte.
	s = "01234567890123456789012345678901012345678901234567890123456789"
	h = TransactionHash{}
	err = h.UnmarshalText([]byte(s))
	assert.EqualError(t, err, "unmarshaling transaction hash: input length 62 expected 64")
}
