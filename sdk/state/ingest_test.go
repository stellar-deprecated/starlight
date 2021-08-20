package state

import (
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// note: transaction result meta xdr strings for these tests were generated
// with Protocol 17.

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
	err = initiatorChannel.IngestTx(declTx, placeholderXDR)
	require.NoError(t, err)
	closeState, err := initiatorChannel.CloseState()
	require.NoError(t, err)
	require.Equal(t, CloseStateClosing, closeState)

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

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	// Pretend responder broadcasts latest declTx, and
	// initiator ingests it.
	declTx, _, err := responderChannel.CloseTxs()
	require.NoError(t, err)
	err = initiatorChannel.IngestTx(declTx, placeholderXDR)
	require.NoError(t, err)
	closeState, err := initiatorChannel.CloseState()
	require.NoError(t, err)
	require.Equal(t, CloseStateClosing, closeState)
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

	// To prevent xdr parsing error.
	placeholderXDR := "AAAAAgAAAAIAAAADABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAAKAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABABArWwAAAAAAAAAAWPnYf+6kQN3t44vgesQdWh4JOOPj7aer852I7RJhtzAAAAAWg8TZOwANrPwAAAALAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMAD/39AAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABdjSVwcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAAD49aUpVx7fhJPK6wDdlPJgkA1HkAi85qUL1tii8YSZzQAAABee5CYcAA/8sgAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABaDxNk7AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAEAECtbAAAAAAAAAABY+dh/7qRA3e3ji+B6xB1aHgk44+Ptp6vznYjtEmG3MAAAABZIKg87AA2s/AAAAAsAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA="

	oldDeclTx, _, err := responderChannel.CloseTxs()
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
	err = initiatorChannel.IngestTx(oldDeclTx, placeholderXDR)
	require.NoError(t, err)
	closeState, err := initiatorChannel.CloseState()
	require.NoError(t, err)
	require.Equal(t, CloseStateClosingWithOutdatedState, closeState)
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
