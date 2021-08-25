package state

import (
	"fmt"

	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// IngestTx accepts any transaction that has been seen as successful or
// unsuccessful on the network. The function updates the internal state of the
// channel if the transaction relates to the channel.
func (c *Channel) IngestTx(tx *txnbuild.Transaction, resultMetaXDR string) error {
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

	err = c.ingestFormationTx(resultMetaXDR)
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

func (c *Channel) ingestFormationTx(resultMetaXDR string) (err error) {
	const requiredNumOfSigners = 2

	// If not a valid resultMetaXDR string, return error.
	var txMeta xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(resultMetaXDR, &txMeta)
	if err != nil {
		return fmt.Errorf("parsing the result meta xdr: %w", err)
	}

	// Find escrow account ledger changes. Grabs the latest entry, which gives
	// the latest ledger entry state.
	var initiatorEscrowAccountEntry, responderEscrowAccountEntry *xdr.AccountEntry
	var initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry *xdr.TrustLineEntry
	for _, o := range txMeta.V2.Operations {
		for _, change := range o.Changes {
			updated, ok := change.GetUpdated()
			if !ok {
				continue
			}

			switch updated.Data.Type {
			case xdr.LedgerEntryTypeTrustline:
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
		// Validate the escrow account thresholds are correct.
		if ea.Thresholds != xdr.Thresholds([4]byte{0, requiredNumOfSigners, requiredNumOfSigners, requiredNumOfSigners}) {
			c.openExecutedWithError = fmt.Errorf("incorrect initiator escrow account thresholds found")
			return nil
		}

		// Validate the escrow account has the correct signers and signer weights.
		var initiatorSignerFound, responderSignerFound bool
		for _, signer := range ea.Signers {
			address, err := signer.Key.GetAddress()
			if err != nil {
				c.openExecutedWithError = fmt.Errorf("parsing formation transaction escrow account signer keys: %w", err)
				return nil
			}

			if address == c.initiatorSigner().Address() && signer.Weight == xdr.Uint32(1) {
				initiatorSignerFound = true
			} else if address == c.responderSigner().Address() && signer.Weight == xdr.Uint32(1) {
				responderSignerFound = true
			}
		}
		if !initiatorSignerFound || !responderSignerFound {
			c.openExecutedWithError = fmt.Errorf("incorrect signer weights found")
			return nil
		}
		if len(ea.Signers) != requiredNumOfSigners {
			c.openExecutedWithError = fmt.Errorf("incorrect number of signers, found: %d want: %d", len(ea.Signers), requiredNumOfSigners)
			return nil
		}
	}

	// Validate the trustlines are correct.
	if c.openAgreement.Details.Asset.IsNative() {
		if initiatorEscrowTrustlineEntry != nil || responderEscrowTrustlineEntry != nil {
			c.openExecutedWithError = fmt.Errorf("extraneous trustline found for native asset channel")
			return nil
		}
	} else {
		trustlineEntries := [2]*xdr.TrustLineEntry{initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry}
		for _, te := range trustlineEntries {
			// Validate correct asset.
			if string(c.openAgreement.Details.Asset) != te.Asset.StringCanonical() {
				c.openExecutedWithError = fmt.Errorf("incorrect trustline asset found for nonnative asset channel")
				return nil
			}

			// Validate trustline is authorized.
			if te.Flags != xdr.Uint32(1) {
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
