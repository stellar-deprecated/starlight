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
	if hex.DecodedLen(len(text)) != len(h) {
		return hex.ErrLength
	}
	n, err := hex.Decode(h[:], text)
	if err != nil {
		return fmt.Errorf("unmarshalling hash: %w", err)
	}
	if n != len(h) {
		return hex.ErrLength
	}
	return nil
}
