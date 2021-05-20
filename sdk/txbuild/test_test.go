package txbuild_test

import (
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	const horizonURL = "http://localhost:8000"
	client := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := client.Root()
	require.NoError(t, err)
	networkPassphrase := networkDetails.NetworkPassphrase

	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

	// Common data both participants will have during the test.
	type Participant struct {
		Name                 string
		KP                   *keypair.Full
		Escrow               *keypair.Full
		EscrowSequenceNumber int64
		Contribution         int64
	}

	// Setup initiator.
	initiator := Participant{
		Name:         "Initiator",
		KP:           keypair.MustRandom(),
		Escrow:       keypair.MustRandom(),
		Contribution: 1_000_0000000,
	}
	t.Log("Initiator:", initiator.KP.Address())
	t.Log("Initiator Escrow:", initiator.Escrow.Address())
	{
		err := fund(client, initiator.KP.FromAddress(), 10_000_0000000)
		require.NoError(t, err)
		account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: initiator.KP.Address()})
		require.NoError(t, err)
		seqNum, err := account.GetSequenceNumber()
		require.NoError(t, err)
		tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
			Creator:             initiator.KP.FromAddress(),
			Escrow:              initiator.Escrow.FromAddress(),
			SequenceNumber:      seqNum + 1,
			InitialContribution: initiator.Contribution,
		})
		require.NoError(t, err)
		tx, err = tx.Sign(networkPassphrase, initiator.KP, initiator.Escrow)
		require.NoError(t, err)
		txResp, err := client.SubmitTransaction(tx)
		require.NoError(t, err)
		initiator.EscrowSequenceNumber = int64(txResp.Ledger) << 32
	}
	t.Log("Initiator Escrow Sequence Number:", initiator.EscrowSequenceNumber)
	t.Log("Initiator Contribution:", initiator.Contribution)

	// Setup responder.
	responder := Participant{
		Name:         "Initiator",
		KP:           keypair.MustRandom(),
		Escrow:       keypair.MustRandom(),
		Contribution: 1_000_0000000,
	}
	t.Log("Responder:", responder.KP.Address())
	t.Log("Responder Escrow:", responder.Escrow.Address())
	{
		err := fund(client, responder.KP.FromAddress(), 10_000_0000000)
		require.NoError(t, err)
		account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: responder.KP.Address()})
		require.NoError(t, err)
		seqNum, err := account.GetSequenceNumber()
		require.NoError(t, err)
		tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
			Creator:             responder.KP.FromAddress(),
			Escrow:              responder.Escrow.FromAddress(),
			SequenceNumber:      seqNum + 1,
			InitialContribution: responder.Contribution,
		})
		require.NoError(t, err)
		tx, err = tx.Sign(networkPassphrase, responder.KP, responder.Escrow)
		require.NoError(t, err)
		txResp, err := client.SubmitTransaction(tx)
		require.NoError(t, err)
		responder.EscrowSequenceNumber = int64(txResp.Ledger) << 32
	}
	t.Log("Responder Escrow Sequence Number:", responder.EscrowSequenceNumber)
	t.Log("Responder Contribution:", responder.Contribution)

	// Tx history.
	closeTxs := []*txnbuild.Transaction{}
	declarationTxs := []*txnbuild.Transaction{}

	// Set initial variable state.
	s := initiator.EscrowSequenceNumber + 1
	i := int64(0)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Exchange initial C_i and D_i.
	t.Log("Initial agreement...")
	i++
	t.Log("Vars: s:", s, "i:", i, "e:", e)
	{
		ci, err := txbuild.Close(txbuild.CloseParams{
			ObservationPeriodTime:      observationPeriodTime,
			ObservationPeriodLedgerGap: observationPeriodLedgerGap,
			InitiatorSigner:            initiator.KP.FromAddress(),
			ResponderSigner:            responder.KP.FromAddress(),
			InitiatorEscrow:            initiator.Escrow.FromAddress(),
			ResponderEscrow:            responder.Escrow.FromAddress(),
			StartSequence:              s,
			IterationNumber:            i,
			AmountToInitiator:          0,
			AmountToResponder:          0,
		})
		require.NoError(t, err)
		ci, err = ci.Sign(networkPassphrase, initiator.KP, responder.KP)
		require.NoError(t, err)
		closeTxs = append(closeTxs, ci)
		di, err := txbuild.Declaration(txbuild.DeclarationParams{
			InitiatorEscrow:         initiator.Escrow.FromAddress(),
			StartSequence:           s,
			IterationNumber:         i,
			IterationNumberExecuted: e,
		})
		require.NoError(t, err)
		di, err = di.Sign(networkPassphrase, initiator.KP, responder.KP)
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)
	}

	t.Log("Iteration", i, "Declarations:", declarationTxs)
	t.Log("Iteration", i, "Closes:", closeTxs)

	// Perform formation.
	t.Log("Formation...")
	{
		f, err := txbuild.Formation(txbuild.FormationParams{
			InitiatorSigner: initiator.KP.FromAddress(),
			ResponderSigner: responder.KP.FromAddress(),
			InitiatorEscrow: initiator.Escrow.FromAddress(),
			ResponderEscrow: responder.Escrow.FromAddress(),
			StartSequence:   s,
		})
		require.NoError(t, err)
		f, err = f.Sign(networkPassphrase, initiator.KP, responder.KP)
		require.NoError(t, err)
		_, err = client.SubmitTransaction(f)
		require.NoError(t, err)
	}

	// Owing tracks how much each participant owes the other particiant.
	// A positive amount = I owes R.
	// A negative amount = R owes I.
	owing := int64(0)

	// Perform a number of iterations, much like two participants may.
	// Exchange signed C_i and D_i for each
	t.Log("Subsequent agreements...")
	for i < 20 {
		i++
		t.Log("Vars: s:", s, "i:", i, "e:", e)
		if randomBool(t) {
			amount := randomPositiveInt64(t, initiator.Contribution-owing)
			t.Log("Iteration", i, "I pays R", amount)
			owing += amount
		} else {
			amount := randomPositiveInt64(t, responder.Contribution+owing)
			t.Log("Iteration", i, "R pays I", amount)
			owing -= amount
		}
		rOwesI := int64(0)
		iOwesR := int64(0)
		if owing > 0 {
			iOwesR = owing
		} else if owing < 0 {
			rOwesI = -owing
		}
		t.Log("Iteration", i, "I owes R", iOwesR)
		t.Log("Iteration", i, "R owes I", rOwesI)
		closeParams := txbuild.CloseParams{
			ObservationPeriodTime:      observationPeriodTime,
			ObservationPeriodLedgerGap: observationPeriodLedgerGap,
			InitiatorSigner:            initiator.KP.FromAddress(),
			ResponderSigner:            responder.KP.FromAddress(),
			InitiatorEscrow:            initiator.Escrow.FromAddress(),
			ResponderEscrow:            responder.Escrow.FromAddress(),
			StartSequence:              s,
			IterationNumber:            i,
			AmountToInitiator:          rOwesI,
			AmountToResponder:          iOwesR,
		}
		ci, err := txbuild.Close(closeParams)
		require.NoError(t, err)
		ci, err = ci.Sign(networkPassphrase, initiator.KP, responder.KP)
		require.NoError(t, err)
		closeTxs = append(closeTxs, ci)
		di, err := txbuild.Declaration(txbuild.DeclarationParams{
			InitiatorEscrow:         initiator.Escrow.FromAddress(),
			StartSequence:           s,
			IterationNumber:         i,
			IterationNumberExecuted: e,
		})
		require.NoError(t, err)
		di, err = di.Sign(networkPassphrase, initiator.KP, responder.KP)
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)

		t.Log("Iteration", i, "Declarations:", declarationTxs)
		t.Log("Iteration", i, "Closes:", closeTxs)
	}

	// Confused participant attempts to close channel at old iteration.
	t.Log("Confused participant closes channel at old iteration...")
	{
		oldIteration := len(declarationTxs) - 4
		oldD := declarationTxs[oldIteration]
		_, err := client.SubmitTransaction(oldD)
		t.Log("Submitting Declaration:", oldD.SourceAccount().Sequence)
		require.NoError(t, err)
		go func() {
			oldC := closeTxs[oldIteration]
			for {
				_, err = client.SubmitTransaction(oldC)
				if err == nil {
					t.Log("Submitting:", oldC.SourceAccount().Sequence, "Success")
					break
				}
				t.Log("Submitting:", oldC.SourceAccount().Sequence, "Error:", err.(*horizonclient.Error).Problem.Extras["result_codes"])
				time.Sleep(time.Second * 5)
			}
		}()
	}

	done := make(chan struct{})

	// Good participant closes channel at latest iteration.
	t.Log("Good participant closes channel at latest iteration...")
	{
		lastIteration := len(declarationTxs) - 1
		lastD := declarationTxs[lastIteration]
		_, err := client.SubmitTransaction(lastD)
		t.Log("Submitting Declaration:", lastD.SourceAccount().Sequence)
		require.NoError(t, err)
		go func() {
			defer close(done)
			lastC := closeTxs[lastIteration]
			for {
				_, err = client.SubmitTransaction(lastC)
				if err == nil {
					t.Log("Submitting Close:", lastC.SourceAccount().Sequence, "Success")
					break
				}
				t.Log("Submitting Close:", lastC.SourceAccount().Sequence, "Error:", err.(*horizonclient.Error).Problem.Extras["result_codes"])
				time.Sleep(time.Second * 10)
			}
		}()
	}

	select {
	case <-done:
		t.Log("Channel closed. Test complete.")
	case <-time.After(1 * time.Minute):
		t.Fatal("Channel close timed out after waiting 1 minute.")
	}
}

func randomBool(t *testing.T) bool {
	t.Helper()
	b := [1]byte{}
	_, err := rand.Read(b[:])
	require.NoError(t, err)
	return b[0]%2 == 0
}

func randomPositiveInt64(t *testing.T, max int64) int64 {
	t.Helper()
	var i uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &i)
	require.NoError(t, err)
	return int64(i) % max
}

func fund(client horizonclient.ClientInterface, account *keypair.FromAddress, startingBalance int64) error {
	rootResp, err := client.Root()
	if err != nil {
		return err
	}
	root := keypair.Master(rootResp.NetworkPassphrase).(*keypair.Full)
	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: root.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.CreateAccount{
					Destination: account.Address(),
					Amount:      amount.StringFromInt64(startingBalance),
				},
			},
		},
	)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(rootResp.NetworkPassphrase, root)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}
