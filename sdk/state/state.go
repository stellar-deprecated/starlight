package state

import (
	"context"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type Asset = txnbuild.Asset

// Amount is an amount of an asset.
type Amount struct {
	Asset  Asset
	Amount int64
}

// Balance is an amount of an asset owing from the initiator to the responder,
// if positive, or an amount owing from the responder to the initiator, if
// negative.
type Balance Amount

type ChannelStatus string

const (
	ChannelStatusInitialized = ChannelStatus("initialized")
	ChannelStatusOpen        = ChannelStatus("open")
	ChannelStatusOpenWaiting = ChannelStatus("open_waiting")
	ChannelStatusOpenClosing = ChannelStatus("closing")
	ChannelStatusClosed      = ChannelStatus("closed")
)

type EscrowAccount struct {
	Address        keypair.FromAddress
	SequenceNumber int64
}

type noCopy struct{}

type Channel struct {
	noCopy

	observationPeriodTime      time.Duration
	observationPeriodLedgerGap int64

	status ChannelStatus

	localEscrowAccount      EscrowAccount
	remoteEscrowAccount     EscrowAccount
	sequencingEscrowAccount *EscrowAccount

	balances []Balance

	key *keypair.Full
}

type Config struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64

	Initiator           bool
	LocalEscrowAccount  EscrowAccount
	RemoteEscrowAccount EscrowAccount

	Key *keypair.Full
}

func NewChannel(c Config) *Channel {
	channel := &Channel{
		observationPeriodTime:      c.ObservationPeriodTime,
		observationPeriodLedgerGap: c.ObservationPeriodLedgerGap,
		localEscrowAccount:         c.LocalEscrowAccount,
		remoteEscrowAccount:        c.RemoteEscrowAccount,
		key:                        c.Key,
		status:                     ChannelStatusInitialized,
	}
	channel.sequencingEscrowAccount = &channel.localEscrowAccount
	if !c.Initiator {
		channel.sequencingEscrowAccount = &channel.remoteEscrowAccount
	}
	return channel
}

// OpenPropose proposes the open of the channel, it is called by the participant
// initiating the channel.
func OpenPropose() error {
	return nil
}

// OpenConfirm
func OpenConfirm() error {
	return nil
}

func (c *Channel) CheckNetwork(ctx context.Context, client horizonclient.ClientInterface) error {
	return nil
}

func (c *Channel) Payment(sendAmount int) error {
	return nil
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
