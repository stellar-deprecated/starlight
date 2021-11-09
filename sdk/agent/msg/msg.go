package msg

import (
	"encoding/gob"
	"io"

	"github.com/stellar/go/keypair"
	"github.com/stellar/starlight/sdk/state"
)

type Type int

const (
	TypeHello           Type = 10
	TypeOpenRequest     Type = 20
	TypeOpenResponse    Type = 21
	TypePaymentRequest  Type = 30
	TypePaymentResponse Type = 31
	TypeCloseRequest    Type = 40
	TypeCloseResponse   Type = 41
)

type Message struct {
	Type Type

	Hello *Hello

	OpenRequest  *state.OpenEnvelope
	OpenResponse *state.OpenSignatures

	PaymentRequest  *state.CloseEnvelope
	PaymentResponse *state.CloseSignatures

	CloseRequest  *state.CloseEnvelope
	CloseResponse *state.CloseSignatures
}

type Hello struct {
	EscrowAccount keypair.FromAddress
	Signer        keypair.FromAddress
}

type Encoder = gob.Encoder

func NewEncoder(w io.Writer) *Encoder {
	return gob.NewEncoder(w)
}

type Decoder = gob.Decoder

func NewDecoder(r io.Reader) *Decoder {
	return gob.NewDecoder(r)
}
