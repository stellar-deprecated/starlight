package state

import (
	"fmt"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type Config struct {
	NetworkPassphrase string
	MaxOpenExpiry     time.Duration

	Initiator bool

	LocalEscrowAccount  *keypair.FromAddress
	RemoteEscrowAccount *keypair.FromAddress

	LocalSigner  *keypair.Full
	RemoteSigner *keypair.FromAddress
}

func NewChannel(c Config) *Channel {
	channel := &Channel{
		networkPassphrase:   c.NetworkPassphrase,
		maxOpenExpiry:       c.MaxOpenExpiry,
		initiator:           c.Initiator,
		localEscrowAccount:  &EscrowAccount{Address: c.LocalEscrowAccount},
		remoteEscrowAccount: &EscrowAccount{Address: c.RemoteEscrowAccount},
		localSigner:         c.LocalSigner,
		remoteSigner:        c.RemoteSigner,
	}
	return channel
}

// Snapshot is a snapshot of a Channel's internal state. If a Snapshot is
// combined with a Channel's initialization config they can be used to create a
// new Channel that has the same state.
type Snapshot struct {
	LocalEscrowSequence        int64
	LocalEscrowAccountBalance  int64
	RemoteEscrowSequence       int64
	RemoteEscrowAccountBalance int64

	OpenAgreement            OpenAgreement
	OpenExecutedAndValidated bool
	OpenExecutedWithError    bool

	LatestAuthorizedCloseAgreement   CloseAgreement
	LatestUnauthorizedCloseAgreement CloseAgreement
}

// NewChannelFromSnapshot creates the channel with the given config, and
// restores the internal state of the channel using the snapshot. To restore the
// channel to its identical state the same config should be provided that was in
// use when the snapshot was created.
func NewChannelFromSnapshot(c Config, s Snapshot) *Channel {
	channel := NewChannel(c)

	channel.localEscrowAccount.SequenceNumber = s.LocalEscrowSequence
	channel.localEscrowAccount.Balance = s.LocalEscrowAccountBalance
	channel.remoteEscrowAccount.SequenceNumber = s.RemoteEscrowSequence
	channel.remoteEscrowAccount.Balance = s.RemoteEscrowAccountBalance

	channel.openAgreement = s.OpenAgreement
	channel.openExecutedAndValidated = s.OpenExecutedAndValidated
	if s.OpenExecutedWithError {
		channel.openExecutedWithError = fmt.Errorf("open executed with error")
	}

	channel.latestAuthorizedCloseAgreement = s.LatestAuthorizedCloseAgreement
	channel.latestUnauthorizedCloseAgreement = s.LatestUnauthorizedCloseAgreement

	return channel
}

type EscrowAccount struct {
	Address        *keypair.FromAddress
	SequenceNumber int64
	Balance        int64
}

type Channel struct {
	networkPassphrase string
	maxOpenExpiry     time.Duration

	initiator           bool
	localEscrowAccount  *EscrowAccount
	remoteEscrowAccount *EscrowAccount

	localSigner  *keypair.Full
	remoteSigner *keypair.FromAddress

	openAgreement            OpenAgreement
	openExecutedAndValidated bool
	openExecutedWithError    error

	latestAuthorizedCloseAgreement   CloseAgreement
	latestUnauthorizedCloseAgreement CloseAgreement
}

// Snapshot returns a snapshot of the channel's internal state that if combined
// with it's initialization config can be used to create a new Channel that has
// the same state.
func (c *Channel) Snapshot() Snapshot {
	return Snapshot{
		LocalEscrowSequence:        c.localEscrowAccount.SequenceNumber,
		LocalEscrowAccountBalance:  c.localEscrowAccount.Balance,
		RemoteEscrowSequence:       c.remoteEscrowAccount.SequenceNumber,
		RemoteEscrowAccountBalance: c.remoteEscrowAccount.Balance,

		OpenAgreement:            c.openAgreement,
		OpenExecutedAndValidated: c.openExecutedAndValidated,
		OpenExecutedWithError:    c.openExecutedWithError != nil,

		LatestAuthorizedCloseAgreement:   c.latestAuthorizedCloseAgreement,
		LatestUnauthorizedCloseAgreement: c.latestUnauthorizedCloseAgreement,
	}
}

type State int

const (
	StateError State = iota - 1
	StateNone
	StateOpen
	StateClosing
	StateClosingWithOutdatedState
	StateClosed
)

// State returns a single value representing the overall state of the
// channel. If there was an error finding the state, or internal values are
// unexpected, then a failed channel state is returned, indicating something is
// wrong.
func (c *Channel) State() (State, error) {
	if c.openExecutedWithError != nil {
		return StateError, nil
	}

	if !c.openExecutedAndValidated {
		return StateNone, nil
	}

	// Get the sequence numbers for the latest close agreement transactions.
	declTxAuthorized, closeTxAuthorized, err := c.closeTxs(c.openAgreement.Details, c.latestAuthorizedCloseAgreement.Details)
	if err != nil {
		return -1, fmt.Errorf("building declaration and close txs for latest authorized close agreement: %w", err)
	}
	latestDeclSequence := declTxAuthorized.SequenceNumber()
	latestCloseSequence := closeTxAuthorized.SequenceNumber()

	initiatorEscrowSeqNum := c.initiatorEscrowAccount().SequenceNumber

	if initiatorEscrowSeqNum == c.openAgreement.Details.StartingSequence {
		return StateOpen, nil
	} else if initiatorEscrowSeqNum < latestDeclSequence {
		return StateClosingWithOutdatedState, nil
	} else if initiatorEscrowSeqNum == latestDeclSequence {
		return StateClosing, nil
	} else if initiatorEscrowSeqNum >= latestCloseSequence {
		return StateClosed, nil
	}

	return StateError, fmt.Errorf("initiator escrow account sequence has unexpected value")
}

func (c *Channel) setInitiatorEscrowAccountSequence(seqNum int64) {
	c.initiatorEscrowAccount().SequenceNumber = seqNum
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
