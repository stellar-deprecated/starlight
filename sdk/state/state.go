package state

import (
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

type Amount struct {
	Asset  Asset
	Amount int64
}

type EscrowAccount struct {
	Address        *keypair.FromAddress
	SequenceNumber int64
	Balances       []Amount
}

type Channel struct {
	networkPassphrase          string
	observationPeriodTime      time.Duration
	observationPeriodLedgerGap int64

	startingSequence int64
	// TODO - leave execution out for now
	// iterationNumberExecuted int64

	initiator           bool
	localEscrowAccount  *EscrowAccount
	remoteEscrowAccount *EscrowAccount

	localSigner  *keypair.Full
	remoteSigner *keypair.FromAddress

	latestCloseAgreement *CloseAgreement
	// TODO - set this, probably use different name
	latestUnconfirmedPayment *Payment
}

type Config struct {
	NetworkPassphrase          string
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64

	Initiator bool

	LocalEscrowAccount  *EscrowAccount
	RemoteEscrowAccount *EscrowAccount

	LocalSigner  *keypair.Full
	RemoteSigner *keypair.FromAddress
}

func NewChannel(c Config) *Channel {
	channel := &Channel{
		networkPassphrase:          c.NetworkPassphrase,
		observationPeriodTime:      c.ObservationPeriodTime,
		observationPeriodLedgerGap: c.ObservationPeriodLedgerGap,
		initiator:                  c.Initiator,
		localEscrowAccount:         c.LocalEscrowAccount,
		remoteEscrowAccount:        c.RemoteEscrowAccount,
		localSigner:                c.LocalSigner,
		remoteSigner:               c.RemoteSigner,
	}
	return channel
}

func (c *Channel) NextIterationNumber() int64 {
	var latestI int64
	if c.latestUnconfirmedPayment != nil {
		latestI = c.latestUnconfirmedPayment.IterationNumber
	} else if c.latestCloseAgreement != nil {
		latestI = c.latestCloseAgreement.IterationNumber
	} else {
		latestI = 0
	}
	return latestI + 1
}

// Balance returns the amount owing from the initiator to the responder, if positive, or
// the amount owing from the responder to the initiator, if negative.
func (c *Channel) Balance() Amount {
	if c.latestCloseAgreement == nil {
		return Amount{NativeAsset{}, 0}
	}
	return c.latestCloseAgreement.Balance
}

// newBalance is a hlper method for computing what the new channel balance will be if
// the input payment is submitted successfully.
func (c *Channel) newBalance(p *Payment) Amount {
	var amountFromInitiator, amountFromResponder int64
	if p.FromInitiator {
		amountFromInitiator = p.Amount.Amount
	} else {
		amountFromResponder = p.Amount.Amount
	}
	return Amount{
		Asset:  p.Amount.Asset,
		Amount: c.Balance().Amount + amountFromInitiator - amountFromResponder,
	}
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
