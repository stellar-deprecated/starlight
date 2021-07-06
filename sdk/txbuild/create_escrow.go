package txbuild

import (
	"math"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type CreateEscrowParams struct {
	Creator        *keypair.FromAddress
	Escrow         *keypair.FromAddress
	SequenceNumber int64
	Asset          txnbuild.Asset
}

func CreateEscrow(p CreateEscrowParams) (*txnbuild.Transaction, error) {
	ops := []txnbuild.Operation{
		&txnbuild.BeginSponsoringFutureReserves{
			SponsoredID: p.Escrow.Address(),
		},
		&txnbuild.CreateAccount{
			Destination: p.Escrow.Address(),
			// base reserves sponsored by p.Creator
			Amount: "0",
		},
		&txnbuild.SetOptions{
			SourceAccount:   p.Escrow.Address(),
			MasterWeight:    txnbuild.NewThreshold(0),
			LowThreshold:    txnbuild.NewThreshold(1),
			MediumThreshold: txnbuild.NewThreshold(1),
			HighThreshold:   txnbuild.NewThreshold(1),
			Signer:          &txnbuild.Signer{Address: p.Creator.Address(), Weight: 1},
		},
	}
	if !p.Asset.IsNative() {
		ops = append(ops, &txnbuild.ChangeTrust{
			Line:          p.Asset,
			Limit:         amount.StringFromInt64(math.MaxInt64),
			SourceAccount: p.Escrow.Address(),
		})
	}
	ops = append(ops, &txnbuild.EndSponsoringFutureReserves{
		SourceAccount: p.Escrow.Address(),
	})

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: p.Creator.Address(),
				Sequence:  p.SequenceNumber,
			},
			BaseFee:    0,
			Timebounds: txnbuild.NewTimeout(300),
			Operations: ops,
		},
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
