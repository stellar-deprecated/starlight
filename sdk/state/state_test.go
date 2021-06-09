package state_test

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const horizonURL = "http://localhost:8000"

var networkPassphrase string
var client *horizonclient.Client

type Participant struct {
	Name                 string
	KP                   *keypair.Full
	Escrow               *keypair.Full
	EscrowSequenceNumber int64
	Contribution         int64
}

// Setup
func TestMain(m *testing.M) {
	client = &horizonclient.Client{HorizonURL: horizonURL}
	networkDetails, err := client.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	networkPassphrase = networkDetails.NetworkPassphrase

	os.Exit(m.Run())
}

func initAccounts(t *testing.T) (initiator Participant, responder Participant) {
	initiator = Participant{
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
	responder = Participant{
		Name:         "Responder",
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
	return initiator, responder
}

func initChannels(t *testing.T, initiator Participant, responder Participant) (initiatorChannel *state.Channel, responderChannel *state.Channel) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

	initiatorChannel = state.NewChannel(state.Config{
		NetworkPassphrase:          networkPassphrase,
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Initiator:                  true,
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: initiator.Contribution},
			},
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: responder.Contribution},
			},
		},
		LocalSigner:  initiator.KP,
		RemoteSigner: responder.KP.FromAddress(),
	})
	responderChannel = state.NewChannel(state.Config{
		NetworkPassphrase:          networkPassphrase,
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Initiator:                  false,
		LocalEscrowAccount: &state.EscrowAccount{
			Address:        responder.Escrow.FromAddress(),
			SequenceNumber: responder.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: responder.Contribution},
			},
		},
		RemoteEscrowAccount: &state.EscrowAccount{
			Address:        initiator.Escrow.FromAddress(),
			SequenceNumber: initiator.EscrowSequenceNumber,
			Balances: []state.Amount{
				{Asset: state.NativeAsset{}, Amount: initiator.Contribution},
			},
		},
		LocalSigner:  responder.KP,
		RemoteSigner: initiator.KP.FromAddress(),
	})
	return initiatorChannel, responderChannel
}

func TestOpenUpdatesUncoordinatedClose(t *testing.T) {
	initiator, responder := initAccounts(t)
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	// Tx history.
	closeTxs := []*txnbuild.Transaction{}
	declarationTxs := []*txnbuild.Transaction{}

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	open, err := initiatorChannel.ProposeOpen()
	require.NoError(t, err)
	for {
		var fullySignedR bool
		open, fullySignedR, err = responderChannel.ConfirmOpen(open)
		if err != nil {
			t.Fatal(err)
		}
		var fullySignedI bool
		open, fullySignedI, err = initiatorChannel.ConfirmOpen(open)
		if err != nil {
			t.Fatal(err)
		}
		if fullySignedI && fullySignedR {
			break
		}
	}

	{
		ci, di, fi, err := initiatorChannel.OpenTxs()
		require.NoError(t, err)

		ci, err = ci.AddSignatureDecorated(open.CloseSignatures...)
		require.NoError(t, err)
		closeTxs = append(closeTxs, ci)

		di, err = di.AddSignatureDecorated(open.DeclarationSignatures...)
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)

		fi, err = fi.AddSignatureDecorated(open.FormationSignatures...)
		require.NoError(t, err)

		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      fi,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
	}

	t.Log("Iteration", i, "Declarations:", txSeqs(declarationTxs))
	t.Log("Iteration", i, "Closes:", txSeqs(closeTxs))

	// Perform a number of iterations, much like two participants may.
	// Exchange signed C_i and D_i for each
	t.Log("Subsequent agreements...")
	rBalanceCheck := responder.Contribution
	iBalanceCheck := initiator.Contribution
	endingIterationNumber := int64(20)
	for i < endingIterationNumber {
		i++
		require.Equal(t, i, initiatorChannel.NextIterationNumber())
		require.Equal(t, i, responderChannel.NextIterationNumber())
		// get a random payment amount from 0 to 100 lumens
		amount := randomPositiveInt64(t, 100_0000000)

		var sendingChannel *state.Channel
		var receivingChannel *state.Channel
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
		t.Log("Current channel balances: I: ", sendingChannel.Balance().Amount/1_000_0000, "R: ", receivingChannel.Balance().Amount/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, sends to other party
		payment, err := sendingChannel.ProposePayment(state.Amount{Asset: state.NativeAsset{}, Amount: amount})
		require.NoError(t, err)

		ci, di, err := sendingChannel.PaymentTxs(payment)
		require.NoError(t, err)

		var fullySigned bool

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, fullySigned, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.False(t, fullySigned)

		// Sender: re-confirms P_i by signing D_i and sending back
		payment, fullySigned, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, fullySigned)

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, fullySigned, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, fullySigned)

		ci, err = ci.AddSignatureDecorated(payment.CloseSignatures...)
		require.NoError(t, err)
		closeTxs = append(closeTxs, ci)
		di, err = di.AddSignatureDecorated(payment.DeclarationSignatures...)
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)

		t.Log("Iteration", i, "Declarations:", txSeqs(declarationTxs))
		t.Log("Iteration", i, "Closes:", txSeqs(closeTxs))
	}

	// Confused participant attempts to close channel at old iteration.
	t.Log("Confused participant (responder) closing channel at old iteration...")
	{
		oldIteration := len(declarationTxs) - 4
		oldD := declarationTxs[oldIteration]
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      oldD,
			FeeAccount: responder.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		t.Log("Responder - Submitting Declaration:", oldD.SourceAccount().Sequence)
		require.NoError(t, err)
		go func() {
			oldC := closeTxs[oldIteration]
			for {
				fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
					Inner:      oldC,
					FeeAccount: responder.KP.Address(),
					BaseFee:    txnbuild.MinBaseFee,
				})
				require.NoError(t, err)
				fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
				require.NoError(t, err)
				_, err = client.SubmitFeeBumpTransaction(fbtx)
				if err == nil {
					t.Log("Responder - Submitting:", oldC.SourceAccount().Sequence, "Success")
					break
				}
				t.Log("Responder - Submitting:", oldC.SourceAccount().Sequence, "Error:", err)
				time.Sleep(time.Second * 5)
			}
		}()
	}

	done := make(chan struct{})

	// Good participant closes channel at latest iteration.
	t.Log("Good participant (initiator) closing channel at latest iteration...")
	{
		lastD, lastC, err := initiatorChannel.CloseTxs()
		require.NoError(t, err)
		lastD, err = lastD.AddSignatureDecorated(initiatorChannel.LatestCloseAgreement().DeclarationSignatures...)
		require.NoError(t, err)
		lastC, err = lastC.AddSignatureDecorated(initiatorChannel.LatestCloseAgreement().CloseSignatures...)
		require.NoError(t, err)

		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      lastD,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		t.Log("Initiator - Submitting Declaration:", lastD.SourceAccount().Sequence)
		require.NoError(t, err)
		go func() {
			defer close(done)
			for {
				fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
					Inner:      lastC,
					FeeAccount: initiator.KP.Address(),
					BaseFee:    txnbuild.MinBaseFee,
				})
				require.NoError(t, err)
				fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
				require.NoError(t, err)
				lastCHash, err := lastC.HashHex(networkPassphrase)
				require.NoError(t, err)
				_, err = client.SubmitFeeBumpTransaction(fbtx)
				if err == nil {
					t.Log("Initiator - Submitting Close:", lastCHash, lastC.SourceAccount().Sequence, "Success")
					break
				}
				hErr := horizonclient.GetError(err)
				t.Log("Initiator - Submitting Close:", lastCHash, lastC.SourceAccount().Sequence, "Error:", err)
				t.Log(hErr.ResultString())
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

func TestOpenUpdatesCoordinatedClose(t *testing.T) {
	initiator, responder := initAccounts(t)
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	open, err := initiatorChannel.ProposeOpen()
	require.NoError(t, err)
	for {
		var fullySignedR bool
		open, fullySignedR, err = responderChannel.ConfirmOpen(open)
		if err != nil {
			t.Fatal(err)
		}
		var fullySignedI bool
		open, fullySignedI, err = initiatorChannel.ConfirmOpen(open)
		if err != nil {
			t.Fatal(err)
		}
		if fullySignedI && fullySignedR {
			break
		}
	}

	{
		ci, di, fi, err := initiatorChannel.OpenTxs()
		require.NoError(t, err)

		ci, err = ci.AddSignatureDecorated(open.CloseSignatures...)
		require.NoError(t, err)

		di, err = di.AddSignatureDecorated(open.DeclarationSignatures...)
		require.NoError(t, err)

		fi, err = fi.AddSignatureDecorated(open.FormationSignatures...)
		require.NoError(t, err)

		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      fi,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
	}

	// Perform a number of iterations, much like two participants may.
	// Exchange signed C_i and D_i for each
	t.Log("Subsequent agreements...")
	rBalanceCheck := responder.Contribution
	iBalanceCheck := initiator.Contribution
	endingIterationNumber := int64(20)
	for i < endingIterationNumber {
		i++
		require.Equal(t, i, initiatorChannel.NextIterationNumber())
		require.Equal(t, i, responderChannel.NextIterationNumber())
		// get a random payment amount from 0 to 100 lumens
		amount := randomPositiveInt64(t, 100_0000000)

		paymentLog := ""
		var sendingChannel, receivingChannel *state.Channel
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
		t.Log("Current channel balances: I: ", sendingChannel.Balance().Amount/1_000_0000, "R: ", receivingChannel.Balance().Amount/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, sends to other party
		payment, err := sendingChannel.ProposePayment(state.Amount{Asset: state.NativeAsset{}, Amount: amount})
		require.NoError(t, err)

		var fullySigned bool

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, fullySigned, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.False(t, fullySigned)

		// Sender: re-confirms P_i by signing D_i and sending back
		payment, fullySigned, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, fullySigned)

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, fullySigned, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, fullySigned)
		ci, di, err := sendingChannel.PaymentTxs(payment)
		require.NoError(t, err)
		ci, err = ci.AddSignatureDecorated(payment.CloseSignatures...)
		require.NoError(t, err)
		di, err = di.AddSignatureDecorated(payment.DeclarationSignatures...)
		require.NoError(t, err)
	}

	// Coordinated Close
	t.Log("Begin coordinated close process ...")
	t.Log("Initiator submitting latest declaration transaction")
	lastD, _, err := initiatorChannel.CloseTxs()
	require.NoError(t, err)
	lastD, err = lastD.AddSignatureDecorated(initiatorChannel.LatestCloseAgreement().DeclarationSignatures...)
	require.NoError(t, err)

	fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      lastD,
		FeeAccount: initiator.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
	require.NoError(t, err)
	_, err = client.SubmitFeeBumpTransaction(fbtx)
	require.NoError(t, err)

	t.Log("Initiator proposes a coordinated close")
	cc, err := initiatorChannel.ProposeCoordinatedClose(0, 0)
	require.NoError(t, err)
	cc, err = responderChannel.ConfirmCoordinatedClose(cc)
	require.NoError(t, err)
	cc, err = initiatorChannel.ConfirmCoordinatedClose(cc)
	require.NoError(t, err)

	t.Log("Initiator closing channel with new coordinated close transaction")
	txCoordinated, err := initiatorChannel.CoordinatedCloseTx()
	require.NoError(t, err)
	txCoordinated, err = txCoordinated.AddSignatureDecorated(initiatorChannel.CoordinatedClose().CloseSignatures()...)
	fbtx, err = txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      txCoordinated,
		FeeAccount: initiator.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
	require.NoError(t, err)
	_, err = client.SubmitFeeBumpTransaction(fbtx)
	if err != nil {
		hErr := horizonclient.GetError(err)
		t.Log("Submitting Close:", txCoordinated.SourceAccount().Sequence, "Error:", err)
		t.Log(hErr.ResultString())
		require.NoError(t, err)
	}
	t.Log("Coordinated close successful")

	// check final escrow fund amounts are correct
	accountRequest := horizonclient.AccountRequest{AccountID: responder.Escrow.Address()}
	responderEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.EqualValues(t, fmt.Sprintf("%.7f", float64(rBalanceCheck)/float64(1_000_0000)), responderEscrowResponse.Balances[0].Balance)

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.EqualValues(t, fmt.Sprintf("%.7f", float64(iBalanceCheck)/float64(1_000_0000)), initiatorEscrowResponse.Balances[0].Balance)
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

func retry(maxAttempts int, f func() error) (err error) {
	for i := 0; i < maxAttempts; i++ {
		err = f()
		if err == nil {
			return
		}
	}
	return err
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

func txSeqs(txs []*txnbuild.Transaction) []int64 {
	seqs := make([]int64, len(txs))
	for i := range txs {
		seqs[i] = txs[i].SequenceNumber()
	}
	return seqs
}
