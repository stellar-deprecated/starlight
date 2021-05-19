package main

import (
	"fmt"
	"time"

	"github.com/stellar/experimental-payment-channels/examples/02-using-go-sdk/pctx"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
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

	// Tx history
	c := []Tx{}
	d := []Tx{}

	// Initial variable state
	s := initiator.EscrowSequenceNumber() + 1
	i := 0
	e := 0
	fmt.Println("s:", s, "i:", i, "e:", e)

	// Build F
	f, err := pctx.BuildFormationTx(initiator.Address(), responder.Address(), initiator.EscrowAddress(), responder.EscrowAddress(), s, i)
	iferrpanic(err)

	// Exchange signed C_i, D_i
	i++
	fmt.Println("s:", s, "i:", i, "e:", e)
	{
		ci, err := pctx.BuildCloseTx(initiator.Address(), responder.Address(), initiator.EscrowAddress(), responder.EscrowAddress(), s, i, "100.0", "200.0")
		iferrpanic(err)
		ci, err = ci.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		c = append(c, Tx{ci})
		di, err := pctx.BuildDeclarationTx(initiator.EscrowAddress(), s, i, e)
		iferrpanic(err)
		di, err = di.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		d = append(d, Tx{di})
	}

	// Sign and submit F
	f, err = f.Sign(networkPassphrase, initiator.Key(), responder.Key())
	iferrpanic(err)
	_, err = client.SubmitTransaction(f)
	iferrpanic(err)

	fmt.Println("d:", d)
	fmt.Println("c:", c)

	// Exchange signed C_i, D_i
	i++
	fmt.Println("s:", s, "i:", i, "e:", e)
	{
		ci, err := pctx.BuildCloseTx(initiator.Address(), responder.Address(), initiator.EscrowAddress(), responder.EscrowAddress(), s, i, "100.0", "200.0")
		iferrpanic(err)
		ci, err = ci.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		c = append(c, Tx{ci})
		di, err := pctx.BuildDeclarationTx(initiator.EscrowAddress(), s, i, e)
		iferrpanic(err)
		di, err = di.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		d = append(d, Tx{di})
	}

	fmt.Println("d:", d)
	fmt.Println("c:", c)

	// Submit latest D_i
	fmt.Println("Submitting:", d[len(d)-1])
	_, err = client.SubmitTransaction(d[len(d)-1].Transaction)
	iferrpanic(err)
	fmt.Println("Submitted:", d[len(d)-1])
	// Continue trying to submit C_i
	for {
		fmt.Println("Submitting:", c[len(d)-1])
		_, err = client.SubmitTransaction(c[len(c)-1].Transaction)
		if err == nil {
			fmt.Println("Success")
			break
		}
		fmt.Printf("Error: %#v", err.(*horizonclient.Error).Problem)
		time.Sleep(time.Second * 10)
	}
}

type Tx struct {
	*txnbuild.Transaction
}

func (tx Tx) String() string {
	return fmt.Sprintf("%d", tx.ToXDR().SeqNum())
}
