package state

import (
	"fmt"
	"time"

	"github.com/stellar/go/keypair"
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

func (c *Channel) OpenTx() (formationTx *txnbuild.Transaction, err error) {
	openAgreement := c.OpenAgreement()
	formationTx, err = c.openTxs(openAgreement.Details)
	if err != nil {
		return nil, fmt.Errorf("building declaration and close txs for latest close agreement: %w", err)
	}
	formationTx, err = formationTx.AddSignatureDecorated(openAgreement.FormationSignatures...)
	if err != nil {
		return nil, fmt.Errorf("attaching signatures to formation tx for latest close agreement: %w", err)
	}
	return
}

func (c *Channel) LatestCloseAgreement() CloseAgreement {
	return c.latestAuthorizedCloseAgreement
}

func (c *Channel) CloseTxs() (declTx *txnbuild.Transaction, closeTx *txnbuild.Transaction, err error) {
	closeAgreement := c.LatestCloseAgreement()
	declTx, closeTx, err = c.closeTxs(c.openAgreement.Details, closeAgreement.Details)
	if err != nil {
		return nil, nil, fmt.Errorf("building declaration and close txs for latest close agreement: %w", err)
	}
	declTx, err = declTx.AddSignatureDecorated(closeAgreement.DeclarationSignatures...)
	if err != nil {
		return nil, nil, fmt.Errorf("attaching signatures to declaration tx for latest close agreement: %w", err)
	}
	closeTx, err = closeTx.AddSignatureDecorated(closeAgreement.CloseSignatures...)
	if err != nil {
		return nil, nil, fmt.Errorf("attaching signatures to close tx for latest close agreement: %w", err)
	}
	return
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

func (c *Channel) verifySigned(tx *txnbuild.Transaction, sigs []xdr.DecoratedSignature, signer keypair.KP) (bool, error) {
	hash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return false, err
	}
	for _, sig := range sigs {
		if sig.Hint != signer.Hint() {
			continue
		}
		err := signer.Verify(hash[:], sig.Signature)
		if err == nil {
			return true, nil
		}
	}
	return false, nil
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

func appendNewSignatures(oldSignatures []xdr.DecoratedSignature, newSignatures []xdr.DecoratedSignature) []xdr.DecoratedSignature {
	m := make(map[string]bool)
	for _, os := range oldSignatures {
		m[string(os.Signature)] = true
	}

	for _, ns := range newSignatures {
		if !m[string(ns.Signature)] {
			oldSignatures = append(oldSignatures, ns)
		}
	}
	return oldSignatures
}
