import "time"

type Asset struct{}

//// Methods directly on the SDK

func NewChannel() {

}

func NewChannelListenerService() {

}

func GetChannel() {

}

//// Channel State Machine

type ChannelStateMachine struct {
	ChannelID              string
	Status                 string // waiting | active | closing | closed
	Asset                  Asset
	InitiatorEscrowAccount string
	ResponderClaimAmount   int
	ReserveEscrowAccount   string
	InitiatorClaimAmount   int
	IsInitiator            bool
	MyAccount              string
	MyClaimAMount          int
	OtherAccount           string
	OtherClaimAmount       int
}

func (c *ChannelStateMachine) Init() {
	return nil
}

func (c *ChannelStateMachine) RegisterMonitoringNotificationHandler(handler MonitoringNotificationHandler) error {
	return nil
}

func (c *ChannelStateMachine) CheckChannel() (*ChannelCheckResponse, error) {
	return nil, error
}

func (c *ChannelStateMachine) Payment(sendAmount int, timeoutMillis int) error {
	return nil
}

func (c *ChannelStateMachine) CloseStart(declareID int) error {
	return nil
}

func (c *ChannelStateMachine) CloseCoordinated(timeoutMillis int, id string) (newStatus string, err error) {
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

type MonitoringNotificationHandler struct {
}

//// ChannelListenerService

type ChannelListenerService struct {
	Port int
	// []func(ChannelCreationRequest) void
}

type ChannelCreationRequest struct {
	ChannelID         string
	RequestTime       time.Time
	Asset             Asset
	NegotiationParams NegotiationParams
}

type NegotiationParams struct {
	InitiatorEscrowAccount   string
	InitiatorStartingBalance int
	ReserveEscrowAccount     string
	ReserveStartingBalance   int
}

func (cls *ChannelListenerService) RegisterNotificationhandler() {
	return nil
}

func (cls *ChannelListenerService) Trigger() {
	return nil
}

func (cls *ChannelListenerService) GetPendingChannelCreationRequests() {
	return nil
}

func (cls *ChannelListenerService) RespondChannelCreationRequest(channelID string, response ChannelCreationResponse, timeoutMillis int) (*ChannelConnection, error) {
	return nil, nil
}

