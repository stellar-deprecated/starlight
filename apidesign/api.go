package apidesign

import "time"

type Asset struct{}

type EscrowAccount struct {
	PublicKey      string
	SequenceNumber int64
	Contributions  []Balance
}

type Balance struct {
	Asset  Asset
	Amount int
}

type Channel struct {
	Config                  Config
	Initiator               bool
	Status                  string // waiting | active | closed_collaborative | closed_hostile
	ThisEscrow              EscrowAccount
	OtherEscrow             EscrowAccount
	BalancesIncoming        []Balance // balances owed to this participant
	BalancesOutgoing        []Balance // balances owed to the other participant
	SequenceStart           int
	IterationNumber         int
	ExecutedIterationNumber int
}

type ChannelState Channel // TODO: All the fields that need persisting to persist the state of the channel.

type ObservationPeriod struct {
	Time      time.Duration
	LedgerGap int64
}

type Config struct {
	SecretKey         string
	ObservationPeriod ObservationPeriod
}

type Payment struct {
	Index       int
	Source      string
	Destination string
	Amount      string
}

type Connection interface{}

func NewChannel(config Config) (*Channel, error) {
	return nil, nil
}

func (c *Channel) Connect(conn Connection) (ChannelState, error) {
	return nil
}

func (c *Channel) CheckState() (ChannelState, error) {
	return ChannelState{}, nil
}

func (c *Channel) Pay(amount string) (ChannelState, error) {
	return ChannelState{}, nil
}

func (c *Channel) StartClose() (ChannelState, error) {
	return ChannelState{}, nil
}

func (c *Channel) CompleteClose() (ChannelState, error) {
	return ChannelState{}, nil
}
