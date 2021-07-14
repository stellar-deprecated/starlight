package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
)

func SubmitDeclarationTx(channel *state.Channel, client horizonclient.ClientInterface, networkPassphrase string, feeAccount *keypair.FromAddress, feeAccountSigner *keypair.Full) error {
	ca := channel.LatestCloseAgreement()

	// Get declaration transaction.
	declTx, _, err := channel.CloseTxs(ca.Details)
	if err != nil {
		return fmt.Errorf("building declaration tx: %w", err)
	}

	// Attach signatures to declaration transaction.
	declTx, err = declTx.AddSignatureDecorated(ca.DeclarationSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the declaration tx: %w", err)
	}

	// Submit fee bump transaction that wraps the declaration transaction.
	err = SubmitFeeBumpTx(client, networkPassphrase, declTx, feeAccount, feeAccountSigner)
	if err != nil {
		return fmt.Errorf("building fee bump tx: %w", err)
	}

	return nil
}
