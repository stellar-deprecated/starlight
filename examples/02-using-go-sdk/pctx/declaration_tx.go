package pctx

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func BuildDeclarationTx(ei *keypair.FromAddress, s int64, i int, e int) (*txnbuild.Transaction, error) {
	var err error

	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: ei.Address(),
			Sequence:  s_i(s, i),
		},
		BaseFee:           txnbuild.MinBaseFee,
		Timebounds:        txnbuild.NewTimeout(300),
		MinSequenceNumber: &[]int64{s_e(s, e)}[0],
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
