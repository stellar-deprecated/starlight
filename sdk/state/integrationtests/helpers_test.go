package integrationtests

import (
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	stellarAmount "github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/require"
)

// functions to be used in the state_test integration tests

type AssetParam struct {
	Asset       state.Asset
	Distributor *keypair.Full
}

func initAccounts(t *testing.T, assetParam AssetParam) (initiator Participant, responder Participant) {
	initiator = Participant{
		Name:         "Initiator",
		KP:           keypair.MustRandom(),
		Escrow:       keypair.MustRandom(),
		Contribution: 1_000_0000000,
	}
	t.Log("Initiator:", initiator.KP.Address())
	t.Log("Initiator Escrow:", initiator.Escrow.Address())
	{
		err := retry(2, func() error { return createAccount(initiator.KP.FromAddress(), 10_000_0000000) })
		require.NoError(t, err)
		err = retry(2, func() error {
			return fundAsset(assetParam.Asset, initiator.Contribution, initiator.KP, assetParam.Distributor)
		})
		require.NoError(t, err)

		t.Log("Initiator Contribution:", initiator.Contribution, "of asset:", assetParam.Asset.Code(), "issuer: ", assetParam.Asset.Issuer())
		initEscrowAccount(t, &initiator, assetParam)
	}
	t.Log("Initiator Escrow Sequence Number:", initiator.EscrowSequenceNumber)

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
		err := retry(2, func() error { return createAccount(responder.KP.FromAddress(), 10_000_0000000) })
		require.NoError(t, err)
		err = retry(2, func() error {
			return fundAsset(assetParam.Asset, responder.Contribution, responder.KP, assetParam.Distributor)
		})
		require.NoError(t, err)

		t.Log("Responder Contribution:", responder.Contribution, "of asset:", assetParam.Asset.Code(), "issuer: ", assetParam.Asset.Issuer())
		initEscrowAccount(t, &responder, assetParam)
	}
	t.Log("Responder Escrow Sequence Number:", responder.EscrowSequenceNumber)

	return initiator, responder
}

func initEscrowAccount(t *testing.T, participant *Participant, assetParam AssetParam) {
	// create escrow account
	account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: participant.KP.Address()})
	require.NoError(t, err)
	seqNum, err := account.GetSequenceNumber()
	require.NoError(t, err)

	tx, err := txbuild.CreateEscrow(txbuild.CreateEscrowParams{
		Creator:        participant.KP.FromAddress(),
		Escrow:         participant.Escrow.FromAddress(),
		SequenceNumber: seqNum + 1,
		Asset:          assetParam.Asset.Asset(),
	})
	require.NoError(t, err)
	tx, err = tx.Sign(networkPassphrase, participant.KP, participant.Escrow)
	require.NoError(t, err)
	fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: participant.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	require.NoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, participant.KP)
	require.NoError(t, err)
	txResp, err := client.SubmitFeeBumpTransaction(fbtx)
	require.NoError(t, err)
	participant.EscrowSequenceNumber = int64(txResp.Ledger) << 32

	// add initial contribution, use the same contribution for each asset
	_, err = account.IncrementSequenceNumber()
	require.NoError(t, err)

	payments := []txnbuild.Operation{
		&txnbuild.Payment{
			Destination: participant.Escrow.Address(),
			Amount:      stellarAmount.StringFromInt64(participant.Contribution),
			Asset:       assetParam.Asset.Asset(),
		},
	}

	tx, err = txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &account,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		IncrementSequenceNum: true,
		Operations:           payments,
	})
	require.NoError(t, err)

	tx, err = tx.Sign(networkPassphrase, participant.KP)
	require.NoError(t, err)
	_, err = client.SubmitTransaction(tx)
	require.NoError(t, err)
}

func initChannels(t *testing.T, initiator Participant, responder Participant) (initiatorChannel *state.Channel, responderChannel *state.Channel) {
	initiatorEscrowAccount := state.EscrowAccount{
		Address:        initiator.Escrow.FromAddress(),
		SequenceNumber: initiator.EscrowSequenceNumber,
	}
	responderEscrowAccount := state.EscrowAccount{
		Address:        responder.Escrow.FromAddress(),
		SequenceNumber: responder.EscrowSequenceNumber,
	}

	initiatorChannel = state.NewChannel(state.Config{
		NetworkPassphrase:   networkPassphrase,
		MaxOpenExpiry:       5 * time.Minute,
		Initiator:           true,
		LocalEscrowAccount:  &initiatorEscrowAccount,
		RemoteEscrowAccount: &responderEscrowAccount,
		LocalSigner:         initiator.KP,
		RemoteSigner:        responder.KP.FromAddress(),
	})
	responderChannel = state.NewChannel(state.Config{
		NetworkPassphrase:   networkPassphrase,
		MaxOpenExpiry:       5 * time.Minute,
		Initiator:           false,
		LocalEscrowAccount:  &responderEscrowAccount,
		RemoteEscrowAccount: &initiatorEscrowAccount,
		LocalSigner:         responder.KP,
		RemoteSigner:        initiator.KP.FromAddress(),
	})
	return initiatorChannel, responderChannel
}

func initAsset(t *testing.T, client horizonclient.ClientInterface, code string) (state.Asset, *keypair.Full) {
	issuerKP := keypair.MustRandom()
	distributorKP := keypair.MustRandom()

	err := retry(2, func() error { return createAccount(issuerKP.FromAddress(), 1_000_0000000) })
	require.NoError(t, err)
	err = retry(2, func() error { return createAccount(distributorKP.FromAddress(), 1_000_0000000) })
	require.NoError(t, err)

	distributor, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: distributorKP.Address()})
	require.NoError(t, err)

	asset := txnbuild.CreditAsset{Code: code, Issuer: issuerKP.Address()}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &distributor,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewInfiniteTimeout(),
			Operations: []txnbuild.Operation{
				&txnbuild.ChangeTrust{
					Line:  asset,
					Limit: "5000",
				},
				&txnbuild.Payment{
					Destination:   distributorKP.Address(),
					Asset:         asset,
					Amount:        "5000",
					SourceAccount: issuerKP.Address(),
				},
			},
		},
	)
	require.NoError(t, err)
	tx, err = tx.Sign(networkPassphrase, distributorKP, issuerKP)
	require.NoError(t, err)
	_, err = client.SubmitTransaction(tx)
	require.NoError(t, err)

	return state.Asset(asset.Code + ":" + asset.Issuer), distributorKP
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
		time.Sleep(time.Second)
	}
	return err
}

func fundAsset(asset state.Asset, amount int64, accountKP *keypair.Full, distributorKP *keypair.Full) error {
	distributor, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: distributorKP.Address()})
	if err != nil {
		return err
	}

	ops := []txnbuild.Operation{}
	if !asset.Native() {
		ops = append(ops, &txnbuild.ChangeTrust{
			SourceAccount: accountKP.Address(),
			Line:          asset.Asset(),
			Limit:         "5000",
		})
	}
	ops = append(ops, &txnbuild.Payment{
		Destination: accountKP.Address(),
		Amount:      stellarAmount.StringFromInt64(amount),
		Asset:       asset.Asset(),
	})
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &distributor,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		Operations:           ops,
	})
	if err != nil {
		return err
	}
	if !asset.Native() {
		tx, err = tx.Sign(networkPassphrase, accountKP)
		if err != nil {
			return err
		}
	}
	tx, err = tx.Sign(networkPassphrase, distributorKP)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}

func createAccount(account *keypair.FromAddress, startingBalance int64) error {
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
					Amount:      stellarAmount.StringFromInt64(startingBalance),
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

func assetBalance(asset state.Asset, account horizon.Account) string {
	for _, b := range account.Balances {
		if b.Asset.Code == asset.Code() {
			return b.Balance
		}
	}
	return "0"
}
