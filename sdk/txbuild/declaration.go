package txbuild

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type DeclarationParams struct {
	InitiatorEscrow         *keypair.FromAddress
	StartSequence           int64
	IterationNumber         int64
	IterationNumberExecuted int64
}

func Declaration(p DeclarationParams) (*txnbuild.Transaction, error) {
	minSequenceNumber := startSequenceOfIteration(p.StartSequence, p.IterationNumberExecuted)
	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: p.InitiatorEscrow.Address(),
			Sequence:  startSequenceOfIteration(p.StartSequence, p.IterationNumber) + 0, // Declaration is the first transaction in an iteration's transaction set.
		},
		BaseFee:           0,
		Timebounds:        txnbuild.NewInfiniteTimeout(),
		MinSequenceNumber: &minSequenceNumber,
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
