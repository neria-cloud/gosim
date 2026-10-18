//go:build sim

package behavior_test

import (
	"errors"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim"
)

func TestNetworkConfigDirections(t *testing.T) {
	for _, operation := range []func(string, string) error{gosim.PartitionOneWay, gosim.RepairOneWay} {
		for _, pair := range [][2]string{{aAddr, bAddr}, {bAddr, aAddr}} {
			if err := operation(pair[0], pair[1]); !errors.Is(err, syscall.EOPNOTSUPP) {
				t.Fatal(err)
			}
		}
		if err := operation("bad", bAddr); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(err)
		}
	}
	networkCheck(t, gosim.Partition(aAddr, bAddr))
	if err := gosim.RepairOneWay(aAddr, bAddr); !errors.Is(err, syscall.EOPNOTSUPP) {
		t.Fatal(err)
	}
	networkCheck(t, gosim.Repair(bAddr, aAddr))
	networkCheck(t, gosim.SetConnected(aAddr, bAddr, false))
	networkCheck(t, gosim.Repair(aAddr, bAddr))
	networkCheck(t, gosim.Partition(aAddr, bAddr))
	networkCheck(t, gosim.SetConnected(bAddr, aAddr, true))
}
