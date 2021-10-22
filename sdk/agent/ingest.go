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
		err = fmt.Errorf("ingesting tx (cursor=%s): hashing tx: %w", tx.Cursor, err)
		a.events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.logWriter, "ingesting cursor: %s tx: %s\n", tx.Cursor, txHash)

	stateBefore, err := a.channel.State()
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): getting channel state before: %w", tx.Cursor, txHash, err)
		a.events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.logWriter, "state before: %v\n", stateBefore)

	defer a.takeSnapshot()

	err = a.channel.IngestTx(tx.TransactionOrderID, tx.TransactionXDR, tx.ResultXDR, tx.ResultMetaXDR)
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): ingesting xdr: %w", tx.Cursor, txHash, err)
		a.events <- ErrorEvent{Err: err}
		return err
	}

	stateAfter, err := a.channel.State()
	if err != nil {
		err = fmt.Errorf("ingesting tx (cursor=%s hash=%s): getting channel state after: %w", tx.Cursor, txHash, err)
		a.events <- ErrorEvent{Err: err}
		return err
	}
	fmt.Fprintf(a.logWriter, "state after: %v\n", stateAfter)

	if a.events != nil {
		if stateAfter != stateBefore {
			fmt.Fprintf(a.logWriter, "writing event: %v\n", stateAfter)
			switch stateAfter {
			case state.StateOpen:
				a.events <- OpenedEvent{a.channel.OpenAgreement()}
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
