package txbuild

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type CreateEscrowParams struct {
	Creator             *keypair.FromAddress
	Escrow              *keypair.FromAddress
	SequenceNumber      int64
	InitialContribution int64
}

func CreateEscrow(p CreateEscrowParams) (*txnbuild.Transaction, error) {
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: p.Creator.Address(),
				Sequence:  p.SequenceNumber,
			},
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.BeginSponsoringFutureReserves{
					SponsoredID: p.Escrow.Address(),
				},
				&txnbuild.CreateAccount{
					Destination: p.Escrow.Address(),
					Amount:      amount.StringFromInt64(p.InitialContribution),
				},
				&txnbuild.SetOptions{
					SourceAccount:   p.Escrow.Address(),
					MasterWeight:    txnbuild.NewThreshold(0),
					LowThreshold:    txnbuild.NewThreshold(1),
					MediumThreshold: txnbuild.NewThreshold(1),
					HighThreshold:   txnbuild.NewThreshold(1),
					Signer:          &txnbuild.Signer{Address: p.Creator.Address(), Weight: 1},
				},
				&txnbuild.EndSponsoringFutureReserves{
					SourceAccount: p.Escrow.Address(),
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
