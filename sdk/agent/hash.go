package agent

import (
	"fmt"

	"github.com/stellar/go/txnbuild"
)

func hashTx(txXDR, networkPassphrase string) (string, error) {
	tx, err := txnbuild.TransactionFromXDR(txXDR)
	if err != nil {
		return "", fmt.Errorf("parsing transaction xdr: %w", err)
	}
	if feeBump, ok := tx.FeeBump(); ok {
		hash, err := feeBump.HashHex(networkPassphrase)
		if err != nil {
			return "", fmt.Errorf("hashing fee bump tx: %w", err)
		}
		return hash, nil
	}
	if transaction, ok := tx.Transaction(); ok {
		hash, err := transaction.HashHex(networkPassphrase)
		if err != nil {
			return "", fmt.Errorf("hashing tx: %w", err)
		}
		return hash, nil
	}
	return "", fmt.Errorf("transaction unrecognized")
}
