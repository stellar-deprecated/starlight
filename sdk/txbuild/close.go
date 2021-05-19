package txbuild

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func Close(initiatorSigner *keypair.FromAddress, responderSigner *keypair.FromAddress, initiatorEscrow *keypair.FromAddress, responderEscrow *keypair.FromAddress, startSequence int64, iterationNumber int64, amountToInitiator int64, amountToResponder int64) (*txnbuild.Transaction, error) {
	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: initiatorEscrow.Address(),
			Sequence:  startSequenceOfIteration(startSequence, iterationNumber) + 1, // Close is the second transaction in an iteration's transaction set.
		},
		BaseFee:              txnbuild.MinBaseFee,
		Timebounds:           txnbuild.NewTimeout(300),
		MinSequenceAge:       int64(observationPeriodTime.Seconds()),
		MinSequenceLedgerGap: int64(observationPeriodLedgerGap),
		Operations: []txnbuild.Operation{
			&txnbuild.SetOptions{
				SourceAccount:   initiatorEscrow.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: responderSigner.Address(), Weight: 0},
			},
			&txnbuild.SetOptions{
				SourceAccount:   responderEscrow.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: initiatorSigner.Address(), Weight: 0},
			},
		},
	}
	if amountToInitiator != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: responderEscrow.Address(),
			Destination:   initiatorEscrow.Address(),
			Asset:         txnbuild.NativeAsset{},
			Amount:        amount.StringFromInt64(amountToInitiator),
		})
	}
	if amountToResponder != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: initiatorEscrow.Address(),
			Destination:   responderEscrow.Address(),
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
