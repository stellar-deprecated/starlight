package main

import "github.com/stellar/experimental-payment-channels/sdk/state"

type message struct {
	Hello  *hello
	Open   *state.OpenAgreement
	Update *state.CloseAgreement
	Close  *state.CloseAgreement
}

type hello struct {
	EscrowAccount string
	Signer        string
}
