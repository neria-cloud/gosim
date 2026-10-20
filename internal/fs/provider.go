// Package fs implements the translated filesystem provider. Its session owns
// persistent disks and recovery handles; the kernel owns machine state and fds.
package fs

import "github.com/neria-cloud/gosim/internal/contracts"

type Provider struct{}

func NewProvider() contracts.Provider        { return &Provider{} }
func (*Provider) Feature() contracts.Feature { return contracts.FS }
func (*Provider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	if err := contracts.ValidatePorts(ports); err != nil {
		return nil, err
	}
	return &Session{ports: ports, machines: make(map[uint64]*machine), destroyed: make(map[uint64]bool), handles: make(map[uint64]*recoveryHandle), nextHandle: 1}, nil
}
