package state

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
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
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalEscrowAccountBalance(100)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(100)
	responderChannel.UpdateLocalEscrowAccountBalance(100)
	responderChannel.UpdateRemoteEscrowAccountBalance(100)

	// Mock initiatorChannel ingested formation tx successfully.
	initiatorChannel.openExecutedAndValidated = true

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Create a close agreement in initiator that will remain unauthorized in
	// initiator even though it is authorized in responder.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(close)
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
	err = initiatorChannel.IngestTx(declTxFeeBumpXDR, validResultXDR, placeholderXDR)
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
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalEscrowAccountBalance(100)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(100)
	responderChannel.UpdateLocalEscrowAccountBalance(100)
	responderChannel.UpdateRemoteEscrowAccountBalance(100)

	// Mock initiatorChannel ingested formation tx successfully.
	initiatorChannel.openExecutedAndValidated = true

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Create a close agreement in initiator that will remain unauthorized in
	// initiator even though it is authorized in responder.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(close)
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
	err = initiatorChannel.IngestTx(declTxXDR, validResultXDR, placeholderXDR)
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
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	// Mock initiatorChannel ingested formation tx successfully.
	initiatorChannel.openExecutedAndValidated = true

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Pretend responder broadcasts latest declTx, and
	// initiator ingests it.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	declTxXDR, err := declTx.Base64()
	require.NoError(t, err)
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(declTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosing, cs)
}

func TestChannel_IngestTx_oldDeclTx(t *testing.T) {
	// Setup
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()
	initiatorEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)
	initiatorChannel.UpdateLocalEscrowAccountBalance(100)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(100)
	responderChannel.UpdateLocalEscrowAccountBalance(100)
	responderChannel.UpdateRemoteEscrowAccountBalance(100)

	// Mock initiatorChannel ingested formation tx successfully.
	initiatorChannel.openExecutedAndValidated = true

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	oldDeclTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	oldDeclTxXDR, err := oldDeclTx.Base64()
	require.NoError(t, err)

	// New payment.
	close, err := initiatorChannel.ProposePayment(8)
	require.NoError(t, err)
	close, err = responderChannel.ConfirmPayment(close)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(close)
	require.NoError(t, err)

	// Pretend that responder broadcasts the old declTx, and
	// initiator ingests it.
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(oldDeclTxXDR, validResultXDR, placeholderXDR)
	require.NoError(t, err)
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	require.Equal(t, StateClosingWithOutdatedState, cs)
}

func TestChannel_IngestTx_updateBalancesNative(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorEscrow, err := keypair.ParseAddress("GDU5LGMB7QQPP5NABMPCI7JINHSEBJ576W7O5EFCTXUUZX63OJUFRNDI")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GAWWANJAAOTAGEHCF7QD3Y5BAAIAWQ37323GKMI2ZKK34DJT2KX72MAF")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	initiatorChannel.UpdateLocalEscrowAccountBalance(10_000_0000000)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(10_000_0000000)

	// Deposit, payment of 20 xlm to initiator escrow.
	paymentResultMeta := "AAAAAgAAAAIAAAADABAqFAAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAXHr20ywANrPwAAAAGAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABAqFAAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAXHr20ywANrPwAAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECn+AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdIdugAABAp/gAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECoUAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdUYqoAABAp/gAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECoUAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABcevbTLAA2s/AAAAAcAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECoUAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABcS0fLLAA2s/AAAAAcAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_020_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(10_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Deposit, claim claimable balance of 40 xlm to initiator escrow.
	claimableBalanceResultMeta := "AAAAAgAAAAIAAAADABAqUQAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXVGKqAAAQKf4AAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABAqUQAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXVGKqAAAQKf4AAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABgAAAAMAECpGAAAABAAAAAC2Zv4SS0XztUmm9JQ95wv9Sfmece0ESbeDt+pLn6FFhAAAAAEAAAAAAAAAAOnVmYH8IPf1oAseJH0oaeRAp7/1vu6Qop3pTN/bcmhYAAAAAAAAAAAAAAAAF9eEAAAAAAAAAAABAAAAAQAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAAAAAAAACAAAABAAAAAC2Zv4SS0XztUmm9JQ95wv9Sfmece0ESbeDt+pLn6FFhAAAAAMAECpRAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdUYqoAABAp/gAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECpRAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECpGAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABb6+m5nAA2s/AAAAAgAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAEAECpRAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABb6+m5nAA2s/AAAAAgAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(claimableBalanceResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(10_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Deposit, path paymnet send of 100 xlm to remote escrow
	pathPaymentSendResultMeta := "AAAAAgAAAAIAAAADABArRwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWv1+jnwANrPwAAAAJAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArRwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWv1+jnwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECtHAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABa/X6OfAA2s/AAAAAoAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtHAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNmfAA2s/AAAAAoAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAMAECncAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABdIdugAABAp3AAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtHAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABeEEbIAABAp3AAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(pathPaymentSendResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Operation not involving an escrow account should not change balances.
	noOpResultMeta := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(noOpResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Withdrawal, payment of 1000 xlm from initiator escrow.
	withdrawalResultMeta := "AAAAAgAAAAIAAAADABAregAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXp9T3OAAQKf4AAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABAregAAAAAAAAAA6dWZgfwg9/WgCx4kfShp5ECnv/W+7pCinelM39tyaFgAAAAXp9T3OAAQKf4AAAACAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECt6AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABny8AocAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABUYLkoAABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(9_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(10_100_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Withdrawal, payment of 1000 xlm from responder escrow to initiator escrow.
	withdrawalResultMeta = "AAAAAgAAAAIAAAADABArsgAAAAAAAAAALWA1IAOmAxDiL+A946EAEAtDf962ZTEaypW+DTPSr/0AAAAXhBGyAAAQKdwAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABArsgAAAAAAAAAALWA1IAOmAxDiL+A946EAEAtDf962ZTEaypW+DTPSr/0AAAAXhBGyAAAQKdwAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAECt6AAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABUYLkoAABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECuyAAAAAAAAAADp1ZmB/CD39aALHiR9KGnkQKe/9b7ukKKd6Uzf23JoWAAAABdsOi4AABAp/gAAAAIAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECuyAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABeEEbIAABAp3AAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECuyAAAAAAAAAAAtYDUgA6YDEOIv4D3joQAQC0N/3rZlMRrKlb4NM9Kv/QAAABUwBc4AABAp3AAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(9_100_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Bad xdr string should result in no change.
	err = initiatorChannel.ingestTxMetaToUpdateBalances("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing the result meta xdr:")
	assert.Equal(t, int64(10_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(9_100_0000000), initiatorChannel.remoteEscrowAccount.Balance)
}

func TestChannel_IngestTx_updateBalancesNonNative(t *testing.T) {
	initiatorSigner := keypair.MustRandom()
	responderSigner := keypair.MustRandom()

	initiatorEscrow, err := keypair.ParseAddress("GBTIPOMXZUUPVVII2EO4533MP5DUKVMACBRQ73HVW3CZRUUIOESIDZ4O")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GDPR4IOSNLZS2HNE2PM7E2WJOUFCPATP3O4LGXJNE3K5HO42L7HSL6SO")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(101),
	}
	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(202),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})

	asset := Asset("TEST:GAOWNZMMFW25MWBAWKRYBMIEKY2KKEWKOINP2IDTRYOQ4DOEW26NV437")

	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      asset,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	initiatorChannel.UpdateLocalEscrowAccountBalance(1_000_0000000)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(1_000_0000000)

	// Deposit, payment of 10 TEST to issuer escrow.
	paymentResultMeta := "AAAAAgAAAAIAAAADABA5KgAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHbmDAAQOA4AAAADAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABA5KgAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHbmDAAQOA4AAAAEAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEDj9AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAlQL5AB//////////wAAAAEAAAAAAAAAAAAAAAEAEDkqAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_010_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Deposit, path paymnet send of 100 TEST to initiator escrow
	pathPaymentSendResultMeta := "AAAAAgAAAAIAAAADABBjyQAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHblRAAQOA4AAAAFAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBjyQAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHblRAAQOA4AAAAGAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEDkqAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAEAEGPJAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAApWcjwB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(pathPaymentSendResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_110_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Deposit, claim claimable balance of 50 TEST to initiator escrow.
	claimableBalanceResultMeta := "AAAAAgAAAAQAAAADABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAADABA47QAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAACAAAAAMAEGPhAAAABAAAAADT2NmmO5Sjq1foqo2nqykq8A+EJYwwSRG1upvSppSswgAAAAEAAAAAAAAAAGaHuZfNKPrVCNEdzu9sf0dFVYAQYw/s9bbFmNKIcSSBAAAAAAAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAB3NZQAAAAAAAAAAAQAAAAEAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAAAAAAAgAAAAQAAAAA09jZpjuUo6tX6KqNp6spKvAPhCWMMEkRtbqb0qaUrMIAAAADABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbm1AAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAADABBjyQAAAAEAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAABVEVTVAAAAAAdZuWMLbXWWCCyo4CxBFY0pRLKchr9IHOOHQ4NxLa82gAAAAKVnI8Af/////////8AAAABAAAAAAAAAAAAAAABABBj/gAAAAEAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAABVEVTVAAAAAAdZuWMLbXWWCCyo4CxBFY0pRLKchr9IHOOHQ4NxLa82gAAAAKzafQAf/////////8AAAABAAAAAAAAAAAAAAADABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABBj/gAAAAAAAAAAHWbljC211lggsqOAsQRWNKUSynIa/SBzjh0ODcS2vNoAAAAXSHaqegAQOA4AAAAHAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	err = initiatorChannel.ingestTxMetaToUpdateBalances(claimableBalanceResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_160_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Operation not involving an escrow account should not change balances.
	noOpResultMeta := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(noOpResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_160_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Withdrawal, payment of 150 TEST from initiator escrow.
	withdrawalResultMeta := "AAAAAgAAAAIAAAADABBkPgAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbmcAAQN/UAAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkPgAAAAAAAAAAZoe5l80o+tUI0R3O72x/R0VVgBBjD+z1tsWY0ohxJIEAAAAXSHbmcAAQN/UAAAAFAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAEGP+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAArNp9AB//////////wAAAAEAAAAAAAAAAAAAAAEAEGQ+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_010_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(1_000_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Withdrawal, payment of 50 TEST from responder escrow to initiator escrow.
	withdrawalResultMeta = "AAAAAgAAAAIAAAADABBkXAAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbm1AAQOE8AAAACAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkXAAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbm1AAQOE8AAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAEGQ+AAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAloBxQB//////////wAAAAEAAAAAAAAAAAAAAAEAEGRcAAAAAQAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAnfPKgB//////////wAAAAEAAAAAAAAAAAAAAAMAEDj9AAAAAQAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAlQL5AB//////////wAAAAEAAAAAAAAAAAAAAAEAEGRcAAAAAQAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAAAFURVNUAAAAAB1m5YwttdZYILKjgLEEVjSlEspyGv0gc44dDg3EtrzaAAAAAjY+fwB//////////wAAAAEAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(withdrawalResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// Bad xdr string should result in no change.
	err = initiatorChannel.ingestTxMetaToUpdateBalances("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing the result meta xdr:")
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteEscrowAccount.Balance)

	// A payment sending xlm should not affect balance.
	paymentResultMeta = "AAAAAgAAAAIAAAADABBkbQAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbmcAAQOE8AAAADAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABABBkbQAAAAAAAAAA3x4h0mrzLR2k09nyasl1CieCb9u4s10tJtXTu5pfzyUAAAAXSHbmcAAQOE8AAAAEAAAAAgAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAEGQ+AAAAAAAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAABdIduZwABA39QAAAAUAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAEGRtAAAAAAAAAABmh7mXzSj61QjRHc7vbH9HRVWAEGMP7PW2xZjSiHEkgQAAABe/rHpwABA39QAAAAUAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAEGRtAAAAAAAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAABdIduZwABA4TwAAAAQAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAEGRtAAAAAAAAAADfHiHSavMtHaTT2fJqyXUKJ4Jv27izXS0m1dO7ml/PJQAAABbRQVJwABA4TwAAAAQAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.ingestTxMetaToUpdateBalances(paymentResultMeta)
	require.NoError(t, err)
	assert.Equal(t, int64(1_060_0000000), initiatorChannel.localEscrowAccount.Balance)
	assert.Equal(t, int64(950_0000000), initiatorChannel.remoteEscrowAccount.Balance)
}

func TestChannel_IngestTx_updateState_nativeAsset(t *testing.T) {
	initiatorSigner, err := keypair.ParseFull("SCBMAMOPWKL2YHWELK63VLAY2R74A6GTLLD4ON223B7K5KZ37MUR6IDF")
	require.NoError(t, err)
	responderSigner, err := keypair.ParseFull("SBM7D2IIDSRX5Y3VMTMTXXPB6AIB4WYGZBC2M64U742BNOK32X6SW4NF")
	require.NoError(t, err)

	initiatorEscrow, err := keypair.ParseAddress("GAU4CFXQI6HLK5PPY2JWU3GMRJIIQNLF24XRAHX235F7QTG6BEKLGQ36")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GBQNGSEHTFC4YGQ3EXHIL7JQBA6265LFANKFFAYKHM7JFGU5CORROEGO")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(28037546508288),
	}

	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(28054726377472),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})

	// Before confirming an open, channel should not be open.
	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	formationTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	formationTxXDR, err := formationTx.Base64()
	require.NoError(t, err)

	// After confirming an open, but not ingested the formation tx,
	// channel should not be open.
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	// Valid formation transaction xdr.
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	resultMetaXDR := "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)

	// Invalid Result XDR, should return with no state changes.
	invalidResultXDR := "AAAAAAAAAGT////6AAAAAA=="
	err = initiatorChannel.IngestTx(formationTxXDR, invalidResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)

	// Not the formationTx, should return with no state change. The tx is the
	// same seq number as the formationTx, and has identical operation as the
	// formationTx, but has been modified to contain a memo so it is a different
	// tx.
	imposterTx := formationTx.ToXDR()
	imposterTxMemo, err := xdr.NewMemo(xdr.MemoTypeMemoText, "imposter")
	require.NoError(t, err)
	imposterTx.V1.Tx.Memo = imposterTxMemo
	imposterTxBytes, err := imposterTx.MarshalBinary()
	require.NoError(t, err)
	imposterTxXDR := base64.StdEncoding.EncodeToString(imposterTxBytes)
	err = initiatorChannel.IngestTx(imposterTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)
}

func TestChannel_IngestTx_updateState_nonNativeAsset(t *testing.T) {
	initiatorSigner, err := keypair.ParseFull("SBQEQ2SJLI4DKK7T7DYNGAVHDIC2FJSMD2D4HZQTH67Y4YJ2HCIW23E2")
	require.NoError(t, err)
	responderSigner, err := keypair.ParseFull("SD3VHLBEPXOW74B2VLMRSNERLL4HMULIYNLCVLBSYS3ZIFJE5T5VIOBO")
	require.NoError(t, err)

	initiatorEscrow, err := keypair.ParseAddress("GDF7GNJLI6H5ENPPVHRNQF3LN6AT2N2UTXVX57INKELND3DIMROCYXCC")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GBEWOADTWFUS5EKEDB63X5KDWAKBJ32A5WDZKXENOCU3XQTM26GKBV2X")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(24936580120576),
	}

	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(24970939858944),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
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
	})
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	// Open steps.
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	formationTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	formationTxXDR, err := formationTx.Base64()
	require.NoError(t, err)

	// After confirming an open, but not ingested the formation tx,
	// channel should not be open.
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateNone, cs)

	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="

	// Valid formation transaction xdr.
	resultMetaXDR := "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)
	require.NoError(t, initiatorChannel.openExecutedWithError)

	// XDR without updated initiator escrow account should give error.
	resultMetaXDR = "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "could not find an updated ledger entry for both escrow accounts")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong initiator escrow sequence number.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "incorrect initiator escrow account sequence number found, found: 24936580120578 want: 24936580120577")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong signer weights.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "signer not found or incorrect weight")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong signers - extra signer.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "unexpected signer found on escrow account")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong thresholds.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "incorrect initiator escrow account thresholds found")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Missing Trustline.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "trustline not found for asset ABDC:GBW5R35MPDT6JPFRQ3NEHQBMBLX7V6LAPAPPXL6FYQQKNVOCWGV7LKDQ")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)

	// Wrong Trustline flag.
	resultMetaXDR = "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "trustline not authorized")
	cs, err = initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateError, cs)
}

func TestChannel_IngestTx_updateState_invalid_initiatorEscrowHasExtraSigner(t *testing.T) {
	initiatorSigner, err := keypair.ParseFull("SAWFAB3JBDIB3WUW4GDWZJFDH4LYK646PFU2TUTQ2QPIJ7UDPFDALDLJ")
	require.NoError(t, err)
	responderSigner, err := keypair.ParseFull("SDM45WXZOOXEOG23LVWDHBUYTSLZ27YKIN5N3C6QBD3TIIWWQHFFH7FI")
	require.NoError(t, err)

	initiatorEscrow, err := keypair.ParseAddress("GC264CPQA3WZ64USLDCHXG4AFUYGMQXUIW7UY5WYM2QA2WFPS6FARAD4")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GA63LTOE6CXAUGQTQW4332Z6UDBTAN7KTXSJKN4Y5KP4DBJFKEYOHWM7")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(1254130450432),
	}

	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(1271310319616),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})
	open, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		ExpiresAt:                  time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	formationTx, err := responderChannel.OpenTx()
	require.NoError(t, err)
	formationTxXDR, err := formationTx.Base64()
	require.NoError(t, err)

	// Initiator Escrow has an extra signer before the formation tx, should fail.
	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	resultMetaXDR := "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)
	assert.EqualError(t, initiatorChannel.openExecutedWithError, "unexpected signer found on escrow account")
}
