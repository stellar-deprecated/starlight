package main

import (
	"crypto/rand"
	"encoding/binary"
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

	iContribution := int64(1000_0000000)
	rContribution := int64(1000_0000000)

	// Setup initiator escrow account.
	err = initiator.SetupEscrowAccount(iContribution)
	iferrpanic(err)

	// Setup responder escrow account.
	err = responder.SetupEscrowAccount(rContribution)
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
		ci, err := pctx.BuildCloseTx(initiator.Address(), responder.Address(), initiator.EscrowAddress(), responder.EscrowAddress(), s, i, 0, 0)
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

	// Positive I owes R, negative R owes I
	owing := int64(0)

	// Perform some number of iterations, exchange signed C_i and D_i for each
	for i < 20 {
		i++
		fmt.Println("s:", s, "i:", i, "e:", e)
		if randomBool() {
			amount := randomPositiveInt64(iContribution - owing)
			fmt.Println("i pays r", amount)
			owing += amount
		} else {
			amount := randomPositiveInt64(rContribution + owing)
			fmt.Println("r pays i", amount)
			owing -= amount
		}
		rOwesI := int64(0)
		iOwesR := int64(0)
		if owing > 0 {
			iOwesR = owing
		} else if owing < 0 {
			rOwesI = -owing
		}
		fmt.Println("i owes r", iOwesR)
		fmt.Println("r owes i", rOwesI)
		ci, err := pctx.BuildCloseTx(initiator.Address(), responder.Address(), initiator.EscrowAddress(), responder.EscrowAddress(), s, i, rOwesI, iOwesR)
		iferrpanic(err)
		ci, err = ci.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		c = append(c, Tx{ci})
		di, err := pctx.BuildDeclarationTx(initiator.EscrowAddress(), s, i, e)
		iferrpanic(err)
		di, err = di.Sign(networkPassphrase, initiator.Key(), responder.Key())
		iferrpanic(err)
		d = append(d, Tx{di})

		time.Sleep(2 * time.Second)
	}

	fmt.Println("d:", d)
	fmt.Println("c:", c)

	// Someone tries to submit an old D_i
	oldIteration := len(d) - 4
	oldD := d[oldIteration]
	fmt.Println("Submitting:", oldD)
	_, err = client.SubmitTransaction(oldD.Transaction)
	iferrpanic(err)
	fmt.Println("Submitted:", oldD)
	// Continue trying to submit C_i
	go func() {
		oldC := c[oldIteration]
		for {
			fmt.Println("Submitting:", oldC)
			_, err = client.SubmitTransaction(oldC.Transaction)
			if err == nil {
				fmt.Println("Submitting:", oldC, "Success")
				break
			}
			fmt.Println("Submitting:", oldC, "Error:", err.(*horizonclient.Error).Problem.Extras["result_codes"])
			time.Sleep(time.Second * 5)
		}
	}()

	// Submit latest D_i
	lastIteration := len(d)-1
	lastD := d[lastIteration]
	fmt.Println("Submitting:", lastD)
	_, err = client.SubmitTransaction(lastD.Transaction)
	iferrpanic(err)
	fmt.Println("Submitted:", lastD)
	// Continue trying to submit C_i
	lastC := c[lastIteration]
	for {
		fmt.Println("Submitting:", lastC)
		_, err = client.SubmitTransaction(lastC.Transaction)
		if err == nil {
			fmt.Println("Submitting:", lastC, "Success")
			break
		}
		fmt.Println("Submitting:", lastC, "Error:", err.(*horizonclient.Error).Problem.Extras["result_codes"])
		time.Sleep(time.Second * 10)
	}
}

type Tx struct {
	*txnbuild.Transaction
}

func (tx Tx) String() string {
	return fmt.Sprintf("%d", tx.ToXDR().SeqNum())
}

func randomBool() bool {
	b := [1]byte{}
	_, err := rand.Read(b[:])
	iferrpanic(err)
	return b[0]%2 == 0
}

func randomPositiveInt64(max int64) int64 {
	var i uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &i)
	iferrpanic(err)
	return int64(i) % max
}
