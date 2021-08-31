package agent

import (
	"errors"
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
)

var ingestingFinished = errors.New("ingesting finished")

func (a *Agent) ingest() error {
	tx, ok := <-a.streamerTransactions
	if !ok {
		return ingestingFinished
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	txHash, err := hashTx(tx.TransactionXDR, a.networkPassphrase)
	if err != nil {
		a.events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.logWriter, "ingesting cursor: %s tx: %s\n", tx.Cursor, txHash)

	stateBefore, err := a.channel.State()
	if err != nil {
		a.events <- ErrorEvent{Err: err}
		return fmt.Errorf("getting state: %w", err)
	}

	fmt.Fprintf(a.logWriter, "state before: %v\n", stateBefore)

	err = a.channel.IngestTx(tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR)
	if err != nil {
		a.events <- ErrorEvent{Err: err}
		return fmt.Errorf("ingesting tx: %s result: %s result meta: %s: %w", tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR, err)
	}

	stateAfter, err := a.channel.State()
	if err != nil {
		a.events <- ErrorEvent{Err: err}
		return fmt.Errorf("getting state after ingesting tx: %w", err)
	}

	fmt.Fprintf(a.logWriter, "state after: %v\n", stateAfter)

	if a.events != nil {
		if stateAfter != stateBefore {
			fmt.Fprintf(a.logWriter, "triggering event: %v\n", stateAfter)
			switch stateAfter {
			case state.StateOpen:
				a.events <- OpenedEvent{}
			case state.StateClosing:
				a.events <- ClosingEvent{}
			case state.StateClosingWithOutdatedState:
				a.events <- ClosingWithOutdatedStateEvent{}
			case state.StateClosed:
				a.streamerCancel()
				a.events <- ClosedEvent{}
			}
		}
	}

	return nil
}

func (a *Agent) ingestLoop() {
	for {
		err := a.ingest()
		if err != nil {
			fmt.Fprintf(a.logWriter, "error ingesting: %v\n", err)
		}
		if errors.Is(err, ingestingFinished) {
			break
		}
	}
}
