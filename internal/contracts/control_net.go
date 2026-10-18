package contracts

type NetConfig struct {
	MinNS, MaxNS         int64
	Lambda, Fail, Repair float64
}
type NetLink struct {
	From, To  [4]byte
	Direction uint8
}
type NetLinkPolicy struct {
	SetLatency, SetLoss  bool
	MinNS, MaxNS         int64
	Lambda, Fail, Repair float64
}
type NetControl interface {
	Configure(NetConfig) error
	ConfigureMachine(MachineRef, [4]byte, bool) error // bool=automatic address
	Address(MachineRef) ([4]byte, error)
	SetLinkPolicy(NetLink, NetLinkPolicy) error
	ResetLinkPolicy(NetLink) error
	SetConnectivity(NetLink, bool) error
}
