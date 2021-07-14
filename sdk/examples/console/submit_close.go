package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
)

func SubmitCloseTx(channel *state.Channel, client horizonclient.ClientInterface, networkPassphrase string, feeAccount *keypair.FromAddress, feeAccountSigner *keypair.Full) error {
	ca := channel.LatestCloseAgreement()

	// Get close transaction.
	_, closeTx, err := channel.CloseTxs(ca.Details)
	if err != nil {
		return fmt.Errorf("building close tx: %w", err)
	}

	// Attach signatures to close transaction.
	closeTx, err = closeTx.AddSignatureDecorated(ca.CloseSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the close tx: %w", err)
	}

	// Submit fee bump transaction that wraps the close transaction.
	err = SubmitFeeBumpTx(client, networkPassphrase, closeTx, feeAccount, feeAccountSigner)
	if err != nil {
		return fmt.Errorf("building fee bump tx: %w", err)
	}

	return nil
}
