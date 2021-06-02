package state

import (
	"encoding/hex"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

type (
	Asset       = txnbuild.Asset
	NativeAsset = txnbuild.NativeAsset
	CreditAsset = txnbuild.CreditAsset
)

type Balance struct {
	Asset  Asset
	Amount int64
}

type EscrowAccount struct {
	Address        *keypair.FromAddress
	SequenceNumber int64
	Balances       []Balance
}

type Channel struct {
	networkPassphrase          string
	observationPeriodTime      time.Duration
	observationPeriodLedgerGap int64

	startingSequence int64
	iterationNumber  int64
	// TODO - leave execution out for now
	// iterationNumberExecuted int64

	// The balance owing from the initiator to the responder, if positive, or
	// the balance owing from the responder to the initiator, if negative.
	// TODO - use Balance struct
	balance Balance

	initiator           bool
	localEscrowAccount  *EscrowAccount
	remoteEscrowAccount *EscrowAccount

	localSigner  *keypair.Full
	remoteSigner *keypair.FromAddress
}

type ChannelConfig struct {
	NetworkPassphrase          string
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	StartingSequence           int64

	Initiator bool

	LocalEscrowAccount  *EscrowAccount
	RemoteEscrowAccount *EscrowAccount

	LocalSigner  *keypair.Full
	RemoteSigner *keypair.FromAddress
}

func NewChannel(c ChannelConfig) *Channel {
	channel := &Channel{
		networkPassphrase:          c.NetworkPassphrase,
		observationPeriodTime:      c.ObservationPeriodTime,
		observationPeriodLedgerGap: c.ObservationPeriodLedgerGap,
		startingSequence:           c.StartingSequence,
		initiator:                  c.Initiator,
		localEscrowAccount:         c.LocalEscrowAccount,
		remoteEscrowAccount:        c.RemoteEscrowAccount,
		localSigner:                c.LocalSigner,
		remoteSigner:               c.RemoteSigner,
	}
	return channel
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

func (c *Channel) sign(tx *txnbuild.Transaction) (xdr.DecoratedSignature, error) {
	hash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return xdr.DecoratedSignature{}, err
	}
	sig, err := c.localSigner.SignDecorated(hash[:])
	if err != nil {
		return xdr.DecoratedSignature{}, err
	}
	return sig, nil
}

type errNotSigned struct {
	hash   string
	signer string
}

func (e errNotSigned) Error() string { return "tx " + e.hash + " not signed by signer " + e.signer }

func (c *Channel) verifySigned(tx *txnbuild.Transaction, sigs []xdr.DecoratedSignature, signer keypair.KP) error {
	hash, err := tx.Hash(c.networkPassphrase)
	if err != nil {
		return err
	}
	for _, sig := range sigs {
		if sig.Hint != signer.Hint() {
			continue
		}
		err := signer.Verify(hash[:], sig.Signature)
		if err == nil {
			return nil
		}
	}
	return errNotSigned{
		hash:   hex.EncodeToString(hash[:]),
		signer: signer.Address(),
	}
}

func (c *Channel) CloseStart(iterationNumber int) error {
	return nil
}

func (c *Channel) CloseCoordinated(id string) (newStatus string, err error) {
	return "", nil
}

func (c *Channel) CloseUncoordinated(id string) error {
	return nil
}

func (c *Channel) GetLatestDeclarationTx() (*TxInfo, error) {
	return nil, nil
}

func (c *Channel) GetLatestCloseTx(id string) (*TxInfo, error) {
	return nil, nil
}

// helper method
func (c *Channel) MyClaimAmount() error {
	return nil
}

// helper method
func (c *Channel) OtherClaimAmount() error {
	return nil
}

type TxInfo struct {
	ID        string
	Iteration int
	Type      string // declaration | close
	Seq       int64
}

// helper method
func (t *TxInfo) MyBalance() error {
	return nil
}

type ChannelCheckResponse struct {
	IsContestable   bool
	Asset           Asset
	TriggeredTxInfo TxInfo
	LatestTxInfo    TxInfo
}
