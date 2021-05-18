package apidesign

import "time"

type ObservationParams struct{}

type Asset struct{}

type ChannelFactory struct{}

type ChannelConnection struct {
	ChannelID                string
	Status                   string // waiting | active | closed_collaborative | closed_hostile
	Asset                    Asset
	InitiatorEscrowAccount   string
	InitiatorStartingBalance int
	InitiatorCurrentBalance  int
	ReserveEscrowAccount     string
	ReserveStartingBalance   int
	ReserveCurrentBalance    int
	IsInitiator              bool
	MyAccount                string
	MyStartingBalance        int
	MyBalance                int
	OtherAccount             string
	OtherStartingBalance     int
	OtherBalance             int
}

type NotificationHandler func(ChannelCreationRequest)

type NegotiationParams struct {
	InitiatorEscrowAccount   string
	InitiatorStartingBalance int
	ReserveEscrowAccount     string
	ReserveStartingBalance   int
}

type ChannelCreationRequest struct {
	ChannelID         string
	RequestTime       time.Time
	Asset             Asset
	NegotiationParams NegotiationParams
}

type ChannelCreationResponse struct {
	ChannelID         string
	Response          string // accept | reject
	Asset             Asset
	NegotiationParams NegotiationParams
}

type TxInfo struct {
	ID string
	Type string // declaration | close
	Seq int64
	InitiatorBalance int
	ResponseBalance int
}

func NewChannelFactory(secretKey string, op ObservationParams, startIndex int) (*ChannelFactory, error) {
	return nil, nil
}

func GetChannelFactories() []*ChannelFactory {
	return nil
}

func GetChannelFactory(publicKey string) *ChannelFactory {
	return nil
}

func (f *ChannelFactory) InitiateNewChannel(ipAddress string, counterpartyIPAddress string, initiatorStartingAmount string, counterpartyStartingAmount int, asset Asset) (*ChannelConnection, error) {
	return nil, nil
}

func (f *ChannelFactory) TriggerChannelListenerService(port int, notificationHandler NotificationHandler) error {
	return nil
}

func (f *ChannelFactory) GetPendingChannelCreationRequests(sinceTime *time.Time) []*ChannelCreationRequest {
	return nil
}

func (f *ChannelFactory) RespondChannelCreationRequest(channelID string, response ChannelCreationResponse, timeoutMillis int) (*ChannelConnection, error) {
	return nil, nil
}

func GetChannelConnection(channelID string) *ChannelConnection {
	return nil
}

type MonitoringNotificationHandler func(channelID string, isContestable bool, asset Asset, triggeredTxInfo TxInfo, latestTxInfo TxInfo) (attemptContest bool)

func (c *ChannelConnection) RegisterMonitoringNotificationHandler(handler MonitoringNotificationHandler) error {
	return nil
}

func (c *ChannelConnection) StartMonitoringService() error {
	return nil
}

func (c *ChannelConnection) UpdateChannelState(newInitiatorBalance int, newResponderBalance int, timeoutMillis int) error {
	return nil
}

func (c *ChannelConnection) CloseDeclarationSubmit(id string) error {
	return nil
}

func (c *ChannelConnection) CloseCoordinated(timeoutMillis int, id string) (newStatus string, err error) {
	return "", nil
}

func (c *ChannelConnection) CloseUncoordinated(id string) (error) {
	return nil
}

func (c *ChannelConnection) GetDeclarationTxList() []*TxInfo {
	return nil
}

func (c *ChannelConnection) GetDeclarationTx(id string) *TxInfo {
	return nil
}

func (c *ChannelConnection) GetPrevDeclarationTx(id string) *TxInfo {
	return nil
}

func (c *ChannelConnection) GetNextDeclarationTx(id string) *TxInfo {
	return nil
}

func (c *ChannelConnection) GetCloseTxList() []*TxInfo {
	return nil
}

func (c *ChannelConnection) GetCloseTx(id string) *TxInfo {
	return nil
}

func (c *ChannelConnection) GetPrevCloseTx(id string) *TxInfo {
	return nil
}

func (c *ChannelConnection) GetNextCloseTx(id string) *TxInfo {
	return nil
}
