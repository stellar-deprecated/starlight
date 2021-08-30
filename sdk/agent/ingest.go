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

	txHash, err := hashTx(tx.TransactionXDR, a.NetworkPassphrase)
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.LogWriter, "ingesting tx: %s\n", txHash)

	stateBefore, err := a.channel.State()
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("getting state: %w", err)
	}

	fmt.Fprintf(a.LogWriter, "state before: %v\n", stateBefore)

	err = a.channel.IngestTx(tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR)
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("ingesting tx: %s result: %s result meta: %s: %w", tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR, err)
	}

	stateAfter, err := a.channel.State()
	if err != nil {
		a.Events <- ErrorEvent{Err: err}
		return fmt.Errorf("getting state after ingesting tx: %w: %w", err)
	}

	fmt.Fprintf(a.LogWriter, "state after: %v\n", stateAfter)

	if a.Events != nil {
		if stateAfter != stateBefore {
			fmt.Fprintf(a.LogWriter, "triggering event: %v\n", stateAfter)
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
