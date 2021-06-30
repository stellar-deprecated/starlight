package integrationtests

import (
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
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
	Contribution         int64 // The contribution of the asset that will be used for payments
}

func TestOpenUpdatesUncoordinatedClose(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

	asset := txnbuild.NativeAsset{}
	// native asset has no asset limit
	assetLimit := int64(0)
	rootResp, err := client.Root()
	require.NoError(t, err)
	distributor := keypair.Master(rootResp.NetworkPassphrase).(*keypair.Full)
	initiator, responder := initAccounts(t, []AssetParam{
		AssetParam{
			Asset:       asset,
			AssetLimit:  assetLimit,
			Distributor: distributor,
		}})
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
	// I signs txClose
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		ObservationPeriodTime:      observationPeriodTime,
		ObservationPeriodLedgerGap: observationPeriodLedgerGap,
		Assets: []state.Trustline{
			state.Trustline{Asset: asset, AssetLimit: assetLimit},
		},
	})
	require.NoError(t, err)
	assert.Len(t, open.CloseSignatures, 1)
	assert.Len(t, open.DeclarationSignatures, 0)
	assert.Len(t, open.FormationSignatures, 0)
	{
		var authorizedR bool
		// R signs txClose and txDecl
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedR)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 1)
		assert.Len(t, open.FormationSignatures, 0)

		var authorizedI bool
		// I signs txDecl and F
		open, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedI)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 1)

		// R signs F, R is done
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedR)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 2)

		// I receives the last signatures for F, I is done
		open, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedI)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 2)
	}

	{
		ci, di, fi, err := initiatorChannel.OpenTxs(initiatorChannel.OpenAgreement().Details)
		require.NoError(t, err)

		ci, err = ci.AddSignatureDecorated(initiatorChannel.OpenAgreement().CloseSignatures...)
		require.NoError(t, err)
		closeTxs = append(closeTxs, ci)

		di, err = di.AddSignatureDecorated(initiatorChannel.OpenAgreement().DeclarationSignatures...)
		require.NoError(t, err)
		declarationTxs = append(declarationTxs, di)

		fi, err = fi.AddSignatureDecorated(initiatorChannel.OpenAgreement().FormationSignatures...)
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

		var authorized bool

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, authorized, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.False(t, authorized)

		// Sender: re-confirms P_i by signing D_i and sending back
		payment, authorized, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, authorized)

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, authorized, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, authorized)

		// Record the close tx's at this point in time.
		di, ci, err := sendingChannel.CloseTxs(sendingChannel.LatestCloseAgreement().Details)
		require.NoError(t, err)
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
		lastD, lastC, err := initiatorChannel.CloseTxs(initiatorChannel.LatestCloseAgreement().Details)
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
	assert.Equal(t, responderEscrowResponse.Balances[0].Balance, amount.StringFromInt64(rBalanceCheck))

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, initiatorEscrowResponse.Balances[0].Balance, amount.StringFromInt64(iBalanceCheck))
}

func TestOpenUpdatesCoordinatedClose(t *testing.T) {
	// Channel constants.
	const observationPeriodTime = 20 * time.Second
	const averageLedgerDuration = 5 * time.Second
	const observationPeriodLedgerGap = int64(observationPeriodTime / averageLedgerDuration)

	asset, distributor := initAsset(t, client, "ABDC")
	assetLimit := int64(5_000_0000000)
	initiator, responder := initAccounts(t, []AssetParam{
		AssetParam{
			Asset:       asset,
			AssetLimit:  assetLimit,
			Distributor: distributor,
		}})
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
		Assets: []state.Trustline{
			state.Trustline{Asset: asset, AssetLimit: assetLimit},
		},
	})
	require.NoError(t, err)
	assert.Len(t, open.CloseSignatures, 1)
	assert.Len(t, open.DeclarationSignatures, 0)
	assert.Len(t, open.FormationSignatures, 0)
	{
		var authorizedR bool
		// R signs txClose and txDecl
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedR)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 1)
		assert.Len(t, open.FormationSignatures, 0)

		var authorizedI bool
		// I signs txDecl and F
		open, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedI)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 1)

		// R signs F, R is done
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedR)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 2)

		// I receives the last signatures for F, I is done
		open, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedI)
		assert.Len(t, open.CloseSignatures, 2)
		assert.Len(t, open.DeclarationSignatures, 2)
		assert.Len(t, open.FormationSignatures, 2)
	}

	{
		ci, di, fi, err := initiatorChannel.OpenTxs(initiatorChannel.OpenAgreement().Details)
		require.NoError(t, err)

		_, err = ci.AddSignatureDecorated(initiatorChannel.OpenAgreement().CloseSignatures...)
		require.NoError(t, err)

		_, err = di.AddSignatureDecorated(initiatorChannel.OpenAgreement().DeclarationSignatures...)
		require.NoError(t, err)

		fi, err = fi.AddSignatureDecorated(initiatorChannel.OpenAgreement().FormationSignatures...)
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
		payment, err := sendingChannel.ProposePayment(state.Amount{Asset: asset, Amount: amount})
		require.NoError(t, err)

		var authorized bool

		// Receiver: receives new payment, validates, then confirms by signing both
		payment, authorized, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.False(t, authorized)

		// Sender: re-confirms P_i by signing D_i and sending back
		payment, authorized, err = sendingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, authorized)

		// Receiver: receives new payment, validates, then confirms by signing both
		_, authorized, err = receivingChannel.ConfirmPayment(payment)
		require.NoError(t, err)
		require.True(t, authorized)
	}

	// Coordinated Close
	t.Log("Begin coordinated close process ...")
	t.Log("Initiator submitting latest declaration transaction")
	lastD, _, err := initiatorChannel.CloseTxs(initiatorChannel.LatestCloseAgreement().Details)
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
	ca, err := initiatorChannel.ProposeCoordinatedClose()
	require.NoError(t, err)

	ca, authorized, err := responderChannel.ConfirmCoordinatedClose(ca)
	require.NoError(t, err)
	require.True(t, authorized)

	_, authorized, err = initiatorChannel.ConfirmCoordinatedClose(ca)
	require.NoError(t, err)
	require.True(t, authorized)

	t.Log("Initiator closing channel with new coordinated close transaction")
	_, txCoordinated, err := initiatorChannel.CloseTxs(initiatorChannel.LatestCloseAgreement().Details)
	require.NoError(t, err)
	txCoordinated, err = txCoordinated.AddSignatureDecorated(initiatorChannel.LatestCloseAgreement().CloseSignatures...)
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

func TestOpen_multipleAssets(t *testing.T) {
	asset1, distributor := initAsset(t, client, "code1")
	assetLimit1 := int64(5_000_0000000)
	ap1 := AssetParam{
		Asset:       asset1,
		Distributor: distributor,
		AssetLimit:  assetLimit1,
	}

	asset2, distributor := initAsset(t, client, "code2")
	assetLimit2 := int64(10_000_0000000)
	ap2 := AssetParam{
		Asset:       asset2,
		Distributor: distributor,
		AssetLimit:  assetLimit2,
	}

	initiator, responder := initAccounts(t, []AssetParam{ap1, ap2})
	initiatorChannel, responderChannel := initChannels(t, initiator, responder)

	s := initiator.EscrowSequenceNumber + 1
	i := int64(1)
	e := int64(0)
	t.Log("Vars: s:", s, "i:", i, "e:", e)

	// Open
	t.Log("Open...")
	// I signs txClose
	open, err := initiatorChannel.ProposeOpen(state.OpenParams{
		Assets: []state.Trustline{
			state.Trustline{Asset: asset1, AssetLimit: assetLimit1},
			state.Trustline{Asset: asset2, AssetLimit: assetLimit2},
		},
	})
	require.NoError(t, err)
	{
		var authorizedR bool
		// R signs txClose and txDecl
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedR)

		var authorizedI bool
		// I signs txDecl and F
		open, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.False(t, authorizedI)

		// R signs F, R is done
		open, authorizedR, err = responderChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedR)

		// I receives the last signatures for F, I is done
		_, authorizedI, err = initiatorChannel.ConfirmOpen(open)
		require.NoError(t, err)
		require.True(t, authorizedI)
	}

	{
		ci, di, fi, err := initiatorChannel.OpenTxs(initiatorChannel.OpenAgreement().Details)
		require.NoError(t, err)

		_, err = ci.AddSignatureDecorated(initiatorChannel.OpenAgreement().CloseSignatures...)
		require.NoError(t, err)

		_, err = di.AddSignatureDecorated(initiatorChannel.OpenAgreement().DeclarationSignatures...)
		require.NoError(t, err)

		fi, err = fi.AddSignatureDecorated(initiatorChannel.OpenAgreement().FormationSignatures...)
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

	// check the balances are correct for both escrow accounts
	accountRequest := horizonclient.AccountRequest{AccountID: responder.Escrow.Address()}
	responderEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, amount.StringFromInt64(1_000_0000000), responderEscrowResponse.Balances[0].Balance)
	assert.Equal(t, "code1", responderEscrowResponse.Balances[0].Asset.Code)
	assert.Equal(t, amount.StringFromInt64(assetLimit1), responderEscrowResponse.Balances[0].Limit)
	assert.Equal(t, amount.StringFromInt64(1_000_0000000), responderEscrowResponse.Balances[1].Balance)
	assert.Equal(t, amount.StringFromInt64(assetLimit2), responderEscrowResponse.Balances[1].Limit)
	assert.Equal(t, "code2", responderEscrowResponse.Balances[1].Asset.Code)

	accountRequest = horizonclient.AccountRequest{AccountID: initiator.Escrow.Address()}
	initiatorEscrowResponse, err := client.AccountDetail(accountRequest)
	require.NoError(t, err)
	assert.Equal(t, amount.StringFromInt64(1_000_0000000), initiatorEscrowResponse.Balances[0].Balance)
	assert.Equal(t, "code1", initiatorEscrowResponse.Balances[0].Asset.Code)
	assert.Equal(t, amount.StringFromInt64(assetLimit1), initiatorEscrowResponse.Balances[0].Limit)
	assert.Equal(t, amount.StringFromInt64(1_000_0000000), initiatorEscrowResponse.Balances[1].Balance)
	assert.Equal(t, amount.StringFromInt64(assetLimit2), initiatorEscrowResponse.Balances[1].Limit)
	assert.Equal(t, "code2", initiatorEscrowResponse.Balances[1].Asset.Code)
}
