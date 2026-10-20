package contracts

import "github.com/neria-cloud/gosim/rt/runtime/abi"

// fakeFiles implements the complete contract and explicitly rejects unexercised calls.
type fakeFiles struct{}

var _ FileOps = (*fakeFiles)(nil)

func (*fakeFiles) Openat(CallContext, int, string, int, uint32) (int, error) { return 0, errInjected }
func (*fakeFiles) Renameat(CallContext, int, string, int, string) error      { return errInjected }
func (*fakeFiles) Getdents64(CallContext, int, abi.ByteView) (int, error)    { return 0, errInjected }
func (*fakeFiles) Fallocate(CallContext, int, uint32, int64, int64) error    { return errInjected }
func (*fakeFiles) Seek(CallContext, int, int64, int) (int64, error)          { return 0, errInjected }
func (*fakeFiles) Pwrite(CallContext, int, abi.ByteView, int64) (int, error) { return 0, errInjected }
func (*fakeFiles) Pread(CallContext, int, abi.ByteView, int64) (int, error)  { return 0, errInjected }
func (*fakeFiles) Sync(CallContext, int, bool) error                         { return errInjected }
func (*fakeFiles) Fstat(CallContext, int) (Stat, error)                      { return Stat{}, errInjected }
func (*fakeFiles) Statat(CallContext, int, string, int) (Stat, error)        { return Stat{}, errInjected }
func (*fakeFiles) Unlinkat(CallContext, int, string, int) error              { return errInjected }
func (*fakeFiles) Truncate(CallContext, int, int64) error                    { return errInjected }
func (*fakeFiles) Chdir(CallContext, string) error                           { return errInjected }
func (*fakeFiles) Getcwd(CallContext, abi.ByteView) (int, error)             { return 0, errInjected }
func (*fakeFiles) Mkdirat(CallContext, int, string, uint32) error            { return errInjected }
func (*fakeFiles) Flock(CallContext, int, int) error                         { return errInjected }
func (*fakeFiles) FcntlInt(CallContext, int, int, int) (int, error)          { return 0, errInjected }
func (*fakeFiles) FcntlLock(CallContext, int, int, *Lock) error              { return errInjected }
func (*fakeFiles) Map(CallContext, uintptr, uintptr, int, int, int, int64) (uintptr, error) {
	return 0, errInjected
}
func (*fakeFiles) Unmap(CallContext, uintptr, uintptr) error   { return errInjected }
func (*fakeFiles) Advise(CallContext, abi.ByteView, int) error { return errInjected }

// fakeSockets implements the complete contract and explicitly rejects unexercised calls.
type fakeSockets struct{}

var _ SocketOps = (*fakeSockets)(nil)

func (*fakeSockets) Socket(CallContext, int, int, int) (int, error) { return 0, errInjected }
func (*fakeSockets) Bind(CallContext, int, Address) error           { return errInjected }
func (*fakeSockets) Listen(CallContext, int, int) error             { return errInjected }
func (*fakeSockets) Accept(CallContext, int, int) (int, Address, error) {
	return 0, Address{}, errInjected
}
func (*fakeSockets) Connect(CallContext, int, Address) error                  { return errInjected }
func (*fakeSockets) LocalAddress(CallContext, int) (Address, error)           { return Address{}, errInjected }
func (*fakeSockets) PeerAddress(CallContext, int) (Address, error)            { return Address{}, errInjected }
func (*fakeSockets) SetOption(CallContext, int, int, int, abi.ByteView) error { return errInjected }
func (*fakeSockets) GetOption(CallContext, int, int, int, abi.ByteView) (int, error) {
	return 0, errInjected
}
func (*fakeSockets) FcntlInt(CallContext, int, int, int) (int, error) { return 0, errInjected }

// fakeFSControl implements the complete contract and explicitly rejects unexercised calls.
type fakeFSControl struct{}

var _ FSControl = (*fakeFSControl)(nil)

func (*fakeFSControl) ConfigureMachine(MachineRef, FSConfig) error  { return errInjected }
func (*fakeFSControl) SetCrashMode(MachineRef, uint8) error         { return errInjected }
func (*fakeFSControl) SetSyncFault(MachineRef, FSFaultConfig) error { return errInjected }
func (*fakeFSControl) SetReadOnly(MachineRef, bool) error           { return errInjected }
func (*fakeFSControl) Stats(MachineRef) (FSStorageStats, error)     { return FSStorageStats{}, errInjected }
func (*fakeFSControl) InspectInode(MachineRef, int) (InodeInfo, error) {
	return InodeInfo{}, errInjected
}
func (*fakeFSControl) Enumerate(MachineRef, uint64) (uint64, error) { return 0, errInjected }
func (*fakeFSControl) NextSnapshot(uint64) (uint64, bool, error)    { return 0, false, errInjected }
func (*fakeFSControl) CloseIterator(uint64) error                   { return errInjected }
func (*fakeFSControl) Restore(MachineRef, uint64) error             { return errInjected }
func (*fakeFSControl) CloseSnapshot(uint64) error                   { return errInjected }

// fakeNetControl implements the complete contract and explicitly rejects unexercised calls.
type fakeNetControl struct{}

var _ NetControl = (*fakeNetControl)(nil)

func (*fakeNetControl) Configure(NetConfig) error                        { return errInjected }
func (*fakeNetControl) ConfigureMachine(MachineRef, [4]byte, bool) error { return errInjected }
func (*fakeNetControl) Address(MachineRef) ([4]byte, error)              { return [4]byte{}, errInjected }
func (*fakeNetControl) SetLinkPolicy(NetLink, NetLinkPolicy) error       { return errInjected }
func (*fakeNetControl) ResetLinkPolicy(NetLink) error                    { return errInjected }
func (*fakeNetControl) SetConnectivity(NetLink, bool) error              { return errInjected }
