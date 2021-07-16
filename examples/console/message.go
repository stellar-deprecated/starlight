package main

import "github.com/stellar/experimental-payment-channels/sdk/state"

type message struct {
	Introduction *introduction
	Open         *state.OpenAgreement
	Update       *state.CloseAgreement
	Close        *state.CloseAgreement
}

type introduction struct {
	EscrowAccount string
	Signer        string
}
