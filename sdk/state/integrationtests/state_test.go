package integrationtests

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Participant struct {
	Name                 string
	KP                   *keypair.Full
	Escrow               *keypair.Full
	EscrowSequenceNumber int64
	Contribution         int64 // The contribution of the asset that will be used for payments
}

func TestOpenUpdatesUncoordinatedClose(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)
	const formationExpiry = 5 * time.Minute

	asset := state.NativeAsset
	// native asset has no asset limit
	rootResp, err := client.Root()
	require.NoError(t, err)
	distributor := keypair.Master(rootResp.NetworkPassphrase).(*keypair.Full)
	initiator, responder := initAccounts(t, AssetParam{
		Asset:       asset,
		Distributor: distributor,
	})
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
	// I signs
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(formationExpiry),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, open.ProposerSignatures.Declaration)
	assert.NotEmpty(t, open.ProposerSignatures.Close)
	assert.NotEmpty(t, open.ProposerSignatures.Formation)
	assert.Empty(t, open.ConfirmerSignatures)
	{
		// R signs, R is done
		open, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)

		// I receives the signatures, I is done
		open, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)
	}

	{
		di, ci, err := initiatorChannel.CloseTxs()
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)
		closeTxs = append(closeTxs, ci)

		fi, err := initiatorChannel.OpenTx()
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

	// Update balances known for each other.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(responder.Contribution)
	responderChannel.UpdateRemoteEscrowAccountBalance(initiator.Contribution)

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
		t.Log("Current channel balances: I: ", sendingChannel.Balance()/1_000_0000, "R: ", receivingChannel.Balance()/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, signs, sends to other party
		payment, err := sendingChannel.ProposePayment(amount)
		require.NoError(t, err)

		// Receiver: receives new payment, validates, then confirms by signing
		payment, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)

		// Sender: stores receiver's signatures
		_, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)

		// Record the close tx's at this point in time.
		di, ci, err := sendingChannel.CloseTxs()
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)
		closeTxs = append(closeTxs, ci)

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
	assert.Equal(t, responderEscrowResponse.Balances[0].Balance, amount.StringFromInt64(rBalanceCheck))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, initiatorEscrowResponse.Balances[0].Balance, amount.StringFromInt64(iBalanceCheck))
}

func TestOpenUpdatesCoordinatedCloseStartCloseThenCoordinate(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)
	const formationExpiry = 5 * time.Minute

	asset, distributor := initAsset(t, client, "ABDC")
	initiator, responder := initAccounts(t, AssetParam{
		Asset:       asset,
		Distributor: distributor,
	})
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	// I signs
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(formationExpiry),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, open.ProposerSignatures.Declaration)
	assert.NotEmpty(t, open.ProposerSignatures.Close)
	assert.NotEmpty(t, open.ProposerSignatures.Formation)
	assert.Empty(t, open.ConfirmerSignatures)
	{
		// R signs, R is done
		open, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)

		// I stores the signatures, I is done.
		open, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)
	}

	{
		fi, err := initiatorChannel.OpenTx()
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

	// Update balances known for each other.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(responder.Contribution)
	responderChannel.UpdateRemoteEscrowAccountBalance(initiator.Contribution)

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
		t.Log("Current channel balances: I: ", sendingChannel.Balance()/1_000_0000, "R: ", receivingChannel.Balance()/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, signs, sends to other party
		payment, err := sendingChannel.ProposePayment(amount)
		require.NoError(t, err)

		// Receiver: receives new payment, validates, then confirms by signing
		payment, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)

		// Sender: stores the receivers signatures
		_, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
	}

	// Coordinated Close
	t.Log("Begin coordinated close process ...")
	t.Log("Initiator submitting latest declaration transaction")
	lastD, _, err := initiatorChannel.CloseTxs()
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
	ca, err := initiatorChannel.ProposeClose()
	require.NoError(t, err)

	ca, err = responderChannel.ConfirmClose(ca)
	require.NoError(t, err)

	_, err = initiatorChannel.ConfirmClose(ca)
	require.NoError(t, err)

	t.Log("Initiator closing channel with new coordinated close transaction")
	_, txCoordinated, err := initiatorChannel.CloseTxs()
	require.NoError(t, err)
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
	assert.Equal(t, amount.StringFromInt64(rBalanceCheck), assetBalance(asset, responderEscrowResponse))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, amount.StringFromInt64(iBalanceCheck), assetBalance(asset, initiatorEscrowResponse))
}

func TestOpenUpdatesCoordinatedCloseCoordinateThenStartClose(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)
	const formationExpiry = 5 * time.Minute

	asset, distributor := initAsset(t, client, "ABDC")
	initiator, responder := initAccounts(t, AssetParam{
		Asset:       asset,
		Distributor: distributor,
	})
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	// I signs
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(formationExpiry),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, open.ProposerSignatures.Declaration)
	assert.NotEmpty(t, open.ProposerSignatures.Close)
	assert.NotEmpty(t, open.ProposerSignatures.Formation)
	assert.Empty(t, open.ConfirmerSignatures)

	{
		// R signs txClose and txDecl
		open, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)

		// I receives the signatures, I is done
		open, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)
	}

	{
		fi, err := initiatorChannel.OpenTx()
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

	// Update balances known for each other.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(responder.Contribution)
	responderChannel.UpdateRemoteEscrowAccountBalance(initiator.Contribution)

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
		t.Log("Current channel balances: I: ", sendingChannel.Balance()/1_000_0000, "R: ", receivingChannel.Balance()/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, signs, sends to other party
		payment, err := sendingChannel.ProposePayment(amount)
		require.NoError(t, err)

		// Receiver: receives new payment, validates, then confirms by signing
		payment, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)

		// Sender: stores the signatures from receiver
		_, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
	}

	// Coordinated Close
	t.Log("Begin coordinated close process ...")

	t.Log("Initiator proposes a coordinated close")
	ca, err := initiatorChannel.ProposeClose()
	require.NoError(t, err)

	ca, err = responderChannel.ConfirmClose(ca)
	require.NoError(t, err)

	_, err = initiatorChannel.ConfirmClose(ca)
	require.NoError(t, err)

	t.Log("Initiator submitting latest declaration transaction")
	lastD, _, err := initiatorChannel.CloseTxs()
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

	t.Log("Initiator closing channel with new coordinated close transaction")
	_, txCoordinated, err := initiatorChannel.CloseTxs()
	require.NoError(t, err)
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
	assert.Equal(t, amount.StringFromInt64(rBalanceCheck), assetBalance(asset, responderEscrowResponse))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, amount.StringFromInt64(iBalanceCheck), assetBalance(asset, initiatorEscrowResponse))
}

func TestOpenUpdatesCoordinatedCloseCoordinateThenStartCloseByRemote(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)
	const formationExpiry = 5 * time.Minute

	asset, distributor := initAsset(t, client, "ABDC")
	initiator, responder := initAccounts(t, AssetParam{
		Asset:       asset,
		Distributor: distributor,
	})
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	// I signs txClose
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(formationExpiry),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, open.ProposerSignatures.Declaration)
	assert.NotEmpty(t, open.ProposerSignatures.Close)
	assert.NotEmpty(t, open.ProposerSignatures.Formation)
	assert.Empty(t, open.ConfirmerSignatures)
	{
		// R signs
		open, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)

		// I receives the signatures, I is done
		open, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		assert.NotEmpty(t, open.ProposerSignatures.Declaration)
		assert.NotEmpty(t, open.ProposerSignatures.Close)
		assert.NotEmpty(t, open.ProposerSignatures.Formation)
		assert.NotEmpty(t, open.ConfirmerSignatures.Declaration)
		assert.NotEmpty(t, open.ConfirmerSignatures.Close)
		assert.NotEmpty(t, open.ConfirmerSignatures.Formation)
	}

	{
		fi, err := initiatorChannel.OpenTx()
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

	// Update balances known for each other.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(responder.Contribution)
	responderChannel.UpdateRemoteEscrowAccountBalance(initiator.Contribution)

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
		t.Log("Current channel balances: I: ", sendingChannel.Balance()/1_000_0000, "R: ", receivingChannel.Balance()/1_000_0000)
		t.Log("Current channel iteration numbers: I: ", sendingChannel.NextIterationNumber(), "R: ", receivingChannel.NextIterationNumber())
		t.Log("Proposal: ", i, paymentLog, amount/1_000_0000)

		// Sender: creates new Payment, signs, sends to other party
		payment, err := sendingChannel.ProposePayment(amount)
		require.NoError(t, err)

		// Receiver: receives new payment, validates, then confirms by signing
		payment, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)

		// Sender: stores signatures from receiver
		_, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
	}

	// Coordinated Close
	t.Log("Begin coordinated close process ...")

	t.Log("Initiator proposes a coordinated close")
	ca, err := initiatorChannel.ProposeClose()
	require.NoError(t, err)

	ca, err = responderChannel.ConfirmClose(ca)
	require.NoError(t, err)

	_, err = initiatorChannel.ConfirmClose(ca)
	require.NoError(t, err)

	t.Log("Responder submitting latest declaration transaction")
	lastD, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)

	fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      lastD,
		FeeAccount: responder.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
	require.NoError(t, err)
	_, err = client.SubmitFeeBumpTransaction(fbtx)
	require.NoError(t, err)

	t.Log("Responder closing channel with new coordinated close transaction")
	_, txCoordinated, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	fbtx, err = txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      txCoordinated,
		FeeAccount: responder.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
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
	assert.Equal(t, amount.StringFromInt64(rBalanceCheck), assetBalance(asset, responderEscrowResponse))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, amount.StringFromInt64(iBalanceCheck), assetBalance(asset, initiatorEscrowResponse))
}

func TestOpenUpdatesUncoordinatedClose_recieverNotReturningSigs(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)
	const formationExpiry = 5 * time.Minute

	asset := state.NativeAsset
	// native asset has no asset limit
	rootResp, err := client.Root()
	require.NoError(t, err)
	distributor := keypair.Master(rootResp.NetworkPassphrase).(*keypair.Full)
	initiator, responder := initAccounts(t, AssetParam{
		Asset:       asset,
		Distributor: distributor,
	})
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	{
		open, err := initiatorChannel.ProposeOpen(state.OpenParams{
			ObservationPeriodTime:      observationPeriodTime,
			ObservationPeriodLedgerGap: observationPeriodLedgerGap,
			Asset:                      asset,
			ExpiresAt:                  time.Now().Add(formationExpiry),
		})
		require.NoError(t, err)
		open, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		tx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		require.NoError(t, err)
	}

	// Update balances known for each other.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(responder.Contribution)
	responderChannel.UpdateRemoteEscrowAccountBalance(initiator.Contribution)

	// Perform a transaction.
	{
		payment, err := initiatorChannel.ProposePayment(8)
		require.NoError(t, err)
		payment, err = responderChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(payment)
		require.NoError(t, err)
	}

	// Perform another transaction.
	{
		payment, err := initiatorChannel.ProposePayment(2)
		require.NoError(t, err)
		_, err = responderChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		// Pretend the responder doesn't pass back their response to the
		// initiator. The initiator never sees their signatures, so the
		// initiator never calls confirm payment with the responders signature.
		// The responder has a last authorized agreement with total balance
		// owing as 10. The initiator has a last authorized agreement with total
		// balance owing as 8, and an unauthorized agreement with total balance
		// as 10.
		assert.Equal(t, initiatorChannel.LatestCloseAgreement().Details.Balance, int64(8))
		assert.Equal(t, responderChannel.LatestCloseAgreement().Details.Balance, int64(10))
	}

	// Responder starts but doesn't finish closing the channel.
	var broadcastedTx *txnbuild.Transaction // << Pretend this broadcasted tx is how the initiator finds out about the tx.
	{
		t.Log("Responder starts but doesn't complete an uncoordinated close...")
		t.Log("Responder submits the declaration transaction for the agreement that the initiator does not have all the signatures...")

		{
			declTx, _, err := responderChannel.CloseTxs()
			require.NoError(t, err)
			declHash, _ := declTx.HashHex(networkPassphrase)
			t.Log("Responder tries to submit the declaration without disclosing signatures of the close tx:", declHash)
			declTxXDR := declTx.ToXDR()
			// Assume the declTx has 3 signatures, 2 to authorize the
			// declaration, 1 that is a the close tx's signature disclosed.
			assert.Len(t, declTxXDR.V1.Signatures, 3)
			// Truncate the declTx signatures so the disclosure of the close tx
			// does not occur.
			declTxXDR.V1.Signatures = declTxXDR.V1.Signatures[:2]
			declTxModified64, err := xdr.MarshalBase64(declTxXDR)
			require.NoError(t, err)
			declTxModifiedEnv, err := txnbuild.TransactionFromXDR(declTxModified64)
			require.NoError(t, err)
			declTxModified, ok := declTxModifiedEnv.Transaction()
			require.True(t, ok)
			// Submit the modified declaration transaction and confirm.
			fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
				Inner:      declTxModified,
				FeeAccount: responder.KP.Address(),
				BaseFee:    txnbuild.MinBaseFee,
			})
			require.NoError(t, err)
			fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
			require.NoError(t, err)
			_, err = client.SubmitFeeBumpTransaction(fbtx)
			require.Error(t, err)
			resultsStr, err := horizonclient.GetError(err).ResultString()
			require.NoError(t, err)
			results := xdr.TransactionResult{}
			_, err = xdr.Unmarshal(base64.NewDecoder(base64.StdEncoding, strings.NewReader(resultsStr)), &results)
			require.NoError(t, err)
			require.Equal(t, xdr.TransactionResultCodeTxBadAuth, results.Result.InnerResultPair.Result.Result.Code)
			t.Log("Responder failed to submit tx, result:", results.Result.InnerResultPair.Result.Result.Code)
		}

		{
			declTx, closeTx, err := responderChannel.CloseTxs()
			require.NoError(t, err)
			declHash, _ := declTx.HashHex(networkPassphrase)
			t.Log("Responder tries to submit the declaration with all signatures:", declHash)
			t.Log("Responder submits the declaration:", declHash)
			fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
				Inner:      declTx,
				FeeAccount: responder.KP.Address(),
				BaseFee:    txnbuild.MinBaseFee,
			})
			require.NoError(t, err)
			fbtx, err = fbtx.Sign(networkPassphrase, responder.KP)
			require.NoError(t, err)
			_, err = client.SubmitFeeBumpTransaction(fbtx)
			require.NoError(t, err)
			t.Log("Responder succeeds in submitting declaration")
			closeHash, _ := closeTx.HashHex(networkPassphrase)
			t.Log("Responder does not submit the close:", closeHash)
			broadcastedTx = declTx
		}
	}

	// Initiator must find the signatures for the close tx on network to complete.
	{
		t.Log("Initiator sees the declaration and goes looking for the close signatures...")
		// To prevent xdr parsing error.
		placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
		validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
		err = initiatorChannel.IngestTx(broadcastedTx, validResultXDR, placeholderXDR)
		require.NoError(t, err)

		t.Log("Initiator found signature:", base64.StdEncoding.EncodeToString(initiatorChannel.LatestCloseAgreement().ConfirmerSignatures.Close))

		t.Log("Initiator waits the observation period...")
		time.Sleep(observationPeriodTime)

		t.Log("Initiator submits close")
		_, closeTx, err := initiatorChannel.CloseTxs()
		require.NoError(t, err)
		fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
			Inner:      closeTx,
			FeeAccount: initiator.KP.Address(),
			BaseFee:    txnbuild.MinBaseFee,
		})
		require.NoError(t, err)
		fbtx, err = fbtx.Sign(networkPassphrase, initiator.KP)
		require.NoError(t, err)
		_, err = client.SubmitFeeBumpTransaction(fbtx)
		if !assert.NoError(t, err) {
			t.Log(horizonclient.GetError(err).ResultString())
		}
		t.Log("Closed")
	}

	// Check the final state of the escrow accounts.
	{
		// Initiator should be down 10 (0.0000010).
		initiatorEscrowResponse, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()})
		require.NoError(t, err)
		assert.Equal(t, "999.9999990", assetBalance(asset, initiatorEscrowResponse))

		// Responder should be up 10 (0.0000010).
		responderEscrowResponse, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: responder.Escrow.Address()})
		require.NoError(t, err)
		assert.Equal(t, "1000.0000010", assetBalance(asset, responderEscrowResponse))
	}
}
