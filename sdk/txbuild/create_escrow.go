package txbuild

import (
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func CreateEscrow(creator *keypair.Full, escrow *keypair.Full, sequenceNumber int64, initialContribution int64) (*txnbuild.Transaction, error) {
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: creator.Address(),
				Sequence:  sequenceNumber,
			},
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.BeginSponsoringFutureReserves{
					SponsoredID: escrow.Address(),
				},
				&txnbuild.CreateAccount{
					Destination: escrow.Address(),
					Amount:      amount.StringFromInt64(initialContribution),
				},
				&txnbuild.SetOptions{
					SourceAccount:   escrow.Address(),
					MasterWeight:    txnbuild.NewThreshold(0),
					LowThreshold:    txnbuild.NewThreshold(1),
					MediumThreshold: txnbuild.NewThreshold(1),
					HighThreshold:   txnbuild.NewThreshold(1),
					Signer:          &txnbuild.Signer{Address: creator.Address(), Weight: 1},
				},
				&txnbuild.EndSponsoringFutureReserves{
					SourceAccount: escrow.Address(),
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
