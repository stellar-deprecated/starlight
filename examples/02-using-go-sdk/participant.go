package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
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

func (p *Participant) CreateEscrow(initialContribution int64) error {
	escrow := keypair.MustRandom()
	fmt.Println(p.name+" escrow account:", escrow.Address())

	account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: p.kp.Address()})
	if err != nil {
		return err
	}
	seqNum, err := account.GetSequenceNumber()
	if err != nil {
		return err
	}
	tx, err := txbuild.CreateEscrow(p.kp, escrow, seqNum, initialContribution)
	if err != nil {
		return err
	}

	tx, err = tx.Sign(networkPassphrase, p.kp, escrow)
	if err != nil {
		return err
	}

	txResp, err := client.SubmitTransaction(tx)
	if err != nil {
		return err
	}

	p.escrowAddress = escrow.FromAddress()
	p.escrowSequenceNumber = int64(txResp.Ledger)<<32
	fmt.Println(p.name+" escrow account created.", "Sequence number:", seqNum)
	return nil
}
