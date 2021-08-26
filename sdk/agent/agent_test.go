package agent

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceNumberCollector func(accountID *keypair.FromAddress) (int64, error)

func (f sequenceNumberCollector) GetSequenceNumber(accountID *keypair.FromAddress) (int64, error) {
	return f(accountID)
}

type balanceCollectorFunc func(accountID *keypair.FromAddress, asset state.Asset) (int64, error)

func (f balanceCollectorFunc) GetBalance(accountID *keypair.FromAddress, asset state.Asset) (int64, error) {
	return f(accountID, asset)
}

type submitterFunc func(tx *txnbuild.Transaction) error

func (f submitterFunc) SubmitTx(tx *txnbuild.Transaction) error {
	return f(tx)
}

func TestAgent_openPaymentClose(t *testing.T) {
	localEscrow := keypair.MustRandom()
	localSigner := keypair.MustRandom()
	remoteEscrow := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()

	// Setup the local agent.
	localVars := struct {
		logs                 strings.Builder
		submittedTx          *txnbuild.Transaction
		err                  error
		connected            bool
		opened               bool
		closed               bool
		lastPaymentAgreement state.CloseAgreement
	}{}
	localAgent := &Agent{
		ObservationPeriodTime:      20 * time.Second,
		ObservationPeriodLedgerGap: 1,
		MaxOpenExpiry:              5 * time.Minute,
		NetworkPassphrase:          network.TestNetworkPassphrase,
		SequenceNumberCollector: sequenceNumberCollector(func(accountID *keypair.FromAddress) (int64, error) {
			return 1, nil
		}),
		BalanceCollector: balanceCollectorFunc(func(accountID *keypair.FromAddress, asset state.Asset) (int64, error) {
			return 100_0000000, nil
		}),
		Submitter: submitterFunc(func(tx *txnbuild.Transaction) error {
			localVars.submittedTx = tx
			return nil
		}),
		EscrowAccountKey:    localEscrow.FromAddress(),
		EscrowAccountSigner: localSigner,
		LogWriter:           &localVars.logs,
		OnError: func(a *Agent, err error) {
			localVars.err = err
		},
		OnConnected: func(a *Agent) {
			localVars.connected = true
		},
		OnOpened: func(a *Agent) {
			localVars.opened = true
		},
		OnPaymentReceivedAndConfirmed: func(a *Agent, ca state.CloseAgreement) {
			localVars.lastPaymentAgreement = ca
		},
		OnPaymentSentAndConfirmed: func(a *Agent, ca state.CloseAgreement) {
			localVars.lastPaymentAgreement = ca
		},
		// TODO: Test when ingestion is added to
		// OnClosing: func(a *Agent) {
		// },
		OnClosed: func(a *Agent) {
			localVars.closed = true
		},
	}

	// Setup the remote agent.
	remoteVars := struct {
		logs                 strings.Builder
		submittedTx          *txnbuild.Transaction
		err                  error
		connected            bool
		opened               bool
		closed               bool
		lastPaymentAgreement state.CloseAgreement
	}{}
	remoteAgent := &Agent{
		ObservationPeriodTime:      20 * time.Second,
		ObservationPeriodLedgerGap: 1,
		MaxOpenExpiry:              5 * time.Minute,
		NetworkPassphrase:          network.TestNetworkPassphrase,
		SequenceNumberCollector: sequenceNumberCollector(func(accountID *keypair.FromAddress) (int64, error) {
			return 1, nil
		}),
		BalanceCollector: balanceCollectorFunc(func(accountID *keypair.FromAddress, asset state.Asset) (int64, error) {
			return 100_0000000, nil
		}),
		Submitter: submitterFunc(func(tx *txnbuild.Transaction) error {
			remoteVars.submittedTx = tx
			return nil
		}),
		EscrowAccountKey:    remoteEscrow.FromAddress(),
		EscrowAccountSigner: remoteSigner,
		LogWriter:           &remoteVars.logs,
		OnError: func(a *Agent, err error) {
			remoteVars.err = err
		},
		OnConnected: func(a *Agent) {
			remoteVars.connected = true
		},
		OnOpened: func(a *Agent) {
			remoteVars.opened = true
		},
		OnPaymentReceivedAndConfirmed: func(a *Agent, ca state.CloseAgreement) {
			remoteVars.lastPaymentAgreement = ca
		},
		OnPaymentSentAndConfirmed: func(a *Agent, ca state.CloseAgreement) {
			remoteVars.lastPaymentAgreement = ca
		},
		// TODO: Test when ingestion is added to
		// OnClosing: func(a *Agent) {
		// },
		OnClosed: func(a *Agent) {
			remoteVars.closed = true
		},
	}

	type ReadWriter struct {
		io.Reader
		io.Writer
	}

	// Connect the two agents.
	localMsgs := bytes.Buffer{}
	remoteMsgs := bytes.Buffer{}
	localAgent.conn = ReadWriter{
		Reader: &remoteMsgs,
		Writer: &localMsgs,
	}
	remoteAgent.conn = ReadWriter{
		Reader: &localMsgs,
		Writer: &remoteMsgs,
	}
	err := localAgent.hello()
	require.NoError(t, err)
	err = remoteAgent.receive()
	require.NoError(t, err)
	err = remoteAgent.hello()
	require.NoError(t, err)
	err = localAgent.receive()
	require.NoError(t, err)

	// Open the channel.
	err = localAgent.Open()
	require.NoError(t, err)
	err = remoteAgent.receive()
	require.NoError(t, err)
	err = localAgent.receive()
	require.NoError(t, err)

	// Make a payment.
	err = localAgent.Payment("50.0")
	require.NoError(t, err)
	err = remoteAgent.receive()
	require.NoError(t, err)
	err = localAgent.receive()
	require.NoError(t, err)

	// Make another payment.
	err = remoteAgent.Payment("20.0")
	require.NoError(t, err)
	err = localAgent.receive()
	require.NoError(t, err)
	err = remoteAgent.receive()
	require.NoError(t, err)

	// Close.
	err = localAgent.DeclareClose()
	require.NoError(t, err)
	err = remoteAgent.receive()
	require.NoError(t, err)
	err = localAgent.receive()
	require.NoError(t, err)

	assert.NotNil(t, localVars.submittedTx)
	assert.NotNil(t, remoteVars.submittedTx)
}
