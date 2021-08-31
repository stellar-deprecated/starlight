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
		err = fmt.Errorf("ingesting tx (cursor=%s): hashing tx: %w", tx.Cursor, err)
		a.Events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.LogWriter, "ingesting cursor: %s tx: %s\n", tx.Cursor, txHash)

	stateBefore, err := a.channel.State()
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): getting channel state before: %w", tx.Cursor, txHash, err)
		a.Events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.LogWriter, "state before: %v\n", stateBefore)

	err = a.channel.IngestTx(tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR)
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): ingesting xdr: %w", tx.Cursor, txHash, err)
		a.Events <- ErrorEvent{Err: err}
		return err
	}

	stateAfter, err := a.channel.State()
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): getting channel state after: %w", tx.Cursor, txHash, err)
		a.Events <- ErrorEvent{Err: err}
		return err
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
				a.streamerCancel()
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
