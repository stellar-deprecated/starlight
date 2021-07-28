package main

import (
	"encoding/json"
	"io"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/keypair"
)

type Type int

const (
	TypeHelloRequest    Type = 100
	TypeHelloResponse   Type = 101
	TypeOpenRequest     Type = 200
	TypeOpenResponse    Type = 201
	TypePaymentRequest  Type = 300
	TypePaymentResponse Type = 301
	TypeCloseRequest    Type = 400
	TypeCloseResponse   Type = 401
)

type Message struct {
	Type Type

	HelloRequest  *Hello
	HelloResponse *Hello

	OpenRequest  *state.OpenAgreement
	OpenResponse *state.OpenAgreement

	PaymentRequest  *state.CloseAgreement
	PaymentResponse *state.CloseAgreement

	CloseRequest  *state.CloseAgreement
	CloseResponse *state.CloseAgreement
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
