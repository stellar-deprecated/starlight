package integrationtests

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	stellarAmount "github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/starlight/sdk/state"
	"github.com/stellar/starlight/sdk/txbuild"
	"github.com/stretchr/testify/require"
)

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if hErr := horizonclient.GetError(err); hErr != nil {
		t.Errorf("horizonclient.Error=%#v", hErr)
		t.FailNow()
	} else {
		require.NoError(t, err)
	}
}

// functions to be used in the sdk/state/integrationtests integration tests

type AssetParam struct {
	Asset       state.Asset
	Distributor *keypair.Full
}

func initAccounts(t *testing.T, assetParam AssetParam) (initiator Participant, responder Participant) {
	initiator = Participant{
		Name:           "Initiator",
		KP:             keypair.MustRandom(),
		ChannelAccount: keypair.MustRandom(),
		Contribution:   1_000_0000000,
	}
	t.Log("Initiator:", initiator.KP.Address())
	t.Log("Initiator ChannelAccount:", initiator.ChannelAccount.Address())
	{
		err := retry(t, 2, func() error { return createAccount(initiator.KP.FromAddress()) })
		requireNoError(t, err)
		err = retry(t, 2, func() error {
			return fundAsset(assetParam.Asset, initiator.Contribution, initiator.KP, assetParam.Distributor)
		})
		requireNoError(t, err)

		t.Log("Initiator Contribution:", initiator.Contribution, "of asset:", assetParam.Asset.Code(), "issuer: ", assetParam.Asset.Issuer())
		initChannelAccount(t, &initiator, assetParam)
	}
	t.Log("Initiator ChannelAccount Sequence Number:", initiator.ChannelAccountSequenceNumber)

	// Setup responder.
	responder = Participant{
		Name:           "Responder",
		KP:             keypair.MustRandom(),
		ChannelAccount: keypair.MustRandom(),
		Contribution:   1_000_0000000,
	}
	t.Log("Responder:", responder.KP.Address())
	t.Log("Responder ChannelAccount:", responder.ChannelAccount.Address())
	{
		err := retry(t, 2, func() error { return createAccount(responder.KP.FromAddress()) })
		requireNoError(t, err)
		err = retry(t, 2, func() error {
			return fundAsset(assetParam.Asset, responder.Contribution, responder.KP, assetParam.Distributor)
		})
		requireNoError(t, err)

		t.Log("Responder Contribution:", responder.Contribution, "of asset:", assetParam.Asset.Code(), "issuer: ", assetParam.Asset.Issuer())
		initChannelAccount(t, &responder, assetParam)
	}
	t.Log("Responder ChannelAccount Sequence Number:", responder.ChannelAccountSequenceNumber)

	return initiator, responder
}

func initChannelAccount(t *testing.T, participant *Participant, assetParam AssetParam) {
	// create channel account
	account, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: participant.KP.Address()})
	requireNoError(t, err)
	seqNum, err := account.GetSequenceNumber()
	requireNoError(t, err)

	tx, err := txbuild.CreateChannelAccount(txbuild.CreateChannelAccountParams{
		Creator:        participant.KP.FromAddress(),
		ChannelAccount: participant.ChannelAccount.FromAddress(),
		SequenceNumber: seqNum + 1,
		Asset:          assetParam.Asset.Asset(),
	})
	requireNoError(t, err)
	tx, err = tx.Sign(networkPassphrase, participant.KP, participant.ChannelAccount)
	requireNoError(t, err)
	fbtx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: participant.KP.Address(),
		BaseFee:    txnbuild.MinBaseFee,
	})
	requireNoError(t, err)
	fbtx, err = fbtx.Sign(networkPassphrase, participant.KP)
	requireNoError(t, err)
	var txResp horizon.Transaction
	err = retry(t, 2, func() error {
		txResp, err = client.SubmitFeeBumpTransaction(fbtx)
		return err
	})
	requireNoError(t, err)
	participant.ChannelAccountSequenceNumber = int64(txResp.Ledger) << 32

	// add initial contribution, use the same contribution for each asset
	_, err = account.IncrementSequenceNumber()
	requireNoError(t, err)

	payments := []txnbuild.Operation{
		&txnbuild.Payment{
			Destination: participant.ChannelAccount.Address(),
			Amount:      stellarAmount.StringFromInt64(participant.Contribution),
			Asset:       assetParam.Asset.Asset(),
		},
	}

	tx, err = txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &account,
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		IncrementSequenceNum: true,
		Operations:           payments,
	})
	requireNoError(t, err)

	tx, err = tx.Sign(networkPassphrase, participant.KP)
	requireNoError(t, err)
	err = retry(t, 2, func() error {
		_, err = client.SubmitTransaction(tx)
		return err
	})
	requireNoError(t, err)
}

func initChannels(t *testing.T, initiator Participant, responder Participant) (initiatorChannel *state.Channel, responderChannel *state.Channel) {
	initiatorChannel = state.NewChannel(state.Config{
		NetworkPassphrase:    networkPassphrase,
		MaxOpenExpiry:        5 * time.Minute,
		Initiator:            true,
		LocalChannelAccount:  initiator.ChannelAccount.FromAddress(),
		RemoteChannelAccount: responder.ChannelAccount.FromAddress(),
		LocalSigner:          initiator.KP,
		RemoteSigner:         responder.KP.FromAddress(),
	})
	responderChannel = state.NewChannel(state.Config{
		NetworkPassphrase:    networkPassphrase,
		MaxOpenExpiry:        5 * time.Minute,
		Initiator:            false,
		LocalChannelAccount:  responder.ChannelAccount.FromAddress(),
		RemoteChannelAccount: initiator.ChannelAccount.FromAddress(),
		LocalSigner:          responder.KP,
		RemoteSigner:         initiator.KP.FromAddress(),
	})
	return initiatorChannel, responderChannel
}

func initAsset(t *testing.T, client horizonclient.ClientInterface, code string) (state.Asset, *keypair.Full) {
	issuerKP := keypair.MustRandom()
	distributorKP := keypair.MustRandom()

	err := retry(t, 2, func() error { return createAccount(issuerKP.FromAddress()) })
	requireNoError(t, err)
	err = retry(t, 2, func() error { return createAccount(distributorKP.FromAddress()) })
	requireNoError(t, err)

	var distributor horizon.Account
	err = retry(t, 5, func() error {
		distributor, err = client.AccountDetail(horizonclient.AccountRequest{AccountID: distributorKP.Address()})
		return err
	})
	requireNoError(t, err)

	asset := txnbuild.CreditAsset{Code: code, Issuer: issuerKP.Address()}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &distributor,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
			Operations: []txnbuild.Operation{
				&txnbuild.ChangeTrust{
					Line:  asset.MustToChangeTrustAsset(),
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
	requireNoError(t, err)
	tx, err = tx.Sign(networkPassphrase, distributorKP, issuerKP)
	requireNoError(t, err)
	err = retry(t, 2, func() error {
		_, err = client.SubmitTransaction(tx)
		return err
	})
	requireNoError(t, err)

	return state.Asset(asset.Code + ":" + asset.Issuer), distributorKP
}

func randomBool(t *testing.T) bool {
	t.Helper()
	b := [1]byte{}
	_, err := rand.Read(b[:])
	requireNoError(t, err)
	return b[0]%2 == 0
}

func randomPositiveInt64(t *testing.T, max int64) int64 {
	t.Helper()
	var i uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &i)
	requireNoError(t, err)
	return int64(i) % max
}

func retry(t *testing.T, maxAttempts int, f func() error) (err error) {
	t.Helper()
	for i := 0; i < maxAttempts; i++ {
		err = f()
		if err == nil {
			return
		}
		t.Logf("failed attempt %d at performing a retry-able operation: %v", i, err)
		time.Sleep(2 * time.Second)
	}
	return err
}

func fundAsset(asset state.Asset, amount int64, accountKP *keypair.Full, distributorKP *keypair.Full) error {
	distributor, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: distributorKP.Address()})
	if err != nil {
		return err
	}

	ops := []txnbuild.Operation{}
	if !asset.IsNative() {
		ops = append(ops, &txnbuild.ChangeTrust{
			SourceAccount: accountKP.Address(),
			Line:          asset.Asset().MustToChangeTrustAsset(),
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
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		Operations:           ops,
	})
	if err != nil {
		return err
	}
	if !asset.IsNative() {
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

func createAccount(account *keypair.FromAddress) error {
	// Try to fund via the fund endpoint first.
	_, err := client.Fund(account.Address())
	if err == nil {
		return nil
	}

	// Otherwise, fund via the root account.
	startingBalance := int64(1_000_0000000)
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
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
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
	fmt.Println(tx.HashHex(rootResp.NetworkPassphrase))
	fmt.Println(tx.Base64())
	resp, err := client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	fmt.Printf("%#v", resp)
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
