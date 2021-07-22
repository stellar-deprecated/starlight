package txbuild

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type DeclarationParams struct {
	InitiatorEscrow         *keypair.FromAddress
	StartSequence           int64
	IterationNumber         int64
	IterationNumberExecuted int64
	CloseTxHash             [32]byte
	ConfirmingSigner        *keypair.FromAddress
}

func Declaration(p DeclarationParams) (*txnbuild.Transaction, error) {
	minSequenceNumber := startSequenceOfIteration(p.StartSequence, p.IterationNumberExecuted)

	extraSignerKey := xdr.SignerKey{}
	err := extraSignerKey.SetSignedPayload(p.ConfirmingSigner.Address(), p.CloseTxHash[:])
	if err != nil {
		return nil, err
	}
	extraSigner, err := extraSignerKey.GetAddress()
	if err != nil {
		return nil, err
	}

	tp := txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: p.InitiatorEscrow.Address(),
			Sequence:  startSequenceOfIteration(p.StartSequence, p.IterationNumber) + 0, // Declaration is the first transaction in an iteration's transaction set.
		},
		BaseFee:           0,
		Timebounds:        txnbuild.NewInfiniteTimeout(),
		MinSequenceNumber: &minSequenceNumber,
		ExtraSigners: []string{
			extraSigner,
		},
		Operations: []txnbuild.Operation{
			&txnbuild.BumpSequence{
				BumpTo: 0,
			},
		},
	}
	tx, err := txnbuild.NewTransaction(tp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
