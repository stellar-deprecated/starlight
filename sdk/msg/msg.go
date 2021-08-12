package msg

import (
	"encoding/json"
	"io"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/keypair"
)

type Type int

const (
	TypeHello           Type = 100
	TypeOpenRequest     Type = 200
	TypeOpenResponse    Type = 201
	TypePaymentRequest  Type = 300
	TypePaymentResponse Type = 301
	TypeCloseRequest    Type = 400
	TypeCloseResponse   Type = 401
)

type Message struct {
	Type Type

	Hello *Hello `json:",omitempty"`

	OpenRequest  *state.OpenAgreement `json:",omitempty"`
	OpenResponse *state.OpenAgreement `json:",omitempty"`

	PaymentRequest  *state.CloseAgreement `json:",omitempty"`
	PaymentResponse *state.CloseAgreement `json:",omitempty"`

	CloseRequest  *state.CloseAgreement `json:",omitempty"`
	CloseResponse *state.CloseAgreement `json:",omitempty"`
}

type Hello struct {
	EscrowAccount keypair.FromAddress
	Signer        keypair.FromAddress
}

type Encoder = json.Encoder

func NewEncoder(w io.Writer) *Encoder {
	return json.NewEncoder(w)
}

type Decoder = json.Decoder

func NewDecoder(r io.Reader) *Decoder {
	return json.NewDecoder(r)
}