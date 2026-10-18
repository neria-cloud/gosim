package contracts

type StartSpec struct {
	Ref     EpochRef
	Label   string
	Restart bool
}
type Provider interface {
	Feature() Feature
	Open(Ports) (ProviderSession, error)
}
type ProviderSession interface {
	Controls() ControlBindings
	Prepare(StartSpec, DescriptorTable) (PreparedMachine, error)
	Destroy(MachineRef)
	Close()
}
type PreparedMachine interface {
	Bindings() Bindings
	Publish() MachineInstance
	Abort()
}
type MachineInstance interface {
	Quiesce()
	Settle(StopKind) error
	Detach()
}
type Bindings struct {
	Files   FileOps   // nil when provider does not own this service
	Sockets SocketOps // nil when provider does not own this service
}
type ControlBindings struct {
	FSControl  FSControl // nil when not owned
	NetControl NetControl
}
