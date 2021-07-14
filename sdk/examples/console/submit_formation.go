package main

import (
	"fmt"

	"github.com/stellar/experimental-payment-channels/sdk/state"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
)

func SubmitFormationTx(channel *state.Channel, client horizonclient.ClientInterface, networkPassphrase string, feeAccount *keypair.FromAddress, feeAccountSigner *keypair.Full) error {
	oa := channel.OpenAgreement()

	// Get formation transaction.
	_, _, formationTx, err := channel.OpenTxs(oa.Details)
	if err != nil {
		return fmt.Errorf("building formation tx: %w", err)
	}

	// Attach signatures to formation	 transaction.
	formationTx, err = formationTx.AddSignatureDecorated(oa.FormationSignatures...)
	if err != nil {
		return fmt.Errorf("adding signatures to the formation tx: %w", err)
	}

	// Submit fee bump transaction that wraps the formation	 transaction.
	err = SubmitFeeBumpTx(client, networkPassphrase, formationTx, feeAccount, feeAccountSigner)
	if err != nil {
		return fmt.Errorf("building fee bump tx: %w", err)
	}

	return nil
}
