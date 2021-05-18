type Asset struct{}

//// Methods directly on the SDK

func NewChannel() {

}

func NewChannelListenerService() {

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
	MyClaimAmount          int
	OtherAccount           string
	OtherClaimAmount       int
}

func (c *ChannelStateMachine) Init() {
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

type TxInfo struct {
	ID               string
	Iteration        int
	Type             string // declaration | close
	Seq              int64
	InitiatorBalance int
	ResponderBalance int
}

type ChannelCheckResponse struct {
	IsContestable   bool
	Asset           Asset
	TriggeredTxInfo TxInfo
	LatestTxInfo    TxInfo
}


