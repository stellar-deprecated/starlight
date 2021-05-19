package txbuild

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func Formation(initiator *keypair.FromAddress, responder *keypair.FromAddress, initiatorEscrow *keypair.FromAddress, responderEscrow *keypair.FromAddress, startSequence int64) (*txnbuild.Transaction, error) {
	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: initiatorEscrow.Address(),
			Sequence:  startSequence,
		},
		BaseFee:    txnbuild.MinBaseFee,
		Timebounds: txnbuild.NewTimeout(300),
	}
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: initiator.Address(), SponsoredID: initiatorEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   initiatorEscrow.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: responder.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: initiatorEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: responder.Address(), SponsoredID: responderEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   responderEscrow.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: initiator.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: responderEscrow.Address()})
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
