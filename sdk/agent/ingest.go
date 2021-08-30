package agent

import (
	"errors"
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
)

var ingestingFinished = errors.New("ingesting finished")

func (a *Agent) ingest() error {
	tx, ok := <-a.transactionStreamerTransactions
	if !ok {
		return ingestingFinished
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	stateBefore, err := a.channel.State()
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("ingesting tx: %s result: %s result meta: %s: %w", tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR, err)
	}

	err = a.channel.IngestTx(tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR)
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("ingesting tx: %s result: %s result meta: %s: %w", tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR, err)
	}

	stateAfter, err := a.channel.State()
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("ingesting tx: %s result: %s result meta: %s: %w", tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR, err)
	}

	if a.Events != nil {
		if stateAfter != stateBefore {
			switch stateAfter {
			case state.StateOpen:
				a.Events <- OpenedEvent{}
			case state.StateClosing:
				a.Events <- ClosingEvent{}
			case state.StateClosingWithOutdatedState:
				a.Events <- ClosingWithOutdatedStateEvent{}
			case state.StateClosed:
				a.Events <- ClosedEvent{}
			}
		}
	}

	return nil
}

func (a *Agent) ingestLoop() {
	for {
		err := a.ingest()
		if err != nil {
			fmt.Fprintf(a.LogWriter, "error ingesting: %v\n", err)
		}
		if errors.Is(err, ingestingFinished) {
			break
		}
	}
}
