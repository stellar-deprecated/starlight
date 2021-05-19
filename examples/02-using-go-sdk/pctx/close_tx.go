package pctx

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func BuildCloseTx(initiator *keypair.FromAddress, responder *keypair.FromAddress, ei *keypair.FromAddress, er *keypair.FromAddress, s int64, i int, amountToInitiator int64, amountToResponder int64) (*txnbuild.Transaction, error) {
	var err error

	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: ei.Address(),
			Sequence:  s_i(s, i) + 1,
		},
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		MinSequenceAge:       int64(observationPeriodTime.Seconds()),
		MinSequenceLedgerGap: int64(observationPeriodLedgerGap),
		Operations: []txnbuild.Operation{
			&txnbuild.SetOptions{
				SourceAccount:   ei.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: responder.Address(), Weight: 0},
			},
			&txnbuild.SetOptions{
				SourceAccount:   er.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: initiator.Address(), Weight: 0},
			},
		},
	}
	if amountToInitiator != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: er.Address(),
			Destination:   ei.Address(),
			Asset:         txnbuild.NativeAsset{},
			Amount:        amount.StringFromInt64(amountToInitiator),
		})
	}
	if amountToResponder != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: ei.Address(),
			Destination:   er.Address(),
			Asset:         txnbuild.NativeAsset{},
			Amount:        amount.StringFromInt64(amountToResponder),
		})
	}
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
