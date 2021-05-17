package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/examples/02-using-go-sdk/pctx"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
)

const networkPassphrase = "Standalone Network ; February 2017"

var root = func() *keypair.Full {
	kp, err := keypair.FromRawSeed(network.ID(networkPassphrase))
	if err != nil {
		panic(err)
	}
	return kp
}()

const horizonURL = "http://localhost:8000"

var client = &horizonclient.Client{HorizonURL: horizonURL}

func iferrpanic(err error) {
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
}

func main() {
	var err error

	// Setup initiator and responder.
	initiator, err := NewParticipant("Initiator")
	iferrpanic(err)
	responder, err := NewParticipant("Responder")
	iferrpanic(err)

	// Setup initiator escrow account.
	err = initiator.SetupEscrowAccount()
	iferrpanic(err)

	// Setup responder escrow account.
	err = responder.SetupEscrowAccount()
	iferrpanic(err)

	s := initiator.EscrowSequenceNumber() + 1
	fmt.Println("s:", s)
	i := 0
	fmt.Println("i:", i)
	// e := 0

	f, err := pctx.BuildFormationTx(
		initiator.Address(),
		responder.Address(),
		initiator.EscrowAddress(),
		responder.EscrowAddress(),
		s,
		i,
	)
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
	f, err = f.Sign(networkPassphrase, initiator.Key(), responder.Key())
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
	_, err = client.SubmitTransaction(f)
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
}
