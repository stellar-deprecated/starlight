package state

import (
	"fmt"

	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// IngestTx accepts any transaction that has been seen as successful or
// unsuccessful on the network. The function updates the internal state of the
// channel if the transaction relates to one of the channel's escrow accounts.
//
// The txOrderID is an identifier that orders transactions as they were
// executed on the Stellar network.
//
// The function may be called with transactions for each escrow account out of
// order. For example, transactions for the initiator escrow account can be
// processed in order, and transactions for the responder escrow account can be
// processed in order, but relative to each other they may be out of order. If
// transactions for a single account are processed out of order some state
// transition may be skipped.
//
// The function maybe called with duplicate transactions and duplicates will not
// change the state of the channel.
func (c *Channel) IngestTx(txOrderID int64, txXDR, resultXDR, resultMetaXDR string) error {
	// If channel has not been opened or has been closed, return.
	if c.OpenAgreement().Envelope.Empty() {
		return fmt.Errorf("channel has not been opened")
	}
	cs, err := c.State()
	if err != nil {
		return fmt.Errorf("getting channel state: %w", err)
	}
	if cs == StateClosed || cs == StateClosedWithOutdatedState {
		return fmt.Errorf("channel has been closed")
	}

	// Get transaction object from the transaction XDR.
	gtx, err := txnbuild.TransactionFromXDR(txXDR)
	if err != nil {
		return fmt.Errorf("parsing transaction xdr")
	}
	var tx *txnbuild.Transaction
	if feeBump, ok := gtx.FeeBump(); ok {
		tx = feeBump.InnerTransaction()
	}
	if transaction, ok := gtx.Transaction(); ok {
		tx = transaction
	}
	if tx == nil {
		return fmt.Errorf("transaction unrecognized")
	}

	// Ingest the transaction and update channel state if valid.
	c.ingestTxToUpdateInitiatorEscrowAccountSequence(tx)

	err = c.ingestTxToUpdateUnauthorizedCloseAgreement(tx)
	if err != nil {
		return err
	}

	err = c.ingestTxMetaToUpdateBalances(txOrderID, resultMetaXDR)
	if err != nil {
		return err
	}

	err = c.ingestOpenTx(tx, resultXDR, resultMetaXDR)
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

	// If the transaction is from an earlier sequence number, return.
	if tx.SourceAccount().Sequence <= c.initiatorEscrowAccount().SequenceNumber {
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

	ce := c.latestUnauthorizedCloseAgreement.Envelope

	// If there is no unauthorized close agreement, then there's no need to try
	// and update it.
	if ce.Empty() {
		return nil
	}

	txs, err := c.closeTxs(c.openAgreement.Envelope.Details, ce.Details)
	if err != nil {
		return fmt.Errorf("building txs for latest unauthorized close agreement: %w", err)
	}

	// Compare the hash of the tx with the hash of the declaration tx from the
	// latest unauthorized close agreement. If they match, then the tx is the
	// declaration tx.
	txHash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing tx: %w", err)
	}
	if txHash != txs.DeclarationHash {
		return nil
	}

	// Look for the signatures on the tx that are required to fully authorize
	// the unauthorized close agreement, then confirm the close agreement.
	for _, sig := range tx.Signatures() {
		err = c.remoteSigner.Verify(txs.DeclarationHash[:], sig.Signature)
		if err == nil {
			ce.ConfirmerSignatures.Declaration = sig.Signature
			break
		}
	}
	for _, sig := range tx.Signatures() {
		err = c.remoteSigner.Verify(txs.CloseHash[:], sig.Signature)
		if err == nil {
			ce.ConfirmerSignatures.Close = sig.Signature
			break
		}
	}
	_, err = c.ConfirmPayment(ce)
	if err != nil {
		return fmt.Errorf("confirming the last unauthorized close: %w", err)
	}

	return nil
}

// ingestTxMetaToUpdateBalances uses the transaction result meta data
// from a transaction response to update local and remote escrow account
// balances.
func (c *Channel) ingestTxMetaToUpdateBalances(txOrderID int64, resultMetaXDR string) error {
	// If not a valid resultMetaXDR string, return.
	var txMeta xdr.TransactionMeta
	err := xdr.SafeUnmarshalBase64(resultMetaXDR, &txMeta)
	if err != nil {
		return fmt.Errorf("parsing the result meta xdr: %w", err)
	}

	channelAsset := c.openAgreement.Envelope.Details.Asset

	// Find ledger changes for the escrow accounts' balances,
	// if any, and then update.
	for _, o := range txMeta.V2.Operations {
		for _, change := range o.Changes {
			var entry *xdr.LedgerEntry
			switch change.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				entry = change.Created
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				entry = change.Updated
			default:
				continue
			}

			var ledgerEntryAddress string
			var ledgerEntryAvailableBalance int64

			if channelAsset.IsNative() {
				account, ok := entry.Data.GetAccount()
				if !ok {
					continue
				}
				ledgerEntryAddress = account.AccountId.Address()
				liabilities := account.Liabilities()
				ledgerEntryAvailableBalance = int64(account.Balance - liabilities.Buying)
			} else {
				tl, ok := entry.Data.GetTrustLine()
				if !ok {
					continue
				}
				if !channelAsset.EqualTrustLineAsset(tl.Asset) {
					continue
				}
				ledgerEntryAddress = tl.AccountId.Address()
				liabilities := tl.Liabilities()
				ledgerEntryAvailableBalance = int64(tl.Balance - liabilities.Selling)
			}

			switch ledgerEntryAddress {
			case c.localEscrowAccount.Address.Address():
				if txOrderID > c.localEscrowAccount.LastSeenTransactionOrderID {
					c.UpdateLocalEscrowAccountBalance(ledgerEntryAvailableBalance)
					c.localEscrowAccount.LastSeenTransactionOrderID = txOrderID
				}
			case c.remoteEscrowAccount.Address.Address():
				if txOrderID > c.remoteEscrowAccount.LastSeenTransactionOrderID {
					c.UpdateRemoteEscrowAccountBalance(ledgerEntryAvailableBalance)
					c.remoteEscrowAccount.LastSeenTransactionOrderID = txOrderID
				}
			}
		}
	}
	return nil
}

// ingestOpenTx accepts a transaction, resultXDR, and resultMetaXDR. The
// method returns with no error if either 1. the resultXDR shows the transaction
// was unsuccessful, or 2. the transaction is not the open transaction this
// channel is expecting, the method returns with no error. Lastly, this method
// will validate that the resulting account and trustlines after the open
// transaction was submitted are in this channel's expected states to mark the
// channel as open.
func (c *Channel) ingestOpenTx(tx *txnbuild.Transaction, resultXDR string, resultMetaXDR string) (err error) {
	// If the transaction is not the open transaction, ignore.
	openTx, err := c.OpenTx()
	if err != nil {
		return fmt.Errorf("creating open tx: %w", err)
	}
	txHash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("getting transaction hash: %w", err)
	}
	openHash, err := openTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("getting transaction hash: %w", err)
	}
	if txHash != openHash {
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
		return fmt.Errorf("result meta version unrecognized")
	}

	// Find escrow account ledger changes. Grabs the latest entry, which gives
	// the latest ledger entry state.
	var initiatorEscrowAccountEntry, responderEscrowAccountEntry *xdr.AccountEntry
	var initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry *xdr.TrustLineEntry
	for _, o := range txMetaV2.Operations {
		for _, change := range o.Changes {
			var entry *xdr.LedgerEntry
			switch change.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				entry = change.Created
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				entry = change.Updated
			default:
				continue
			}

			switch entry.Data.Type {
			case xdr.LedgerEntryTypeTrustline:
				if !c.openAgreement.Envelope.Details.Asset.EqualTrustLineAsset(entry.Data.TrustLine.Asset) {
					continue
				}
				if entry.Data.TrustLine.AccountId.Address() == c.initiatorEscrowAccount().Address.Address() {
					initiatorEscrowTrustlineEntry = entry.Data.TrustLine
				} else if entry.Data.TrustLine.AccountId.Address() == c.responderEscrowAccount().Address.Address() {
					responderEscrowTrustlineEntry = entry.Data.TrustLine
				}
			case xdr.LedgerEntryTypeAccount:
				if entry.Data.Account.AccountId.Address() == c.initiatorEscrowAccount().Address.Address() {
					initiatorEscrowAccountEntry = entry.Data.Account
				} else if entry.Data.Account.AccountId.Address() == c.responderEscrowAccount().Address.Address() {
					responderEscrowAccountEntry = entry.Data.Account
				}
			}
		}
	}

	// Validate both escrow accounts have been updated.
	if initiatorEscrowAccountEntry == nil || responderEscrowAccountEntry == nil {
		c.openExecutedWithError = fmt.Errorf("could not find an updated ledger entry for both escrow accounts")
		return nil
	}

	// Validate the initiator escrow account sequence number is correct.
	if int64(initiatorEscrowAccountEntry.SeqNum) != openTx.SequenceNumber() {
		c.openExecutedWithError = fmt.Errorf("incorrect initiator escrow account sequence number found, found: %d want: %d",
			int64(initiatorEscrowAccountEntry.SeqNum), openTx.SequenceNumber())
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
				c.openExecutedWithError = fmt.Errorf("parsing open transaction escrow account signer keys: %w", err)
				return nil
			}

			if address == c.initiatorSigner().Address() {
				initiatorSignerCorrect = signer.Weight == requiredSignerWeight
			} else if address == c.responderSigner().Address() {
				responderSignerCorrect = signer.Weight == requiredSignerWeight
			} else {
				c.openExecutedWithError = fmt.Errorf("unexpected signer found on escrow account")
				return nil
			}
		}
		if !initiatorSignerCorrect || !responderSignerCorrect {
			c.openExecutedWithError = fmt.Errorf("signer not found or incorrect weight")
			return nil
		}
	}

	// Validate the required trustlines are correct for a non-native channel.
	if !c.openAgreement.Envelope.Details.Asset.IsNative() {
		trustlineEntries := [2]*xdr.TrustLineEntry{initiatorEscrowTrustlineEntry, responderEscrowTrustlineEntry}
		for _, te := range trustlineEntries {
			// Validate trustline exists.
			if te == nil {
				c.openExecutedWithError = fmt.Errorf("trustline not found for asset %v", c.openAgreement.Envelope.Details.Asset)
				return nil
			}

			// Validate trustline is authorized.
			if !xdr.TrustLineFlags(te.Flags).IsAuthorized() {
				c.openExecutedWithError = fmt.Errorf("trustline not authorized")
				return nil
			}
		}
	}

	c.openExecutedAndValidated = true
	return nil
}
