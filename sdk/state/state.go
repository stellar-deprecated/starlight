package state

import (
	"fmt"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type EscrowAccount struct {
	Address        *keypair.FromAddress
	SequenceNumber int64
	Balance        int64
}

type Channel struct {
	networkPassphrase string
	maxOpenExpiry     time.Duration

	startingSequence int64
	// TODO - leave execution out for now
	// iterationNumberExecuted int64

	initiator           bool
	localEscrowAccount  *EscrowAccount
	remoteEscrowAccount *EscrowAccount

	localSigner  *keypair.Full
	remoteSigner *keypair.FromAddress

	openAgreement OpenAgreement

	latestAuthorizedCloseAgreement   CloseAgreement
	latestUnauthorizedCloseAgreement CloseAgreement
}

type Config struct {
	NetworkPassphrase string
	MaxOpenExpiry     time.Duration

	Initiator bool

	LocalEscrowAccount  *EscrowAccount
	RemoteEscrowAccount *EscrowAccount

	LocalSigner  *keypair.Full
	RemoteSigner *keypair.FromAddress
}

func NewChannel(c Config) *Channel {
	channel := &Channel{
		networkPassphrase:   c.NetworkPassphrase,
		maxOpenExpiry:       c.MaxOpenExpiry,
		initiator:           c.Initiator,
		localEscrowAccount:  c.LocalEscrowAccount,
		remoteEscrowAccount: c.RemoteEscrowAccount,
		localSigner:         c.LocalSigner,
		remoteSigner:        c.RemoteSigner,
	}
	return channel
}

func (c *Channel) IsInitiator() bool {
	return c.initiator
}

func (c *Channel) NextIterationNumber() int64 {
	if !c.latestUnauthorizedCloseAgreement.isEmpty() {
		return c.latestUnauthorizedCloseAgreement.Details.IterationNumber
	}
	return c.latestAuthorizedCloseAgreement.Details.IterationNumber + 1
}

// Balance returns the amount owing from the initiator to the responder, if positive, or
// the amount owing from the responder to the initiator, if negative.
func (c *Channel) Balance() int64 {
	return c.latestAuthorizedCloseAgreement.Details.Balance
}

func (c *Channel) OpenAgreement() OpenAgreement {
	return c.openAgreement
}

func (c *Channel) LatestCloseAgreement() CloseAgreement {
	return c.latestAuthorizedCloseAgreement
}

func (c *Channel) UpdateLocalEscrowAccountBalance(balance int64) {
	c.localEscrowAccount.Balance = balance
}

func (c *Channel) UpdateRemoteEscrowAccountBalance(balance int64) {
	c.remoteEscrowAccount.Balance = balance
}

func (c *Channel) LocalEscrowAccount() EscrowAccount {
	return *c.localEscrowAccount
}

func (c *Channel) RemoteEscrowAccount() EscrowAccount {
	return *c.remoteEscrowAccount
}

// IngestTx accepts any transaction that has been seen as successful or
// unsuccessful on the network. The function updates the internal state of the
// channel if the transaction relates to the channel.
//
// TODO: Signal when the state of the channel has changed to closed or closing.
// TODO: Accept the xdr.TransactionResult and xdr.TransactionMeta so code can
// determine if successful or not, and understand changes in the ledger as a
// result.
func (c *Channel) IngestTx(tx *txnbuild.Transaction) error {
	// If the tx hash matches an unauthorized decl, copy the close signature.
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

func (c *Channel) ingestTxToUpdateUnauthorizedCloseAgreement(tx *txnbuild.Transaction) error {
	if tx.SourceAccount().AccountID != c.initiatorEscrowAccount().Address.Address() {
		return nil
	}
	ca := c.latestUnauthorizedCloseAgreement
	if ca.isEmpty() {
		return nil
	}
	txHash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing tx: %w", err)
	}
	declTx, closeTx, err := c.closeTxs(c.openAgreement.Details, ca.Details)
	if err != nil {
		return fmt.Errorf("building txs for latest unauthorized close agreement: %w", err)
	}
	declTxHash, err := declTx.Hash(c.networkPassphrase)
	if err != nil {
		return fmt.Errorf("hashing latest unauthorized declaration tx: %w", err)
	}
	if txHash != declTxHash {
		return nil
	}
	if err != nil {
		return fmt.Errorf("building txs for latest unauthorized close agreement: %w", err)
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

func (c *Channel) initiatorEscrowAccount() *EscrowAccount {
	if c.initiator {
		return c.localEscrowAccount
	} else {
		return c.remoteEscrowAccount
	}
}

func (c *Channel) responderEscrowAccount() *EscrowAccount {
	if c.initiator {
		return c.remoteEscrowAccount
	} else {
		return c.localEscrowAccount
	}
}

func (c *Channel) initiatorSigner() *keypair.FromAddress {
	if c.initiator {
		return c.localSigner.FromAddress()
	} else {
		return c.remoteSigner
	}
}

func (c *Channel) responderSigner() *keypair.FromAddress {
	if c.initiator {
		return c.remoteSigner
	} else {
		return c.localSigner.FromAddress()
	}
}

func (c *Channel) amountToLocal(balance int64) int64 {
	if c.initiator {
		return amountToInitiator(balance)
	}
	return amountToResponder(balance)
}

func (c *Channel) amountToRemote(balance int64) int64 {
	if c.initiator {
		return amountToResponder(balance)
	}
	return amountToInitiator(balance)
}

func amountToInitiator(balance int64) int64 {
	if balance < 0 {
		return balance * -1
	}
	return 0
}

func amountToResponder(balance int64) int64 {
	if balance > 0 {
		return balance
	}
	return 0
}

func signTx(tx *txnbuild.Transaction, networkPassphrase string, kp *keypair.Full) (xdr.Signature, error) {
	h, err := network.HashTransactionInEnvelope(tx.ToXDR(), networkPassphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to hash transaction: %w", err)
	}
	sig, err := kp.Sign(h[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction hash: %w", err)
	}
	return xdr.Signature(sig), nil
}

func verifySigned(tx *txnbuild.Transaction, networkPassphrase string, signer keypair.KP, sig xdr.Signature) error {
	hash, err := tx.Hash(networkPassphrase)
	if err != nil {
		return err
	}
	err = signer.Verify(hash[:], []byte(sig))
	if err != nil {
		return err
	}
	return nil
}
