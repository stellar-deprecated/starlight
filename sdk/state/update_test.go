package state

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate(t *testing.T) {
	// create I
	// create R
	// ... setup
	// I proposes new payment, P_1
	// R receives P_1
	// R confirms P_1
	// I re-confirms P_1

	const horizonURL = "http://localhost:8000"
	client := &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := client.Root()
	require.NoError(t, err)
	networkPassphrase := networkDetails.NetworkPassphrase

	// Channel constants.
	const observationPeriodTime = 1 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

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
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		txResp, err := client.SubmitFeeBumpTransaction(fbtx)
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
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: responder.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
		require.NoError(t, err)
		txResp, err := client.SubmitFeeBumpTransaction(fbtx)
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

	t.Log("Iteration", i, "Declarations:", len(declarationTxs))
	t.Log("Iteration", i, "Closes:", len(closeTxs))

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
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      f,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
	}

	// TODO - create NewChannel method - always sets Balance to 0 at first
	initiatorChannel := Channel{
		Status:                 "initialized",
		ProposalStatus:         "none",
		Initiator:              true,
		InitiatorEscrowAccount: initiator.Escrow.FromAddress(),
		ResponderEscrowAccount: responder.Escrow.FromAddress(),
		Balance:                0,
		Asset:                  txnbuild.NativeAsset{},
	}

	responderChannel := Channel{
		Status:                 "initialized",
		ProposalStatus:         "none",
		Initiator:              false,
		InitiatorEscrowAccount: initiator.Escrow.FromAddress(),
		ResponderEscrowAccount: responder.Escrow.FromAddress(),
		Balance:                0,
		Asset:                  txnbuild.NativeAsset{},
	}

	//// SETUP DONE

	//// NEW PROPOSALS
	paymentProposal := PaymentProposal{}
	for i < 7 {
		i++

		amount := randomPositiveInt64(t, 100_0000000)
		amountToResponder := int64(0)
		amountToInitiator := int64(0)
		paymentLog := ""
		if randomBool(t) {
			amountToResponder = amount
			paymentLog = "I payment to R of: "
		} else {
			amountToInitiator = amount
			paymentLog = "R payment to I of: "
		}
		t.Log("Proposal: ", i, paymentLog, amount)

		//// INITIATOR: creates new Payment, sends to R
		paymentProposal, err := initiatorChannel.NewPaymentProposal(initiator, responder, amountToInitiator, amountToResponder, s, i, e, 0, 0, networkPassphrase)
		require.NoError(t, err)
		j, err := json.Marshal(paymentProposal)
		require.NoError(t, err)

		//// RESPONDER: receives new payment proposal, validates, then confirms by signing both
		paymentProposal = &PaymentProposal{}
		err = json.Unmarshal(j, paymentProposal)
		require.NoError(t, err)
		paymentProposal, err = responderChannel.ConfirmPayment(paymentProposal, initiator, responder, networkPassphrase)
		require.NoError(t, err)
		j, err := json.Marshal(paymentProposal)
		require.NoError(t, err)

		//// INITIATOR: re-confirms P_i by signing D_i and sending back
		paymentProposal = PaymentProposal{}
		err = json.Unmarshal(j, &paymentProposal)
		paymentProposal, err = initiatorChannel.ConfirmPayment(paymentProposal, networkPassphrase)
		require.NoError(t, err)
	}

	// TODO - how does Initiator submit "latest", is it store in the Channel?
	//// INITIATOR: submits latest proposal
	txDGeneric, err := txnbuild.TransactionFromXDR(paymentProposal.DeclarationTxXDR)
	require.NoError(t, err)
	txD, isSimple := txDGeneric.Transaction()
	require.True(t, isSimple)
	fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      txD,
		FeeAccount: initiator.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
	require.NoError(t, err)
	_, err = client.SubmitFeeBumpTransaction(fbtx)
	if err != nil {
		t.Log("Error submitting fee bumpbed txD", err.(*horizonclient.Error).Problem.Extras["result_codes"])
		return
	}

	txCGeneric, err := txnbuild.TransactionFromXDR(paymentProposal.ClosingTxXDR)
	require.NoError(t, err)
	txC, isSimple := txCGeneric.Transaction()
	require.True(t, isSimple)

	fbtx, err = txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      txC,
		FeeAccount: initiator.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
	require.NoError(t, err)
	_, err = client.SubmitFeeBumpTransaction(fbtx)
	if err != nil {
		t.Log("Error submitting fee bumpbed txC", err.(*horizonclient.Error).Problem.Extras["result_codes"])
		return
	}

	assert.NoError(t, err)

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