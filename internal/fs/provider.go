// Package fs implements the translated filesystem provider. Its session owns
// persistent disks and recovery handles; the kernel owns machine state and fds.
package fs

import "github.com/neria-cloud/gosim/internal/contracts"

// FlockSemantics selects the executable host's open-file lock lifetime. The
// composition generator supplies it; translated source is loaded as Linux even
// when the executable runs on Darwin.
type FlockSemantics uint8

const (
	FlockLinux FlockSemantics = iota + 1
	FlockDarwin
)

type Provider struct{ flockSemantics FlockSemantics }

func NewProvider(semantics FlockSemantics) contracts.Provider {
	return &Provider{flockSemantics: semantics}
}
func (*Provider) Feature() contracts.Feature { return contracts.FS }
func (p *Provider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	if p.flockSemantics != FlockLinux && p.flockSemantics != FlockDarwin {
		return nil, contracts.ErrInvalidConfig
	}
	if err := contracts.ValidatePorts(ports); err != nil {
		return nil, err
	}
	return &Session{ports: ports, flockSemantics: p.flockSemantics, machines: make(map[uint64]*machine), destroyed: make(map[uint64]bool), handles: make(map[uint64]*recoveryHandle), nextHandle: 1}, nil
}
