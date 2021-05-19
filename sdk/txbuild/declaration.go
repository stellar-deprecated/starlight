package txbuild

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func Declaration(initiatorEscrow *keypair.FromAddress, startSequence int64, iterationNumber int64, executedIterationNumber int64) (*txnbuild.Transaction, error) {
	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: initiatorEscrow.Address(),
			Sequence:  startSequenceOfIteration(startSequence, iterationNumber) + 0, // Declaration is the first transaction in an iteration's transaction set.
		},
		BaseFee:           txnbuild.MinBaseFee,
		Timebounds:        txnbuild.NewTimeout(300),
		MinSequenceNumber: int64ptr(startSequenceOfIteration(startSequence, executedIterationNumber)),
		Operations: []txnbuild.Operation{
			&txnbuild.BumpSequence{
				BumpTo: 0,
			},
		},
	}
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
