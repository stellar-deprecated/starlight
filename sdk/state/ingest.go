package state

import (
	"fmt"

	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// IngestTx accepts any transaction that has been seen as successful or
// unsuccessful on the network. The function updates the internal state of the
// channel if the transaction relates to the channel.
func (c *Channel) IngestTx(tx *txnbuild.Transaction, resultXDR string, resultMetaXDR string) error {
	// TODO: Use the transaction result to affect on success/failure.

	c.ingestTxToUpdateInitiatorEscrowAccountSequence(tx)

	err := c.ingestTxToUpdateUnauthorizedCloseAgreement(tx)
	if err != nil {
		return err
	}

	err = c.ingestTxMetaToUpdateBalances(resultMetaXDR)
	if err != nil {
		return err
	}

	err = c.ingestFormationTx(tx, resultXDR, resultMetaXDR)
	if err != nil {
		return err
	}

	return nil
}

func (c *Channel) ingestTxToUpdateInitiatorEscrowAccountSequence(tx *txnbuild.Transaction) {
	// If the transaction's source account is not the initiator's escrow
	// account, return.
	if tx.SourceAccount().AccountID != c.initiatorEscrowAccount().Address.Address() {
		return
	}

	c.setInitiatorEscrowAccountSequence(tx.SourceAccount().Sequence)
}

// ingestTxToUpdateUnauthorizedCloseAgreement uses the signatures in the transaction to
// authorize an unauthorized close agreement if the channel has one.
// This process helps to give a participant who proposed an agreement the
// ability to close the channel if they did not receive the confirmers
// signatures for a close agreement when the agreement was being negotiated. If
// the transaction cannot be used to do this the function returns a nil error.
// If the transaction should be able to provide this data and cannot, the
// function errors.
func (c *Channel) ingestTxToUpdateUnauthorizedCloseAgreement(tx *txnbuild.Transaction) error {
	// If the transaction's source account is not the initiator's escrow
	// account, then the transaction is not a part of a close agreement.
	if tx.SourceAccount().AccountID != c.initiatorEscrowAccount().Address.Address() {
		return nil
	}

	ca := c.latestUnauthorizedCloseAgreement

	// If there is no unauthorized close agreement, then there's no need to try
	// and update it.
	if ca.isEmpty() {
		return nil
	}

	declTx, closeTx, err := c.closeTxs(c.openAgreement.Details, ca.Details)
	if err != nil {
		return fmt.Errorf("building txs for latest unauthorized close agreement: %w", err)
	}

	// Compare the hash of the tx with the hash of the declaration tx from the
	// latest unauthorized close agreement. If they match, then the tx is the
	// declaration tx.
	declTxHash, err := declTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized declaration tx: %w", err)
	}
	txHash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing tx: %w", err)
	}
	if txHash != declTxHash {
		return nil
	}

	closeTxHash, err := closeTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized close tx: %w", err)
	}

	// Look for the signatures on the tx that are required to fully authorize
	// the unauthorized close agreement, then confirm the close agreement.
	for _, sig := range tx.Signatures() {
		err = c.remoteSigner.Verify(declTxHash[:], sig.Signature)
		if err == nil {
			ca.ConfirmerSignatures.Declaration = sig.Signature
			break
		}
	}
	for _, sig := range tx.Signatures() {
		err = c.remoteSigner.Verify(closeTxHash[:], sig.Signature)
		if err == nil {
			ca.ConfirmerSignatures.Close = sig.Signature
			break
		}
	}
	_, err = c.ConfirmPayment(ca)
	if err != nil {
		return fmt.Errorf("confirming the last unauthorized close: %w", err)
	}

	return nil
}

// ingestTxMetaToUpdateBalances uses the transaction result meta data
// from a transaction response to update local and remote escrow account
// balances.
func (c *Channel) ingestTxMetaToUpdateBalances(resultMetaXDR string) error {
	// If not a valid resultMetaXDR string, return.
	var txMeta xdr.TransactionMeta
	err := xdr.SafeUnmarshalBase64(resultMetaXDR, &txMeta)
	if err != nil {
		return fmt.Errorf("parsing the result meta xdr: %w", err)
	}

	channelAsset := c.openAgreement.Details.Asset

	// Find ledger changes for the escrow accounts' balances,
	// if any, and then update.
	for _, o := range txMeta.V2.Operations {
		for _, change := range o.Changes {
			updated, ok := change.GetUpdated()
			if !ok {
				continue
			}

			var ledgerEntryAddress string
			var ledgerEntryBalance int64

			if channelAsset.IsNative() {
				account, ok := updated.Data.GetAccount()
				if !ok {
					continue
				}
				ledgerEntryAddress = account.AccountId.Address()
				ledgerEntryBalance = int64(account.Balance)
			} else {
				tl, ok := updated.Data.GetTrustLine()
				if !ok {
					continue
				}
				if string(channelAsset) != tl.Asset.StringCanonical() {
					continue
				}
				ledgerEntryAddress = tl.AccountId.Address()
				ledgerEntryBalance = int64(tl.Balance)
			}

			switch ledgerEntryAddress {
			case c.localEscrowAccount.Address.Address():
				c.UpdateLocalEscrowAccountBalance(ledgerEntryBalance)
			case c.remoteEscrowAccount.Address.Address():
				c.UpdateRemoteEscrowAccountBalance(ledgerEntryBalance)
			}
		}
	}
	return nil
}

// ingestFormationTx accepts a transaction, resultXDR, and resultMetaXDR. The
// method returns with no error if either 1. the resultXDR shows the transaction
// was unsuccessful, or 2. the transaction is not the formation transaction this
// channel is expecting, the method returns with no error. Lastly, this method
// will validate that the resulting account and trustlines after the formation
// transaction was submitted are in this channel's expected states to mark the
// channel as open.
func (c *Channel) ingestFormationTx(tx *txnbuild.Transaction, resultXDR string, resultMetaXDR string) (err error) {
	// If the transaction is not the formation transaction, ignore.
	formationTx, err := c.OpenTx()
	if err != nil {
		return fmt.Errorf("creating formation tx: %w", err)
	}
	txHash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("getting transaction hash: %w", err)
	}
	formationHash, err := formationTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("getting transaction hash: %w", err)
	}
	if txHash != formationHash {
		return nil
	}

	// If the transaction was not successful, return.
	var txResult xdr.TransactionResult
	err = xdr.SafeUnmarshalBase64(resultXDR, &txResult)
	if err != nil {
		return fmt.Errorf("parsing the result xdr: %w", err)
	}
	if !txResult.Successful() {
		return nil
	}

	const requiredSignerWeight = 1
	const requiredNumOfSigners = 2
	const requiredThresholds = requiredNumOfSigners * requiredSignerWeight

	// If not a valid resultMetaXDR string, return error.
	var txMeta xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(resultMetaXDR, &txMeta)
	if err != nil {
		return fmt.Errorf("parsing the result meta xdr: %w", err)
	}
	txMetaV2, ok := txMeta.GetV2()
	if !ok {
		return fmt.Errorf("TransationMetaV2 not available")
	}

	// Find escrow account ledger changes. Grabs the latest entry, which gives
	// the latest ledger entry state.
	var initiatorEscrowAccountEntry, responderEscrowAccountEntry *xdr.AccountEntry
	var initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry *xdr.TrustLineEntry
	for _, o := range txMetaV2.Operations {
		for _, change := range o.Changes {
			updated, ok := change.GetUpdated()
			if !ok {
				continue
			}

			switch updated.Data.Type {
			case xdr.LedgerEntryTypeTrustline:
				if updated.Data.TrustLine.Asset.StringCanonical() != string(c.openAgreement.Details.Asset) {
					continue
				}
				if updated.Data.TrustLine.AccountId.Address() == c.initiatorEscrowAccount().Address.Address() {
					initiatorEscrowTrustlineEntry = updated.Data.TrustLine
				} else if updated.Data.TrustLine.AccountId.Address() == c.responderEscrowAccount().Address.Address() {
					responderEscrowTrustlineEntry = updated.Data.TrustLine
				}
			case xdr.LedgerEntryTypeAccount:
				if updated.Data.Account.AccountId.Address() == c.initiatorEscrowAccount().Address.Address() {
					initiatorEscrowAccountEntry = updated.Data.Account
				} else if updated.Data.Account.AccountId.Address() == c.responderEscrowAccount().Address.Address() {
					responderEscrowAccountEntry = updated.Data.Account
				}
			}
		}
	}

	// If initiator escrow account not found, return.
	if initiatorEscrowAccountEntry == nil {
		return nil
	}

	// Validate the initiator escrow account sequence number is correct.
	if int64(initiatorEscrowAccountEntry.SeqNum) != c.startingSequence {
		c.openExecutedWithError = fmt.Errorf("incorrect initiator escrow account sequence number found, found: %d want: %d",
			int64(initiatorEscrowAccountEntry.SeqNum), c.startingSequence)
		return nil
	}

	escrowAccounts := [2]*xdr.AccountEntry{initiatorEscrowAccountEntry, responderEscrowAccountEntry}
	for _, ea := range escrowAccounts {
		// Validate the escrow account thresholds are equal to the number of
		// signers so that all signers are required to sign all transactions.
		// Thresholds are: Master Key, Low, Medium, High.
		if ea.Thresholds != (xdr.Thresholds{0, requiredThresholds, requiredThresholds, requiredThresholds}) {
			c.openExecutedWithError = fmt.Errorf("incorrect initiator escrow account thresholds found")
			return nil
		}

		// Validate the escrow account has the correct signers and signer weights.
		var initiatorSignerCorrect, responderSignerCorrect bool
		for _, signer := range ea.Signers {
			address, err := signer.Key.GetAddress()
			if err != nil {
				c.openExecutedWithError = fmt.Errorf("parsing formation transaction escrow account signer keys: %w", err)
				return nil
			}

			if address == c.initiatorSigner().Address() {
				initiatorSignerCorrect = signer.Weight == requiredSignerWeight
			} else if address == c.responderSigner().Address() {
				responderSignerCorrect = signer.Weight == requiredSignerWeight
			} else {
				c.openExecutedWithError = fmt.Errorf("non channel participant signer found")
				return nil
			}
		}
		if !initiatorSignerCorrect || !responderSignerCorrect {
			c.openExecutedWithError = fmt.Errorf("signer not found or incorrect weight")
			return nil
		}
	}

	// Validate the required trustlines are correct for a non-native channel.
	if !c.openAgreement.Details.Asset.IsNative() {
		trustlineEntries := [2]*xdr.TrustLineEntry{initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry}
		for _, te := range trustlineEntries {
			// Validate trustline exists.
			if te == nil {
				c.openExecutedWithError = fmt.Errorf("trustline not found for nonnative asset channel")
				return nil
			}

			// Validate trustline is authorized.
			if te.Flags != xdr.Uint32(xdr.TrustLineFlagsAuthorizedFlag) {
				c.openExecutedWithError = fmt.Errorf("incorrect trustline flag, needs to be authorized")
				return nil
			}
		}
	}

	// Update the initiator escrow sequence number on the channel.
	// TODO - combine with ingestTxToUpdateInitiatorEscrowAccountSequence so we're updating in one spot.
	c.setInitiatorEscrowAccountSequence(int64(initiatorEscrowAccountEntry.SeqNum))

	c.openExecutedAndValidated = true
	return nil
}
