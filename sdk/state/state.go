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

type CloseState int

const (
	CloseStateError CloseState = iota - 1
	CloseStateNone
	CloseStateEarlyClosing // a proposed declTx before fully confirmed is submitted
	CloseStateClosing      // latest declTx is submitted
	CloseStateNeedsClosing // an earlier declTx is submitted
	CloseStateClosed
)

// CloseState infers the close state by comparing the initator escrow sequence
// number and the close agreement sequence numbers.
func (c *Channel) CloseState() (CloseState, error) {
	// Get the sequence numbers for the latest close agreement transactions.
	declTxAuthorized, closeTxAuthorized, err := c.closeTxs(c.openAgreement.Details, c.latestAuthorizedCloseAgreement.Details)
	if err != nil {
		return CloseStateError, fmt.Errorf("building declaration and close txs for latest authorized close agreement: %w", err)
	}
	latestDeclSequence := declTxAuthorized.SequenceNumber()
	latestCloseSequence := closeTxAuthorized.SequenceNumber()

	// Compare with the initiator escrow account.
	switch c.initiatorEscrowAccount().SequenceNumber {
	case c.startingSequence:
		return CloseStateNone, nil
	case latestDeclSequence:
		return CloseStateClosing, nil
	case latestCloseSequence:
		return CloseStateClosed, nil
	}

	// Compare the initiator escrow account sequence with the latest unauthorized
	// close agreement.
	if !c.latestUnauthorizedCloseAgreement.isEmpty() {
		declTxUnauthorized, _, err := c.closeTxs(c.openAgreement.Details, c.latestUnauthorizedCloseAgreement.Details)
		if err != nil {
			return CloseStateError, fmt.Errorf("building declaration and close txs for latest unauthorized close agreement: %w", err)
		}
		if c.initiatorEscrowAccount().SequenceNumber == declTxUnauthorized.SequenceNumber() {
			return CloseStateEarlyClosing, nil
		}
	}

	// See if in between the startingSequence and the latest unauthorized close
	// agreement, indicating an early close agreement has been submitted.
	if c.initiatorEscrowAccount().SequenceNumber > c.startingSequence && c.initiatorEscrowAccount().SequenceNumber < latestDeclSequence {
		return CloseStateNeedsClosing, nil
	}

	return CloseStateError, fmt.Errorf("initiator escrow account sequence has unexpected value")
}

func (c *Channel) setInitiatorEscrowAccountSequence(seqNum int64) {
	if c.initiator {
		c.localEscrowAccount.SequenceNumber = seqNum
	} else {
		c.remoteEscrowAccount.SequenceNumber = seqNum
	}
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
