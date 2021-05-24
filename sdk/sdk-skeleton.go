package skeleton

type Asset struct{}

//// Methods directly on the SDK

func NewChannel() {

}

//// Channel State Machine

type ChannelStateMachine struct {
	Status                 string // waiting | active | closing | closed
	Asset                  Asset
	InitiatorEscrowAccount string
	ResponderEscrowAccount string
	InitiatorClaimAmount   int
	ResponderClaimAmount   int
	IsInitiator            bool
	MyAccount              string
	OtherAccount           string
}

// Open handles the logic for opening a channel. This includes the Formation Transaction, C_1, and D_1.
func (c *ChannelStateMachine) Open() {
	return nil
}

func (c *ChannelStateMachine) CheckChannel() (*ChannelCheckResponse, error) {
	return nil, error
}

func (c *ChannelStateMachine) Payment(sendAmount int) error {
	return nil
}

func (c *ChannelStateMachine) CloseStart(iterationNumber int) error {
	return nil
}

func (c *ChannelStateMachine) CloseCoordinated(id string) (newStatus string, err error) {
	return "", nil
}

func (c *ChannelStateMachine) CloseUncoordinated(id string) error {
	return nil
}

func (c *ChannelStateMachine) GetLatestDeclarationTx() (*TxInfo, error) {
	return nil, nil
}

func (c *ChannelStateMachine) GetLatestCloseTx(id string) (*TxInfo, error) {
	return nil, nil
}

// helper method
func (c *ChannelStateMachine) MyClaimAmount() error {
	return nil
}

// helper method
func (c *ChannelStateMachine) OtherClaimAmount() error {
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

type EscrowAccount struct {
}
