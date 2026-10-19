//go:build sim

package behavior_test

import (
	"errors"
	"testing"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
)

func TestNetworkConfigDirections(t *testing.T) {
	for _, operation := range []func(simnet.Link) error{simnet.Partition, simnet.Repair} {
		for _, pair := range [][2]string{{aAddr, bAddr}, {bAddr, aAddr}} {
			link := networkLink(pair[0], pair[1])
			link.Direction = simnet.Forward
			networkCheck(t, operation(link))
		}
		if err := operation(simnet.Link{Direction: simnet.Forward}); !errors.Is(err, gosim.ErrInvalidConfig) {
			t.Fatal(err)
		}
	}
	link := networkLink(aAddr, bAddr)
	networkCheck(t, simnet.Partition(link))
	link.Direction = simnet.Forward
	networkCheck(t, simnet.Repair(link))
	networkCheck(t, simnet.Repair(networkLink(bAddr, aAddr)))
}
