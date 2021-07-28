package state

import (
	"fmt"

	"github.com/stellar/go/txnbuild"
)

// IngestTx accepts any transaction that has been seen as successful or
// unsuccessful on the network. The function updates the internal state of the
// channel if the transaction relates to the channel.
//
// TODO: Signal when the state of the channel has changed to closed or closing.
// TODO: Accept the xdr.TransactionResult and xdr.TransactionMeta so code can
// determine if successful or not, and understand changes in the ledger as a
// result.
func (c *Channel) IngestTx(tx *txnbuild.Transaction) error {
	// Use the tx to update the latest unauthorized close agreement if possible.
	err := c.ingestTxToUpdateUnauthorizedCloseAgreement(tx)
	if err != nil {
		return err
	}

	// TODO: If the tx hash matches an authorized or unauthorized declaration,
	// mark the channel as closing.
	// TODO: If the tx hash matches an authorized or unauthorized close, mark the
	// channel as closed.
	// TODO: If the tx is for an older declaration, mark the channel as closing with
	// requiring bump.
	// TODO: Use the transaction result to affect on success/failure.

	return nil
}

// ingestTxToUpdateUnauthorizedCloseAgreement uses the signatures in the
// transaction to authorize an unauthorized close agreement if the channel has
// one. This process helps to give a participant who proposed an agreement the
// ability to close the channel if they did not receive the confirmers
// signatures for a close agreement when the agreement was being negotiated. If
// the transaction cannot be used to do this the function returns a nil error.
// If the transaction should be able to provide this data and cannot, the
// function errors.
func (c *Channel) ingestTxToUpdateUnauthorizedCloseAgreement(tx *txnbuild.Transaction) error {
	// If the transaction's source account is not the initiator's escrow
	// account, then the transaction will not be a transaction for a close
	// agreement and won't be able to update any unauthorized close agreement.
	if tx.SourceAccount().AccountID != c.initiatorEscrowAccount().Address.Address() {
		return nil
	}

	ca := c.latestUnauthorizedCloseAgreement

	// If there is no unauthorized close agreement, then there's no need to try
	// and update it.
	if ca.isEmpty() {
		return nil
	}

	// Load the declaration and close transactions for the unauthorized close
	// agreement.
	declTx, closeTx, err := c.closeTxs(c.openAgreement.Details, ca.Details)
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
	declTxHash, err := declTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized declaration tx: %w", err)
	}
	if txHash != declTxHash {
		return nil
	}

	// Look for the signatures on the tx that are required to fully authorize
	// the unauthorized close agreement, then confirm the close agreement.
	closeTxHash, err := closeTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized close tx: %w", err)
	}
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
