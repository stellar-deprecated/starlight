package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
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

	// Fields that the test will write to when the agent triggers events or
	// logs.
	var (
		logs                      strings.Builder
		submittedTx               *txnbuild.Transaction
		err                       error
		connected, opened, closed bool
		lastPaymentAgreement      state.CloseAgreement
	)

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
			submittedTx = tx
			return nil
		}),
		EscrowAccountKey:    localEscrow.FromAddress(),
		EscrowAccountSigner: localSigner,
		LogWriter:           &logs,
		OnError: func(a *agent.Agent, e error) {
			err = e
		},
		OnConnected: func(a *agent.Agent) {
			connected = true
		},
		OnOpened: func(a *agent.Agent) {
			opened = true
		},
		OnPaymentReceivedAndConfirmed: func(a *agent.Agent, ca state.CloseAgreement) {
			lastPaymentAgreement = ca
		},
		OnPaymentSentAndConfirmed: func(a *agent.Agent, ca state.CloseAgreement) {
			lastPaymentAgreement = ca
		},
		// TODO: Test when ingestion is added to agent.
		// OnClosing: func(a *agent.Agent) {
		// },
		OnClosed: func(a *agent.Agent) {
			closed = true
		},
	}

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
			submittedTx = tx
			return nil
		}),
		EscrowAccountKey:    localEscrow.FromAddress(),
		EscrowAccountSigner: localSigner,
		LogWriter:           &logs,
		OnError: func(a *agent.Agent, e error) {
			err = e
		},
		OnConnected: func(a *agent.Agent) {
			connected = true
		},
		OnOpened: func(a *agent.Agent) {
			opened = true
		},
		OnPaymentReceivedAndConfirmed: func(a *agent.Agent, ca state.CloseAgreement) {
			lastPaymentAgreement = ca
		},
		OnPaymentSentAndConfirmed: func(a *agent.Agent, ca state.CloseAgreement) {
			lastPaymentAgreement = ca
		},
		// TODO: Test when ingestion is added to agent.
		// OnClosing: func(a *agent.Agent) {
		// },
		OnClosed: func(a *agent.Agent) {
			closed = true
		},
	}

	// TODO

}
