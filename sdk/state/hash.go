package state

import (
	"encoding/hex"
	"fmt"
)

type TransactionHash [32]byte

func (h TransactionHash) String() string {
	return hex.EncodeToString(h[:])
}

func (h TransactionHash) MarshalText() ([]byte, error) {
	text := [len(h) * 2]byte{}
	n := hex.Encode(text[:], h[:])
	if n != len(text) {
		return nil, hex.ErrLength
	}
	return text[:], nil
}

func (h *TransactionHash) UnmarshalText(text []byte) error {
	textDecodedLen := hex.DecodedLen(len(text))
	if textDecodedLen != len(h) {
		return fmt.Errorf("unmarshaling transaction hash: input decoded length %d expected length %d", textDecodedLen, len(h))
	}
	n, err := hex.Decode(h[:], text)
	if err != nil {
		return fmt.Errorf("unmarshaling transaction hash: %w", err)
	}
	if n != len(h) {
		return fmt.Errorf("unmarshaling transaction hash: decoded length %d expected length %d", n, len(h))
	}
	return nil
}
