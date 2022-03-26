package txbuild

import (
	"math"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type CreateChannelAccountParams struct {
	Creator        *keypair.FromAddress
	ChannelAccount *keypair.FromAddress
	SequenceNumber int64
	Asset          txnbuild.BasicAsset
}

func CreateChannelAccount(p CreateChannelAccountParams) (*txnbuild.Transaction, error) {
	ops := []txnbuild.Operation{
		&txnbuild.BeginSponsoringFutureReserves{
			SponsoredID: p.ChannelAccount.Address(),
		},
		&txnbuild.CreateAccount{
			Destination: p.ChannelAccount.Address(),
			// base reserves sponsored by p.Creator
			Amount: "0",
		},
		&txnbuild.SetOptions{
			SourceAccount:   p.ChannelAccount.Address(),
			MasterWeight:    txnbuild.NewThreshold(0),
			LowThreshold:    txnbuild.NewThreshold(1),
			MediumThreshold: txnbuild.NewThreshold(1),
			HighThreshold:   txnbuild.NewThreshold(1),
			Signer:          &txnbuild.Signer{Address: p.Creator.Address(), Weight: 1},
		},
	}
	if !p.Asset.IsNative() {
		ops = append(ops, &txnbuild.ChangeTrust{
			Line:          p.Asset.MustToChangeTrustAsset(),
			Limit:         amount.StringFromInt64(math.MaxInt64),
			SourceAccount: p.ChannelAccount.Address(),
		})
	}
	ops = append(ops, &txnbuild.EndSponsoringFutureReserves{
		SourceAccount: p.ChannelAccount.Address(),
	})

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: p.Creator.Address(),
				Sequence:  p.SequenceNumber,
			},
			BaseFee: 0,
			Preconditions: txnbuild.Preconditions{
				Timebounds: txnbuild.NewTimeout(300),
			},
			Operations: ops,
		},
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
