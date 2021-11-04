package txbuild

import (
	"fmt"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type DeclarationParams struct {
	InitiatorMultiSig       *keypair.FromAddress
	StartSequence           int64
	IterationNumber         int64
	IterationNumberExecuted int64
	CloseTxHash             [32]byte
	ConfirmingSigner        *keypair.FromAddress
}

func Declaration(p DeclarationParams) (*txnbuild.Transaction, error) {
	if p.IterationNumber < 0 || p.StartSequence <= 0 {
		return nil, fmt.Errorf("invalid iteration number or start sequence: cannot be negative")
	}

	// Declaration is the first transaction in an iteration's transaction set.
	seq := startSequenceOfIteration(p.StartSequence, p.IterationNumber) + 0
	if seq < 0 {
		return nil, fmt.Errorf("invalid sequence number: cannot be negative")
	}

	minSequenceNumber := startSequenceOfIteration(p.StartSequence, p.IterationNumberExecuted)

	// Build the extra signature required for signing the declaration
	// transaction that will be required in addition to the signers for the
	// account signers. The extra signer will be a signature by the confirming
	// signer for the close transaction so that the confirming signer must
	// reveal that signature publicly when submitting the declaration
	// transaction. This prevents the confirming signer from withholding
	// signatures for the closing transactions.
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
			AccountID: p.InitiatorMultiSig.Address(),
			Sequence:  seq,
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
