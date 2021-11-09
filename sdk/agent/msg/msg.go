// Package msg contains simple types to assist with transmitting and
// communicating across a network about a payment channel between two
// participants. It is rather rudimentary and intended for use in examples.
// There are no stability or compatibility guarantees for the messages or
// encoding format used.
package msg

import (
	"encoding/gob"
	"io"

	"github.com/stellar/go/keypair"
	"github.com/stellar/starlight/sdk/state"
)

// Type is the message type, used to indicate which message is contained inside
// a Message.
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

// Message is a message that can be transmitted to support two participants in a
// payment channel communicating by signaling who they are with a hello, opening
// the channel, making payments, and closing the channel.
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

// Hello can be used to signal to another participant a minimal amount of
// information the other participant needs to know about them.
type Hello struct {
	MultisigAccount keypair.FromAddress
	Signer          keypair.FromAddress
}

// Encoder is an encoder that can be used to encode messages.
// It is currently set as the encoding/gob.Encoder, but may be changed to
// another type at anytime to facilitate testing or to improve performance.
type Encoder = gob.Encoder

func NewEncoder(w io.Writer) *Encoder {
	return gob.NewEncoder(w)
}

type Decoder = gob.Decoder

func NewDecoder(r io.Reader) *Decoder {
	return gob.NewDecoder(r)
}
