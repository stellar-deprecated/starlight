package txbuild

import (
	"fmt"
	"time"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type CloseParams struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap uint32
	InitiatorSigner            *keypair.FromAddress
	ResponderSigner            *keypair.FromAddress
	InitiatorChannelAccount    *keypair.FromAddress
	ResponderChannelAccount    *keypair.FromAddress
	StartSequence              int64
	IterationNumber            int64
	AmountToInitiator          int64
	AmountToResponder          int64
	Asset                      txnbuild.Asset
}

func Close(p CloseParams) (*txnbuild.Transaction, error) {
	if p.IterationNumber < 0 || p.StartSequence <= 0 {
		return nil, fmt.Errorf("invalid iteration number or start sequence: cannot be negative")
	}

	// Close is the second transaction in an iteration's transaction set.
	seq := startSequenceOfIteration(p.StartSequence, p.IterationNumber) + 1
	if seq < 0 {
		return nil, fmt.Errorf("invalid sequence number: cannot be negative")
	}

	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: p.InitiatorChannelAccount.Address(),
			Sequence:  seq,
		},
		BaseFee: 0,
		Preconditions: txnbuild.Preconditions{
			TimeBounds:                 txnbuild.NewInfiniteTimeout(),
			MinSequenceNumberAge:       uint64(p.ObservationPeriodTime.Seconds()),
			MinSequenceNumberLedgerGap: p.ObservationPeriodLedgerGap,
		},
		Operations: []txnbuild.Operation{
			&txnbuild.SetOptions{
				SourceAccount:   p.InitiatorChannelAccount.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: p.ResponderSigner.Address(), Weight: 0},
			},
			&txnbuild.SetOptions{
				SourceAccount:   p.ResponderChannelAccount.Address(),
				MasterWeight:    txnbuild.NewThreshold(0),
				LowThreshold:    txnbuild.NewThreshold(1),
				MediumThreshold: txnbuild.NewThreshold(1),
				HighThreshold:   txnbuild.NewThreshold(1),
				Signer:          &txnbuild.Signer{Address: p.InitiatorSigner.Address(), Weight: 0},
			},
		},
	}
	if p.AmountToInitiator != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: p.ResponderChannelAccount.Address(),
			Destination:   p.InitiatorChannelAccount.Address(),
			Asset:         p.Asset,
			Amount:        amount.StringFromInt64(p.AmountToInitiator),
		})
	}
	if p.AmountToResponder != 0 {
		tp.Operations = append(tp.Operations, &txnbuild.Payment{
			SourceAccount: p.InitiatorChannelAccount.Address(),
			Destination:   p.ResponderChannelAccount.Address(),
			Asset:         p.Asset,
			Amount:        amount.StringFromInt64(p.AmountToResponder),
		})
	}
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
