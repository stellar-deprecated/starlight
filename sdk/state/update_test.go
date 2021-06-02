package state

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
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

type Participant struct {
	Name                 string
	KP                   *keypair.Full
	Escrow               *keypair.Full
	EscrowSequenceNumber int64
	Contribution         int64
}

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
		err := retry(2, func() error { return fund(client, initiator.KP.FromAddress(), 10_000_0000000) })
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
		err := retry(2, func() error { return fund(client, responder.KP.FromAddress(), 10_000_0000000) })
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

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase: networkPassphrase,
		// TODO - increase these params
		ObservationPeriodTime:      0,
		ObservationPeriodLedgerGap: 0,
		StartingSequence:           initiator.EscrowSequenceNumber,
		Initiator:                  true,
		LocalEscrowAccount: &EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances:       []Amount{},
		},
		RemoteEscrowAccount: &EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances:       []Amount{},
		},
		LocalSigner:  initiator.KP,
		RemoteSigner: responder.KP.FromAddress(),
	})

	responderChannel := NewChannel(Config{
		NetworkPassphrase:          networkPassphrase,
		ObservationPeriodTime:      0,
		ObservationPeriodLedgerGap: 0,
		StartingSequence:           initiator.EscrowSequenceNumber,
		Initiator:                  false,
		LocalEscrowAccount: &EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances:       []Amount{},
		},
		RemoteEscrowAccount: &EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances:       []Amount{},
		},
		LocalSigner:  responder.KP,
		RemoteSigner: initiator.KP.FromAddress(),
	})

	//// SETUP DONE

	//// NEW PROPOSALS
	payment := &Payment{}
	rBalanceCheck := responder.Contribution
	iBalanceCheck := initiator.Contribution
	for i < 7 {
		i++
		amount := randomPositiveInt64(t, 100_0000000)

		var sendingChannel *Channel
		var receivingChannel *Channel
		paymentLog := ""
		if randomBool(t) {
			paymentLog = "I payment to R of: "
			sendingChannel = initiatorChannel
			receivingChannel = responderChannel
			rBalanceCheck += amount
			iBalanceCheck -= amount
		} else {
			paymentLog = "R payment to I of: "
			sendingChannel = responderChannel
			receivingChannel = initiatorChannel
			rBalanceCheck -= amount
			iBalanceCheck += amount
		}
		t.Log("Current channel balances: I: ", initiatorChannel.amount.Amount/1_000_0000, "R: ", responderChannel.amount.Amount/1_000_0000)
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		//// Sender: creates new Payment, sends to other party
		sendingChannel.iterationNumber = i
		payment, err = sendingChannel.ProposePayment(amount)
		require.NoError(t, err)
		j, err := json.Marshal(payment)
		require.NoError(t, err)

		//// Receiver: receives new payment proposal, validates, then confirms by signing both
		receivingChannel.iterationNumber = i
		payment = &Payment{}
		err = json.Unmarshal(j, payment)
		require.NoError(t, err)

		payment, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		j, err = json.Marshal(payment)
		require.NoError(t, err)

		//// Sender: re-confirms P_i by signing D_i and sending back
		payment = &Payment{}
		err = json.Unmarshal(j, payment)
		payment, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
	}

	// TODO - how does Initiator submit "latest", is it store in the Channel?
	//// INITIATOR: closes channel by submitting latest proposal

	t.Log("Initiator Closing Channel at i: ", initiatorChannel.iterationNumber)
	t.Log("Final channel balances: I: ", initiatorChannel.amount.Amount/1_000_0000, "R: ", responderChannel.amount.Amount/1_000_0000)

	txD, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         initiatorChannel.localEscrowAccount.Address,
		StartSequence:           initiatorChannel.startingSequence,
		IterationNumber:         initiatorChannel.iterationNumber,
		IterationNumberExecuted: 0,
	})
	require.NoError(t, err)
	for _, sig := range payment.DeclarationSignatures {
		txD, err = txD.AddSignatureDecorated(sig)
		require.NoError(t, err)
	}

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
	}
	require.NoError(t, err)

	amountToInitiator := int64(0)
	amountToResponder := int64(0)
	if initiatorChannel.amount.Amount > 0 {
		amountToResponder = initiatorChannel.amount.Amount
	} else {
		amountToInitiator = initiatorChannel.amount.Amount * -1
	}
	txC, err := txbuild.Close(txbuild.CloseParams{
		ObservationPeriodTime:      initiatorChannel.observationPeriodTime,
		ObservationPeriodLedgerGap: initiatorChannel.observationPeriodLedgerGap,
		InitiatorSigner:            initiatorChannel.localSigner.FromAddress(),
		ResponderSigner:            initiatorChannel.remoteSigner,
		InitiatorEscrow:            initiatorChannel.localEscrowAccount.Address,
		ResponderEscrow:            initiatorChannel.remoteEscrowAccount.Address,
		StartSequence:              initiatorChannel.startingSequence,
		IterationNumber:            initiatorChannel.iterationNumber,
		AmountToInitiator:          amountToInitiator,
		AmountToResponder:          amountToResponder,
	})
	require.NoError(t, err)

	for _, sig := range payment.CloseSignatures {
		txC, err = txC.AddSignatureDecorated(sig)
		require.NoError(t, err)
	}

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
	}
	require.NoError(t, err)

	// check final escrow fund amounts are correct
	accountRequest := horizonclient.AccountRequest{AccountID: responder.Escrow.Address()}
	responderEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.EqualValues(t, responderEscrowResponse.Balances[0].Balance, fmt.Sprintf("%.7f", float64(rBalanceCheck)/float64(1_000_0000)))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.EqualValues(t, initiatorEscrowResponse.Balances[0].Balance, fmt.Sprintf("%.7f", float64(iBalanceCheck)/float64(1_000_0000)))
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

func retry(maxAttempts int, f func() error) (err error) {
	for i := 0; i < maxAttempts; i++ {
		err = f()
		if err == nil {
			return
		}
	}
	return err
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
