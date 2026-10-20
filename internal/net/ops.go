package net

import (
	"encoding/binary"
	"net/netip"
	"syscall"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/net/model"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// Linux ABI constants, independent of the host compiling this provider.
const (
	addressIPv4     = 2
	socketStream    = 1
	socketNonblock  = 0x800
	socketCloseExec = 0x80000
	socketFlags     = socketStream | socketNonblock | socketCloseExec
)

func (i *Instance) socket(ctx contracts.CallContext, fd int) (*Socket, error) {
	if err := i.validateCall(ctx); err != nil {
		return nil, err
	}
	resource, err := i.fds.Lookup(fd)
	if err != nil {
		return nil, err
	}
	s, ok := resource.(*Socket)
	if !ok {
		return nil, syscall.ENOTSOCK
	}
	if s.instance != i || s.released {
		return nil, syscall.EBADF
	}
	return s, nil
}

func (i *Instance) allocate(socket *Socket) (int, error) {
	fd, err := i.fds.Allocate(socket)
	if err != nil {
		if cleanup := socket.Release(contracts.CloseAbort); cleanup != nil {
			panic(cleanup)
		}
		return 0, err
	}
	socket.fd = fd
	i.sockets[fd] = socket
	return fd, nil
}

func (i *Instance) Socket(ctx contracts.CallContext, family, flags, protocol int) (int, error) {
	if err := i.validateCall(ctx); err != nil {
		return 0, err
	}
	// TODO(not planned): IPv6, UDP, raw sockets, and blocking socket mode.
	if family != addressIPv4 {
		return 0, syscall.EAFNOSUPPORT
	}
	if flags != socketFlags {
		return 0, syscall.EINVAL
	}
	// Linux net capability discovery explicitly passes IPPROTO_TCP.
	if protocol != 0 && protocol != 6 {
		return 0, syscall.EPROTONOSUPPORT
	}
	return i.allocate(&Socket{instance: i})
}

func validateAddress(addr contracts.Address) error {
	if addr.Family != addressIPv4 {
		return syscall.EAFNOSUPPORT
	}
	if netip.AddrFrom4(addr.IPv4).IsMulticast() {
		return syscall.EADDRNOTAVAIL
	}
	return nil
}

func (i *Instance) Bind(ctx contracts.CallContext, fd int, addr contracts.Address) error {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return err
	}
	if err := validateAddress(addr); err != nil {
		return err
	}
	if s.bound || s.stream != nil || s.listener != nil {
		return syscall.EINVAL
	}
	if addr.IPv4 != ([4]byte{}) && addr.IPv4 != i.machine.address {
		return syscall.EADDRNOTAVAIL
	}
	// TODO(not planned): ephemeral listening ports and explicit client binds.
	if addr.Port == 0 {
		return syscall.EOPNOTSUPP
	}
	s.bound = true
	s.boundAddress = addr
	return nil
}

func (i *Instance) Listen(ctx contracts.CallContext, fd, backlog int) error {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return err
	}
	if !s.bound || s.listener != nil || s.stream != nil || backlog < 0 {
		return syscall.EINVAL
	}
	// TODO(not planned): configurable listen backlog; model retains five pending connections.
	listener, err := i.stack.OpenListener(s.boundAddress.Port)
	if err == model.ErrPortAlreadyInUse {
		return syscall.EADDRINUSE
	}
	if err != nil {
		return syscall.EINVAL
	}
	s.listener = listener
	s.rebind()
	return nil
}

func address(addr netip.AddrPort) contracts.Address {
	return contracts.Address{Family: addressIPv4, IPv4: addr.Addr().As4(), Port: addr.Port()}
}

func (i *Instance) Accept(ctx contracts.CallContext, fd, flags int) (int, contracts.Address, error) {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return 0, contracts.Address{}, err
	}
	if flags != socketNonblock|socketCloseExec {
		return 0, contracts.Address{}, syscall.EINVAL
	}
	if s.listener == nil {
		return 0, contracts.Address{}, syscall.EINVAL
	}
	stream, err := i.stack.Accept(s.listener)
	if err != nil {
		if err != syscall.EWOULDBLOCK {
			err = syscall.EINVAL
		}
		return 0, contracts.Address{}, err
	}
	peer := address(stream.ID().Dest())
	accepted, err := i.allocate(&Socket{instance: i, stream: stream})
	return accepted, peer, err
}

func (i *Instance) Connect(ctx contracts.CallContext, fd int, addr contracts.Address) error {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return err
	}
	if err := validateAddress(addr); err != nil {
		return err
	}
	if s.listener != nil || s.bound {
		return syscall.EOPNOTSUPP
	}
	if s.stream != nil {
		if i.stack.StreamStatus(s.stream).Open {
			return syscall.EISCONN
		}
		return syscall.EALREADY
	}
	if addr.Port == 0 || addr.IPv4 == ([4]byte{}) {
		return syscall.EINVAL
	}
	stream, err := i.stack.OpenStream(netip.AddrPortFrom(netip.AddrFrom4(addr.IPv4), addr.Port))
	if err != nil {
		return err
	}
	s.stream = stream
	s.rebind()
	return syscall.EINPROGRESS
}

func (i *Instance) LocalAddress(ctx contracts.CallContext, fd int) (contracts.Address, error) {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return contracts.Address{}, err
	}
	if s.stream != nil {
		return address(s.stream.ID().Source()), nil
	}
	if s.bound {
		return s.boundAddress, nil
	}
	return contracts.Address{Family: addressIPv4}, nil
}

func (i *Instance) PeerAddress(ctx contracts.CallContext, fd int) (contracts.Address, error) {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return contracts.Address{}, err
	}
	if s.stream == nil {
		return contracts.Address{}, syscall.ENOTCONN
	}
	return address(s.stream.ID().Dest()), nil
}

func (i *Instance) SetOption(ctx contracts.CallContext, fd, level, name int, value abi.ByteView) error {
	if _, err := i.socket(ctx, fd); err != nil {
		return err
	}
	// Explicitly bounded stubs used by Go TCP setup. No real option semantics
	// are claimed; all other options fail instead of blanket success.
	size := 0
	if level == 1 {
		switch name {
		case 2, 7, 8, 9:
			size = 4
		case 13:
			size = 8
		}
	}
	if level == 6 {
		switch name {
		case 1, 4, 5, 6:
			size = 4
		}
	}
	if size == 0 {
		return syscall.ENOPROTOOPT
	}
	if value.Len() != size {
		return syscall.EINVAL
	}
	return nil
}

func (i *Instance) GetOption(ctx contracts.CallContext, fd, level, name int, value abi.ByteView) (int, error) {
	s, err := i.socket(ctx, fd)
	if err != nil {
		return 0, err
	}
	if level != 1 || name != 4 {
		return 0, syscall.ENOPROTOOPT
	}
	if value.Len() != 4 {
		return 0, syscall.EINVAL
	}
	if s.stream == nil {
		return 0, syscall.ENOTCONN
	}
	status := i.stack.StreamStatus(s.stream)
	var errno uint32
	if !status.Open {
		if status.Closed {
			errno = 110
		} else {
			errno = 115
		}
	}
	var bytes [4]byte
	binary.LittleEndian.PutUint32(bytes[:], errno)
	value.Write(bytes[:])
	return 4, nil
}

func (i *Instance) FcntlInt(ctx contracts.CallContext, fd, _, argument int) (int, error) {
	if _, err := i.socket(ctx, fd); err != nil {
		return 0, err
	}
	return 0, syscall.ENOSYS
}
