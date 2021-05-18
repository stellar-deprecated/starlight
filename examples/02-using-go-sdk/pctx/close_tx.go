package pctx

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func BuildCloseTx(ei *keypair.FromAddress, er *keypair.FromAddress, s int64, i int, amountToInitiator string, amountToResponder string) (*txnbuild.Transaction, error) {
	var err error

	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: ei.Address(),
			Sequence:  s_i(s, i)+1,
		},
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		MinSequenceAge:       int64(observationPeriodTime.Seconds()),
		MinSequenceLedgerGap: int64(observationPeriodLedgerGap),
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				SourceAccount: er.Address(),
				Destination:   ei.Address(),
				Asset:         txnbuild.NativeAsset{},
				Amount:        amountToInitiator,
			},
			&txnbuild.Payment{
				SourceAccount: ei.Address(),
				Destination:   er.Address(),
				Asset:         txnbuild.NativeAsset{},
				Amount:        amountToResponder,
			},
		},
	}
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
