package state

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

type Asset = txnbuild.Asset

type ChannelStatus string

const (
	ChannelStatusInitialized = ChannelStatus("initialized")
	ChannelStatusOpen        = ChannelStatus("open")
	ChannelStatusOpenWaiting = ChannelStatus("open_waiting")
	ChannelStatusOpenClosing = ChannelStatus("closing")
	ChannelStatusClosed      = ChannelStatus("closed")
)

type ProposalStatus string

const (
	ProposalStatusNone      = ProposalStatus("none")
	ProposalStatusProposed  = ProposalStatus("proposed")
	ProposalStatusConfirmed = ProposalStatus("confirmed")
	ProposalStatusRejected  = ProposalStatus("rejected")
)

type Channel struct {
	Status         ChannelStatus
	ProposalStatus ProposalStatus

	Initiator              bool
	InitiatorEscrowAccount *keypair.FromAddress
	ResponderEscrowAccount *keypair.FromAddress

	// TODO - is this the best way to rep Balance? (perspective of initiator/responder and not Me/Them)
	// The balance owing from the initiator to the responder, if positive, or
	// the balance owing from the responder to the initiator, if negative.
	Balance int64
	Asset   Asset
}

type Config struct{}

func NewChannel(c Config) *Channel {
	return nil
}

// Open handles the logic for opening a channel. This includes the Formation Transaction, C_1, and D_1.
func (c *Channel) Open() error {
	return nil
}

func (c *Channel) CheckNetwork() error {
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

type EscrowAccount struct{}
