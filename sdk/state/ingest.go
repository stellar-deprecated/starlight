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
	// TODO: Use the transaction result to affect on success/failure.

	err := c.ingestTxToUpdateCloseState(tx)
	if err != nil {
		return err
	}

	err = c.ingestTxToUpdateBalances(tx)
	if err != nil {
		return err
	}

	return nil
}

func (c *Channel) ingestTxToUpdateCloseState(tx *txnbuild.Transaction) error {
	// If the transaction's source account is not the initiator's escrow
	// account, then the transaction is not a part of a close agreement.
	if tx.SourceAccount().AccountID != c.initiatorEscrowAccount().Address.Address() {
		return fmt.Errorf(`transaction source account is not the initiator escrow account,
			found: %s, should be: %s`, tx.SourceAccount().AccountID, c.initiatorEscrowAccount().Address.Address())
	}

	c.setNetworkEscrowSequence(tx.SourceAccount().Sequence)

	// If we found an unauthorized close agreement has begun closing, update our unauthorized to
	// become authorized.
	if c.CloseState() == CloseEarlyClosing {
		err := c.updateUnauthorizedAgreement(tx)
		if err != nil {
			return err
		}
		if c.CloseState() != CloseClosing {
			return fmt.Errorf("converting unauthorized agreement to authorized should put channel in closing state")
		}
	}

	return nil
}

// updateUnauthorizedAgreement uses the signatures in the transaction to
// authorize an unauthorized close agreement if the channel has one.
// This process helps to give a participant who proposed an agreement the
// ability to close the channel if they did not receive the confirmers
// signatures for a close agreement when the agreement was being negotiated. If
// the transaction cannot be used to do this the function returns a nil error.
// If the transaction should be able to provide this data and cannot, the
// function errors.
func (c *Channel) updateUnauthorizedAgreement(tx *txnbuild.Transaction) error {
	if c.latestUnauthorizedCloseAgreement.isEmpty() {
		return fmt.Errorf("unauthorized close agreement is empty")
	}

	ca := c.latestUnauthorizedCloseAgreement

	declTx, closeTx, err := c.closeTxs(c.openAgreement.Details, c.latestUnauthorizedCloseAgreement.Details)
	if err != nil {
		return fmt.Errorf("building txs for latest unauthorized close agreement: %w", err)
	}

	// Look for the signatures on the tx that are required to fully authorize
	// the unauthorized close agreement, then confirm the close agreement.
	declTxHash, err := declTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized declaration tx: %w", err)
	}
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

func (c *Channel) ingestTxToUpdateBalances(tx *txnbuild.Transaction) error {
	// TODO - validations
	// If transaction is not a payment transaction, return.
	// If transaction is not sending from one of the escrow accounts, return.
	// If transaction is not sending to one of the escrow accounts, return.
	fmt.Printf("%+v\n", tx)

	localEscrowAddress := c.localEscrowAccount.Address.Address()
	remoteEscrowAddress := c.remoteEscrowAccount.Address.Address()

	txSourceIsLocal := tx.SourceAccount().Address == localEscrowAddress
	txSourceIsRemote := tx.SourceAccount().Address == remoteEscrowAddress

	for _, op := range tx.Operations() {
		if paymentOp, ok := op.(*txnbuild.Payment); ok {
			fmt.Printf("operation: %+v\n", paymentOp)
			// check withdrawals
			if paymentOp.SourceAccount == localEscrowAddress ||
				paymentOp.SourceAccount == nil && txSourceIsLocal {
				// update local for withdrawal
				continue
			}

			if paymentOp.SourceAccount == remoteEscrowAddress ||
				paymentOp.SourceAccount == nil && txSourceIsRemote {
				// update remote for withdrawal
			}

			// check deposits
			if paymentOp.Destination == localEscrowAddress {
				// update local escrow for deposit
				continue
			}
			if paymentOp.Destination == remoteEscrowAddress {
				// update remote escrow for deposit
				continue
			}
		}

	}

	return nil
}
