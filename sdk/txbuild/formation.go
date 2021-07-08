package txbuild

import (
	"math"
	"time"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type FormationParams struct {
	InitiatorSigner *keypair.FromAddress
	ResponderSigner *keypair.FromAddress
	InitiatorEscrow *keypair.FromAddress
	ResponderEscrow *keypair.FromAddress
	StartSequence   int64
	Asset           txnbuild.Asset
	ExpiresAt       time.Time
}

func Formation(p FormationParams) (*txnbuild.Transaction, error) {
	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: p.InitiatorEscrow.Address(),
			Sequence:  p.StartSequence,
		},
		BaseFee:    0,
		Timebounds: txnbuild.NewTimebounds(0, p.ExpiresAt.Unix()),
	}

	// I sponsoring ledger entries on EI
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: p.InitiatorSigner.Address(), SponsoredID: p.InitiatorEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   p.InitiatorEscrow.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: p.InitiatorSigner.Address(), Weight: 1},
	})
	if !p.Asset.IsNative() {
		tp.Operations = append(tp.Operations, &txnbuild.ChangeTrust{
			Line:          p.Asset,
			Limit:         amount.StringFromInt64(math.MaxInt64),
			SourceAccount: p.InitiatorEscrow.Address(),
		})
	}
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: p.InitiatorEscrow.Address()})

	// I sponsoring ledger entries on ER
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: p.InitiatorSigner.Address(), SponsoredID: p.ResponderEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount: p.ResponderEscrow.Address(),
		Signer:        &txnbuild.Signer{Address: p.InitiatorSigner.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: p.ResponderEscrow.Address()})

	// R sponsoring ledger entries on ER
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: p.ResponderSigner.Address(), SponsoredID: p.ResponderEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount:   p.ResponderEscrow.Address(),
		MasterWeight:    txnbuild.NewThreshold(0),
		LowThreshold:    txnbuild.NewThreshold(2),
		MediumThreshold: txnbuild.NewThreshold(2),
		HighThreshold:   txnbuild.NewThreshold(2),
		Signer:          &txnbuild.Signer{Address: p.ResponderSigner.Address(), Weight: 1},
	})
	if !p.Asset.IsNative() {
		tp.Operations = append(tp.Operations, &txnbuild.ChangeTrust{
			Line:          p.Asset,
			Limit:         amount.StringFromInt64(math.MaxInt64),
			SourceAccount: p.ResponderEscrow.Address(),
		})
	}
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: p.ResponderEscrow.Address()})

	// R sponsoring ledger entries on EI
	tp.Operations = append(tp.Operations, &txnbuild.BeginSponsoringFutureReserves{SourceAccount: p.ResponderSigner.Address(), SponsoredID: p.InitiatorEscrow.Address()})
	tp.Operations = append(tp.Operations, &txnbuild.SetOptions{
		SourceAccount: p.InitiatorEscrow.Address(),
		Signer:        &txnbuild.Signer{Address: p.ResponderSigner.Address(), Weight: 1},
	})
	tp.Operations = append(tp.Operations, &txnbuild.EndSponsoringFutureReserves{SourceAccount: p.InitiatorEscrow.Address()})

	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
