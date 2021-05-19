package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/examples/02-using-go-sdk/pctx"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
)

type Participant struct {
	name                 string
	kp                   *keypair.Full
	escrowAddress        *keypair.FromAddress
	escrowSequenceNumber int64
}

func NewParticipant(name string) (*Participant, error) {
	kp := keypair.MustRandom()
	fmt.Println(name+":", kp.Address())
	err := friendbot(kp.FromAddress(), "10000.0")
	if err != nil {
		return nil, err
	}
	p := &Participant{
		name: name,
		kp:   kp,
	}
	return p, err
}

func (p *Participant) Name() string {
	return p.name
}

func (p *Participant) Address() *keypair.FromAddress {
	return p.kp.FromAddress()
}

func (p *Participant) Key() *keypair.Full {
	return p.kp
}

func (p *Participant) EscrowAddress() *keypair.FromAddress {
	return p.escrowAddress
}

func (p *Participant) EscrowSequenceNumber() int64 {
	return p.escrowSequenceNumber
}

func (p *Participant) SetupEscrowAccount(initialContribution int64) error {
	ea := keypair.MustRandom()
	fmt.Println(p.name+" escrow account:", ea.Address())
	err := pctx.SetupEscrowAccount(networkPassphrase, client, p.kp, ea, initialContribution)
	if err != nil {
		return err
	}
	account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: ea.Address()})
	if err != nil {
		return err
	}
	seqNum, err := account.GetSequenceNumber()
	if err != nil {
		return err
	}
	p.escrowAddress = ea.FromAddress()
	p.escrowSequenceNumber = seqNum
	fmt.Println(p.name+" escrow account created.", "Sequence number:", seqNum)
	return nil
}
