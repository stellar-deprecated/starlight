package state

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stellar/starlight/sdk/txbuild/txbuildtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// note: transaction result meta xdr strings for these tests were generated
// with Protocol 17.

func TestChannel_IngestTx_latestUnauthorizedDeclTxViaFeeBump(t *testing.T) {
	// Setup
	feeAccount := keypair.MustRandom()
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           1,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	initiatorChannel.UpdateRemoteChannelAccountBalance(100)
	responderChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	// Mock initiatorChannel ingested open tx successfully.
	initiatorChannel.openExecutedAndValidated = true
	responderChannel.openExecutedAndValidated = true
	initiatorChannel.initiatorChannelAccount().SequenceNumber = 1

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Create a close agreement in initiator that will remain unauthorized in
	// initiator even though it is authorized in responder.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(close.Envelope)
	require.NoError(t, err)
	assert.Equal(t, int64(0), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())

	// Pretend that responder broadcasts the declaration tx before returning
	// it to the initiator.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	declTxFeeBump, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      declTx,
		BaseFee:    txnbuild.MinBaseFee,
		FeeAccount: feeAccount.Address(),
	})
	require.NoError(t, err)
	declTxFeeBump, err = declTxFeeBump.Sign(network.TestNetworkPassphrase, feeAccount)
	require.NoError(t, err)
	declTxFeeBumpXDR, err := declTxFeeBump.Base64()
	require.NoError(t, err)
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="

	err = initiatorChannel.IngestTx(1, declTxFeeBumpXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)

	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosing, cs)

	// The initiator channel and responder channel should have the same close
	// agreements.
	assert.Equal(t, int64(8), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())
	assert.Equal(t, initiatorChannel.LatestCloseAgreement(), responderChannel.LatestCloseAgreement())
}

func TestChannel_IngestTx_latestUnauthorizedDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           1,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	initiatorChannel.UpdateRemoteChannelAccountBalance(100)
	responderChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	// Mock initiatorChannel ingested open tx successfully.
	initiatorChannel.openExecutedAndValidated = true
	responderChannel.openExecutedAndValidated = true
	initiatorChannel.initiatorChannelAccount().SequenceNumber = 1

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Create a close agreement in initiator that will remain unauthorized in
	// initiator even though it is authorized in responder.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(close.Envelope)
	require.NoError(t, err)
	assert.Equal(t, int64(0), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())

	// Pretend that responder broadcasts the declaration tx before returning
	// it to the initiator.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	declTxXDR, err := declTx.Base64()
	require.NoError(t, err)
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(1, declTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosing, cs)

	// The initiator channel and responder channel should have the same close
	// agreements.
	assert.Equal(t, int64(8), initiatorChannel.Balance())
	assert.Equal(t, int64(8), responderChannel.Balance())
	assert.Equal(t, initiatorChannel.LatestCloseAgreement(), responderChannel.LatestCloseAgreement())
}

func TestChannel_IngestTx_latestAuthorizedDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           1,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)

	// Mock initiatorChannel ingested open tx successfully.
	initiatorChannel.openExecutedAndValidated = true
	initiatorChannel.initiatorChannelAccount().SequenceNumber = 1

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Pretend responder broadcasts latest declTx, and
	// initiator ingests it.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	declTxXDR, err := declTx.Base64()
	require.NoError(t, err)
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(1, declTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosing, cs)
}

func TestChannel_IngestTx_oldDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           1,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	initiatorChannel.UpdateRemoteChannelAccountBalance(100)
	responderChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	// Mock initiatorChannel ingested open tx successfully.
	initiatorChannel.openExecutedAndValidated = true
	responderChannel.openExecutedAndValidated = true
	initiatorChannel.initiatorChannelAccount().SequenceNumber = 1

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	oldDeclTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	oldDeclTxXDR, err := oldDeclTx.Base64()
	require.NoError(t, err)

	// New payment.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	close, err = responderChannel.ConfirmPayment(close.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(close.Envelope)
	require.NoError(t, err)

	// Pretend that responder broadcasts the old declTx, and
	// initiator ingests it.
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(1, oldDeclTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosingWithOutdatedState, cs)
}

func TestChannel_IngestTx_updateBalancesNative(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorChannelAccount := keypair.MustParseAddress("GDU5LGMB7QQPP5NABMPCI7JINHSEBJ576W7O5EFCTXUUZX63OJUFRNDI")
	responderChannelAccount := keypair.MustParseAddress("GAWWANJAAOTAGEHCF7QD3Y5BAAIAWQ37323GKMI2ZKK34DJT2KX72MAF")
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	initiatorChannel.UpdateLocalChannelAccountBalance(10_000_0000000)
	initiatorChannel.UpdateRemoteChannelAccountBalance(10_000_0000000)

	// Deposit, payment of 20 xlm to initiator channel account.
	paymentResultMeta := "AAAAAgAAAAIAAAADABAqFAAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAXHr20ywANrPwAAAAGAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABAqFAAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAXHr20ywANrPwAAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECn+AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdIdugAABAp/gAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECoUAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdUYqoAABAp/gAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECoUAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABcevbTLAA2s/AAAAAcAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECoUAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABcS0fLLAA2s/AAAAAcAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err := initiatorChannel.ingestTxMetaToUpdateBalances(1, paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_020_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(10_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Deposit, claim claimable balance of 40 xlm to initiator channel account.
	claimableBalanceResultMeta := "AAAAAgAAAAIAAAADABAqUQAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXVGKqAAAQKf4AAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABAqUQAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXVGKqAAAQKf4AAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABgAAAAMAECpGAAAABAAAAAC2Zv4SS0XztUmm9JQ95wv9Sfmece0ESbeDt+pLn6FFhAAAAAEAAAAAAAAAAOnVmYH8IPf1oAseJH0oaeRAp7/1vu6Qop3pTN/bcmhYAAAAAAAAAAAAAAAAF9eEAAAAAAAAAAABAAAAAQAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAAAAAAAACAAAABAAAAAC2Zv4SS0XztUmm9JQ95wv9Sfmece0ESbeDt+pLn6FFhAAAAAMAECpRAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdUYqoAABAp/gAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECpRAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECpGAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABb6+m5nAA2s/AAAAAgAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAEAECpRAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABb6+m5nAA2s/AAAAAgAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(2, claimableBalanceResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(10_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Deposit, path paymnet send of 100 xlm to remote channel account.
	pathPaymentSendResultMeta := "AAAAAgAAAAIAAAADABArRwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWv1+jnwANrPwAAAAJAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArRwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWv1+jnwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECtHAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABa/X6OfAA2s/AAAAAoAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtHAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNmfAA2s/AAAAAoAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAMAECncAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABdIdugAABAp3AAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtHAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABeEEbIAABAp3AAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(3, pathPaymentSendResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Operation not involving an channel account should not change balances.
	noOpResultMeta := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(4, noOpResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Withdrawal, payment of 1000 xlm from initiator channel account.
	withdrawalResultMeta := "AAAAAgAAAAIAAAADABAregAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXp9T3OAAQKf4AAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABAregAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXp9T3OAAQKf4AAAACAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECt6AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABny8AocAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABUYLkoAABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(5, withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(9_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Withdrawal, payment of 1000 xlm from responder channel account to initiator channel account.
	withdrawalResultMeta = "AAAAAgAAAAIAAAADABArsgAAAAAAAAAALWA1IAOmAxDiL+A946EAEAtDf962ZTEaypW+DTPSr/0AAAAXhBGyAAAQKdwAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABArsgAAAAAAAAAALWA1IAOmAxDiL+A946EAEAtDf962ZTEaypW+DTPSr/0AAAAXhBGyAAAQKdwAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABUYLkoAABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECuyAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECuyAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABeEEbIAABAp3AAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECuyAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABUwBc4AABAp3AAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(6, withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(9_100_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Bad xdr string should result in no change.
	err = initiatorChannel.ingestTxMetaToUpdateBalances(7, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing the result meta xdr:")
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(9_100_0000000), initiatorChannel.remoteChannelAccount.Balance)
}

func TestChannel_IngestTx_updateBalancesNonNative(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorChannelAccount := keypair.MustParseAddress("GBTIPOMXZUUPVVII2EO4533MP5DUKVMACBRQ73HVW3CZRUUIOESIDZ4O")
	responderChannelAccount := keypair.MustParseAddress("GDPR4IOSNLZS2HNE2PM7E2WJOUFCPATP3O4LGXJNE3K5HO42L7HSL6SO")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})

	asset := Asset("TEST:GAOWNZMMFW25MWBAWKRYBMIEKY2KKEWKOINP2IDTRYOQ4DOEW26NV437")

	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           1,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)

	initiatorChannel.UpdateLocalChannelAccountBalance(1_000_0000000)
	initiatorChannel.UpdateRemoteChannelAccountBalance(1_000_0000000)

	// Deposit, payment of 10 TEST to issuer channel account.
	paymentResultMeta := "AAAAAgAAAAIAAAADABA5KgAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHbmDAAQOA4AAAADAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABA5KgAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHbmDAAQOA4AAAAEAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEDj9AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAlQL5AB//////////wAAAAEAAAAAAAAAAAAAAAEAEDkqAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(1, paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_010_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Deposit, path paymnet send of 100 TEST to initiator channel account.
	pathPaymentSendResultMeta := "AAAAAgAAAAIAAAADABBjyQAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHblRAAQOA4AAAAFAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBjyQAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHblRAAQOA4AAAAGAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEDkqAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAEAEGPJAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAApWcjwB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(2, pathPaymentSendResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_110_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Deposit, claim claimable balance of 50 TEST to initiator channel account.
	claimableBalanceResultMeta := "AAAAAgAAAAQAAAADABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAADABA47QAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAACAAAAAMAEGPhAAAABAAAAADT2NmmO5Sjq1foqo2nqykq8A+EJYwwSRG1upvSppSswgAAAAEAAAAAAAAAAGaHuZfNKPrVCNEdzu9sf0dFVYAQYw/s9bbFmNKIcSSBAAAAAAAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAB3NZQAAAAAAAAAAAQAAAAEAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAAAAAAAgAAAAQAAAAA09jZpjuUo6tX6KqNp6spKvAPhCWMMEkRtbqb0qaUrMIAAAADABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAADABBjyQAAAAEAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAABVEVTVAAAAAAdZuWMLbXWWCCyo4CxBFY0pRLKchr9IHOOHQ4NxLa82gAAAAKVnI8Af/////////8AAAABAAAAAAAAAAAAAAABABBj/gAAAAEAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAABVEVTVAAAAAAdZuWMLbXWWCCyo4CxBFY0pRLKchr9IHOOHQ4NxLa82gAAAAKzafQAf/////////8AAAABAAAAAAAAAAAAAAADABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	err = initiatorChannel.ingestTxMetaToUpdateBalances(3, claimableBalanceResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_160_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Operation not involving an channel account should not change balances.
	noOpResultMeta := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(4, noOpResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_160_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Withdrawal, payment of 150 TEST from initiator channel account.
	withdrawalResultMeta := "AAAAAgAAAAIAAAADABBkPgAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbmcAAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkPgAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbmcAAQN/UAAAAFAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEGP+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAArNp9AB//////////wAAAAEAAAAAAAAAAAAAAAEAEGQ+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(5, withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_010_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Withdrawal, payment of 50 TEST from responder channel account to initiator channel account.
	withdrawalResultMeta = "AAAAAgAAAAIAAAADABBkXAAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbm1AAQOE8AAAACAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkXAAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbm1AAQOE8AAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAEGQ+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAEAEGRcAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAnfPKgB//////////wAAAAEAAAAAAAAAAAAAAAMAEDj9AAAAAQAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAlQL5AB//////////wAAAAEAAAAAAAAAAAAAAAEAEGRcAAAAAQAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAjY+fwB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(6, withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// Bad xdr string should result in no change.
	err = initiatorChannel.ingestTxMetaToUpdateBalances(7, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing the result meta xdr:")
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteChannelAccount.Balance)

	// A payment sending xlm should not affect balance.
	paymentResultMeta = "AAAAAgAAAAIAAAADABBkbQAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbmcAAQOE8AAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkbQAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbmcAAQOE8AAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAEGQ+AAAAAAAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAABdIduZwABA39QAAAAUAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAEGRtAAAAAAAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAABe/rHpwABA39QAAAAUAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAEGRtAAAAAAAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAABdIduZwABA4TwAAAAQAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAEGRtAAAAAAAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAABbRQVJwABA4TwAAAAQAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(8, paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localChannelAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteChannelAccount.Balance)
}

func TestChannel_IngestTx_updateBalancesNative_withLiabilities(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorChannelAccount := keypair.MustParseAddress("GBTIPOMXZUUPVVII2EO4533MP5DUKVMACBRQ73HVW3CZRUUIOESIDZ4O")
	responderChannelAccount := keypair.MustParseAddress("GDPR4IOSNLZS2HNE2PM7E2WJOUFCPATP3O4LGXJNE3K5HO42L7HSL6SO")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})

	{
		open, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			ExpiresAt:                  time.Now().Add(time.Minute),
			StartingSequence:           1,
		})
		require.NoError(t, err)
		open, err = responderChannel.ConfirmOpen(open.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(open.Envelope)
		require.NoError(t, err)
	}

	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)

	placeholderTx, _, err := initiatorChannel.CloseTxs()
	require.NoError(t, err)
	placeholderXDR, err := placeholderTx.Base64()
	require.NoError(t, err)

	type TestCase struct {
		channelAccount    *keypair.FromAddress
		balance           xdr.Int64
		buying            xdr.Int64
		wantBalanceLocal  int64
		wantBalanceRemote int64
	}

	testCases := []TestCase{
		{initiatorChannelAccount, 200, 200, 0, 0},
		{initiatorChannelAccount, 1000, 100, 900, 0},
		{initiatorChannelAccount, 1000, 0, 1000, 0},
		{responderChannelAccount, 200, 200, 0, 0},
		{responderChannelAccount, 1000, 100, 0, 900},
		{responderChannelAccount, 1000, 0, 0, 1000},
	}

	for i, tc := range testCases {
		initiatorChannel.UpdateLocalChannelAccountBalance(0)
		initiatorChannel.UpdateRemoteChannelAccountBalance(0)
		ale, err := xdr.NewAccountEntryExt(1, xdr.AccountEntryExtensionV1{
			Liabilities: xdr.Liabilities{
				Buying:  tc.buying,
				Selling: 100,
			},
		})
		require.NoError(t, err)

		paymentResultMeta, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
			{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(tc.channelAccount.Address()),
					Balance:   tc.balance,
					Ext:       ale,
				},
			},
		})
		require.NoError(t, err)
		err = initiatorChannel.IngestTx(int64(i), placeholderXDR, validResultXDR, paymentResultMeta)
		require.NoError(t, err)
		assert.Equal(t, tc.wantBalanceLocal, initiatorChannel.localChannelAccount.Balance)
		assert.Equal(t, tc.wantBalanceRemote, initiatorChannel.remoteChannelAccount.Balance)
	}
}

func TestChannel_IngestTx_updateBalancesNonNative_withLiabilities(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorChannelAccount := keypair.MustParseAddress("GBTIPOMXZUUPVVII2EO4533MP5DUKVMACBRQ73HVW3CZRUUIOESIDZ4O")
	responderChannelAccount := keypair.MustParseAddress("GDPR4IOSNLZS2HNE2PM7E2WJOUFCPATP3O4LGXJNE3K5HO42L7HSL6SO")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})

	asset := Asset("TEST:GAOWNZMMFW25MWBAWKRYBMIEKY2KKEWKOINP2IDTRYOQ4DOEW26NV437")

	{
		open, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      asset,
			ExpiresAt:                  time.Now().Add(time.Minute),
			StartingSequence:           1,
		})
		require.NoError(t, err)
		open, err = responderChannel.ConfirmOpen(open.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(open.Envelope)
		require.NoError(t, err)
	}

	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)

	placeholderTx, _, err := initiatorChannel.CloseTxs()
	require.NoError(t, err)
	placeholderXDR, err := placeholderTx.Base64()
	require.NoError(t, err)

	type TestCase struct {
		channelAccount    *keypair.FromAddress
		trustLineBalance  xdr.Int64
		selling           xdr.Int64
		wantBalanceLocal  int64
		wantBalanceRemote int64
	}

	testCases := []TestCase{
		{initiatorChannelAccount, 200, 200, 0, 0},
		{initiatorChannelAccount, 1000, 100, 900, 0},
		{initiatorChannelAccount, 1000, 0, 1000, 0},
		{responderChannelAccount, 200, 200, 0, 0},
		{responderChannelAccount, 1000, 100, 0, 900},
		{responderChannelAccount, 1000, 0, 0, 1000},
	}

	for i, tc := range testCases {
		initiatorChannel.UpdateLocalChannelAccountBalance(0)
		initiatorChannel.UpdateRemoteChannelAccountBalance(0)
		tle, err := xdr.NewTrustLineEntryExt(1, xdr.TrustLineEntryV1{
			Liabilities: xdr.Liabilities{
				Buying:  100,
				Selling: tc.selling,
			},
		})
		require.NoError(t, err)

		paymentResultMeta, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
			{
				Type: xdr.LedgerEntryTypeTrustline,
				TrustLine: &xdr.TrustLineEntry{
					AccountId: xdr.MustAddress(tc.channelAccount.Address()),
					Asset:     xdr.MustNewCreditAsset(asset.Code(), asset.Issuer()).ToTrustLineAsset(),
					Balance:   tc.trustLineBalance,
					Ext:       tle,
				},
			},
		})
		require.NoError(t, err)
		err = initiatorChannel.IngestTx(int64(i), placeholderXDR, validResultXDR, paymentResultMeta)
		require.NoError(t, err)
		assert.Equal(t, tc.wantBalanceLocal, initiatorChannel.localChannelAccount.Balance)
		assert.Equal(t, tc.wantBalanceRemote, initiatorChannel.remoteChannelAccount.Balance)
	}
}

func TestChannel_IngestTx_updateState_nativeAsset(t *testing.T) {
	initiatorSigner := keypair.MustParseFull("SCBMAMOPWKL2YHWELK63VLAY2R74A6GTLLD4ON223B7K5KZ37MUR6IDF")
	responderSigner := keypair.MustParseFull("SBM7D2IIDSRX5Y3VMTMTXXPB6AIB4WYGZBC2M64U742BNOK32X6SW4NF")

	initiatorChannelAccount := keypair.MustParseAddress("GAU4CFXQI6HLK5PPY2JWU3GMRJIIQNLF24XRAHX235F7QTG6BEKLGQ36")
	responderChannelAccount := keypair.MustParseAddress("GBQNGSEHTFC4YGQ3EXHIL7JQBA6265LFANKFFAYKHM7JFGU5CORROEGO")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})

	// Before confirming an open, channel should not be open.
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           28037546508289,
	})
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)

	openTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	openTxXDR, err := openTx.Base64()
	require.NoError(t, err)

	// After confirming an open, but not ingested the open tx,
	// channel should not be open.
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	// Valid open transaction xdr.
	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)
	resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           28037546508289,
		Asset:                   txnbuild.NativeAsset{},
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(1, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)
	assert.Equal(t, openTx.SequenceNumber(), initiatorChannel.initiatorChannelAccount().SequenceNumber)

	// Invalid Result XDR, should return with no state changes and shouldn't
	// affect that fact that the account was confirmed already opened.
	invalidResultXDR, err := txbuildtest.BuildResultXDR(false)
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(2, openTxXDR, invalidResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)

	// Not the openTx, should return with no state change. The tx is the
	// same seq number as the openTx, and has identical operation as the
	// openTx, but has been modified to contain a memo so it is a different
	// tx.
	imposterTx := openTx.ToXDR()
	imposterTxMemo, err := xdr.NewMemo(xdr.MemoTypeMemoText, "imposter")
	require.NoError(t, err)
	imposterTx.V1.Tx.Memo = imposterTxMemo
	imposterTxBytes, err := imposterTx.MarshalBinary()
	require.NoError(t, err)
	imposterTxXDR := base64.StdEncoding.EncodeToString(imposterTxBytes)
	err = initiatorChannel.IngestTx(3, imposterTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)
}

func TestChannel_IngestTx_updateState_nonNativeAsset(t *testing.T) {
	initiatorSigner := keypair.MustParseFull("SBQEQ2SJLI4DKK7T7DYNGAVHDIC2FJSMD2D4HZQTH67Y4YJ2HCIW23E2")
	responderSigner := keypair.MustParseFull("SD3VHLBEPXOW74B2VLMRSNERLL4HMULIYNLCVLBSYS3ZIFJE5T5VIOBO")

	initiatorChannelAccount := keypair.MustParseAddress("GDF7GNJLI6H5ENPPVHRNQF3LN6AT2N2UTXVX57INKELND3DIMROCYXCC")
	responderChannelAccount := keypair.MustParseAddress("GBEWOADTWFUS5EKEDB63X5KDWAKBJ32A5WDZKXENOCU3XQTM26GKBV2X")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})

	asset := Asset("ABDC:GBW5R35MPDT6JPFRQ3NEHQBMBLX7V6LAPAPPXL6FYQQKNVOCWGV7LKDQ")

	// Before confirming an open, channel should not be open.
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		Asset:                      asset,
		StartingSequence:           24936580120577,
	})
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	// Open steps.
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)

	openTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	openTxXDR, err := openTx.Base64()
	require.NoError(t, err)

	// After confirming an open, but not ingested the open tx,
	// channel should not be open.
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)

	// Valid open transaction xdr.
	resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(1, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)

	// XDR without updated initiator channel account should give error.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: keypair.MustRandom().Address(), // Some other account rather than initiator.
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(2, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "could not find an updated ledger entry for both channel accounts")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong initiator channel account sequence number.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120578, // Wrong sequence number.
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(3, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "incorrect initiator channel account sequence number found, found: 24936580120578 want: 24936580120577")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong signer weights.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		InitiatorSignerWeight:   2, // Wrong weight.
		ResponderSigner:         responderSigner.Address(),
		ResponderSignerWeight:   1,
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(4, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "signer not found or incorrect weight")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong signers - extra signer.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		ExtraSigner:             keypair.MustRandom().Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(5, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "unexpected signer found on channel account")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong thresholds.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		Thresholds:              xdr.Thresholds{0, 1, 1, 1}, // Wrong thresholds.
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(6, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "incorrect initiator channel account thresholds found")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Missing Trustline.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   txnbuild.NativeAsset{}, // Native asset here causes no trustline.
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(7, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "trustline not found for asset ABDC:GBW5R35MPDT6JPFRQ3NEHQBMBLX7V6LAPAPPXL6FYQQKNVOCWGV7LKDQ")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong Trustline flag.
	resultMetaXDR, err = txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
		InitiatorSigner:         initiatorSigner.Address(),
		ResponderSigner:         responderSigner.Address(),
		InitiatorChannelAccount: initiatorChannelAccount.Address(),
		ResponderChannelAccount: responderChannelAccount.Address(),
		StartSequence:           24936580120577,
		Asset:                   asset.Asset(),
		TrustLineFlag:           xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag, // Wrong trustline flag.
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(8, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "trustline not authorized")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)
}

func TestChannel_IngestTx_updateState_invalid_initiatorChannelAccountHasExtraSigner(t *testing.T) {
	initiatorSigner := keypair.MustParseFull("SAWFAB3JBDIB3WUW4GDWZJFDH4LYK646PFU2TUTQ2QPIJ7UDPFDALDLJ")
	responderSigner := keypair.MustParseFull("SDM45WXZOOXEOG23LVWDHBUYTSLZ27YKIN5N3C6QBD3TIIWWQHFFH7FI")

	initiatorChannelAccount := keypair.MustParseAddress("GC264CPQA3WZ64USLDCHXG4AFUYGMQXUIW7UY5WYM2QA2WFPS6FARAD4")
	responderChannelAccount := keypair.MustParseAddress("GA63LTOE6CXAUGQTQW4332Z6UDBTAN7KTXSJKN4Y5KP4DBJFKEYOHWM7")

	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		MaxOpenExpiry:        time.Hour,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
		StartingSequence:           102,
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open.Envelope)
	require.NoError(t, err)

	openTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	openTxXDR, err := openTx.Base64()
	require.NoError(t, err)

	// Initiator ChannelAccount has an extra signer before the open tx, should fail.
	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)
	resultMetaXDR, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
		{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress(initiatorChannelAccount.Address()),
				SeqNum:    102,
				Signers: []xdr.Signer{
					{
						Key:    xdr.MustSigner("GAKDNXUGEIRGESAXOPUHU4GOWLVYGQFJVHQOGFXKBXDGZ7AKMPPSDDPV"),
						Weight: 1,
					},
				},
				Thresholds: xdr.Thresholds{0, 2, 2, 2},
			},
		},
		{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress(responderChannelAccount.Address()),
			},
		},
	})
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(1, openTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "unexpected signer found on channel account")
}

func TestChannel_IngestTx_seqNumCantGoBackwards(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:         initiatorSigner.Address(),
			ResponderSigner:         responderSigner.Address(),
			InitiatorChannelAccount: initiatorChannelAccount.Address(),
			ResponderChannelAccount: responderChannelAccount.Address(),
			StartSequence:           101,
			Asset:                   txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	oldDeclTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	oldDeclTxXDR, err := oldDeclTx.Base64()
	require.NoError(t, err)

	// New payment.
	{
		close, err := initiatorChannel.ProposePayment(8)
		require.NoError(t, err)
		close, err = responderChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
	}

	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	declTxXDR, err := declTx.Base64()
	require.NoError(t, err)

	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)

	// Ingest latest declTx to go into StateClosing.
	err = initiatorChannel.IngestTx(2, declTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateClosing, cs)
	assert.Equal(t, int64(105), initiatorChannel.initiatorChannelAccount().SequenceNumber)

	// Ingesting an old transaction with a previous seqNum should not move state backwards.
	err = initiatorChannel.IngestTx(3, oldDeclTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateClosing, cs)
	assert.Equal(t, int64(105), initiatorChannel.initiatorChannelAccount().SequenceNumber)

	// Imposter open tx can not be ingested and move state back.
	openTx, err := initiatorChannel.OpenTx()
	require.NoError(t, err)
	imposterTx := openTx.ToXDR()
	imposterTxMemo, err := xdr.NewMemo(xdr.MemoTypeMemoText, "imposter")
	require.NoError(t, err)
	imposterTx.V1.Tx.Memo = imposterTxMemo
	imposterTxBytes, err := imposterTx.MarshalBinary()
	require.NoError(t, err)
	imposterTxXDR := base64.StdEncoding.EncodeToString(imposterTxBytes)
	err = initiatorChannel.IngestTx(4, imposterTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)

	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateClosing, cs)
}

func TestChannel_IngestTx_balanceCantGoBackwards(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:         initiatorSigner.Address(),
			ResponderSigner:         responderSigner.Address(),
			InitiatorChannelAccount: initiatorChannelAccount.Address(),
			ResponderChannelAccount: responderChannelAccount.Address(),
			StartSequence:           101,
			Asset:                   txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	// New payment.
	{
		close, err := initiatorChannel.ProposePayment(8)
		require.NoError(t, err)
		close, err = responderChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
	}

	// Create two txs that each deposit 10 into channel account.
	depositer := keypair.MustRandom().Address()
	tx1, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: depositer, Sequence: 1},
		BaseFee:       txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{Destination: initiatorChannelAccount.Address(), Asset: txnbuild.NativeAsset{}, Amount: "10"},
			&txnbuild.Payment{Destination: responderChannelAccount.Address(), Asset: txnbuild.NativeAsset{}, Amount: "10"},
		},
	})
	require.NoError(t, err)
	tx1XDR, err := tx1.Base64()
	require.NoError(t, err)
	tx1ResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)
	tx1ResultMetaXDR, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
		{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.AccountEntry{AccountId: xdr.MustAddress(initiatorChannelAccount.Address()), Balance: 110}},
		{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.AccountEntry{AccountId: xdr.MustAddress(responderChannelAccount.Address()), Balance: 110}},
	})
	require.NoError(t, err)
	tx2, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: depositer, Sequence: 2},
		BaseFee:       txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{Destination: initiatorChannelAccount.Address(), Asset: txnbuild.NativeAsset{}, Amount: "5"},
			&txnbuild.Payment{Destination: responderChannelAccount.Address(), Asset: txnbuild.NativeAsset{}, Amount: "5"},
		},
	})
	require.NoError(t, err)
	tx2XDR, err := tx2.Base64()
	require.NoError(t, err)
	tx2ResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)
	tx2ResultMetaXDR, err := txbuildtest.BuildResultMetaXDR([]xdr.LedgerEntryData{
		{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.AccountEntry{AccountId: xdr.MustAddress(initiatorChannelAccount.Address()), Balance: 115}},
		{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.AccountEntry{AccountId: xdr.MustAddress(responderChannelAccount.Address()), Balance: 115}},
	})
	require.NoError(t, err)

	// Process them out-of-order.
	err = initiatorChannel.IngestTx(3, tx2XDR, tx2ResultXDR, tx2ResultMetaXDR)
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(2, tx1XDR, tx1ResultXDR, tx1ResultMetaXDR)
	require.NoError(t, err)

	// Check that balance is the latest balance.
	assert.Equal(t, int64(115), initiatorChannel.initiatorChannelAccount().Balance)
	assert.Equal(t, int64(115), initiatorChannel.responderChannelAccount().Balance)
}

func TestChannel_IngestTx_OpenClose(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})

	// Before channel is open IngestTx should error.
	err := initiatorChannel.IngestTx(1, "", "", "")
	require.EqualError(t, err, "channel has not been opened")

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:         initiatorSigner.Address(),
			ResponderSigner:         responderSigner.Address(),
			InitiatorChannelAccount: initiatorChannelAccount.Address(),
			ResponderChannelAccount: responderChannelAccount.Address(),
			StartSequence:           101,
			Asset:                   txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(2, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// Put channel into the Closed state.
	{
		_, closeTx, err := initiatorChannel.CloseTxs()
		require.NoError(t, err)

		closeTxXDR, err := closeTx.Base64()
		require.NoError(t, err)

		placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
		validResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(3, closeTxXDR, validResultXDR, placeholderXDR)
		require.NoError(t, err)
		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosed, cs)
	}

	// After channel is closed IngestTx should error.
	err = initiatorChannel.IngestTx(4, "", "", "")
	require.EqualError(t, err, "channel has been closed")
}

func TestChannel_IngestTx_ingestOldTransactions(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorChannelAccount := keypair.MustRandom().FromAddress()
	responderChannelAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            true,
		LocalSigner:          initiatorSigner,
		RemoteSigner:         responderSigner.FromAddress(),
		LocalChannelAccount:  initiatorChannelAccount,
		RemoteChannelAccount: responderChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:    network.TestNetworkPassphrase,
		Initiator:            false,
		LocalSigner:          responderSigner,
		RemoteSigner:         initiatorSigner.FromAddress(),
		LocalChannelAccount:  responderChannelAccount,
		RemoteChannelAccount: initiatorChannelAccount,
		MaxOpenExpiry:        2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)

		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:         initiatorSigner.Address(),
			ResponderSigner:         responderSigner.Address(),
			InitiatorChannelAccount: initiatorChannelAccount.Address(),
			ResponderChannelAccount: responderChannelAccount.Address(),
			StartSequence:           101,
			Asset:                   txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(2, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = responderChannel.IngestTx(2, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
	}
	initiatorChannel.UpdateLocalChannelAccountBalance(100)
	responderChannel.UpdateRemoteChannelAccountBalance(100)

	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	validResultXDR, err := txbuildtest.BuildResultXDR(true)
	require.NoError(t, err)

	oldDeclTx, oldCloseTx, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	oldDeclXDR, err := oldDeclTx.Base64()
	require.NoError(t, err)
	oldCloseXDR, err := oldCloseTx.Base64()
	require.NoError(t, err)

	// New payment.
	{
		close, err := initiatorChannel.ProposePayment(8)
		require.NoError(t, err)
		close, err = responderChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(close.Envelope)
		require.NoError(t, err)
	}

	// Close channel with old transactions.
	{
		err = initiatorChannel.IngestTx(1, oldDeclXDR, validResultXDR, placeholderXDR)
		require.NoError(t, err)
		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosingWithOutdatedState, cs)

		err = initiatorChannel.IngestTx(1, oldCloseXDR, validResultXDR, placeholderXDR)
		require.NoError(t, err)
		cs, err = initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateClosedWithOutdatedState, cs)
	}

	// Once closed with old closeTx, ingesting new transactions should error.
	err = initiatorChannel.IngestTx(1, oldCloseXDR, validResultXDR, placeholderXDR)
	require.EqualError(t, err, "channel has been closed")
}
