/*
Package nemesis contains pre-built scenarios that introduce problems into
distributed systems to try and trigger rare bugs.
*/
package nemesis

import (
	"log"
	"math/rand"
	"net/netip"
	"time"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	simnet "github.com/neria-cloud/gosim/net"
)

// A Scenario is a potentially challenging scenario that can be run and
// introduce chaos to running machines or a network to see if they keep behaving
// as expected.
type Scenario interface {
	Run()
}

// Sleep is a scenario that simply sleeps.
type Sleep struct {
	Duration time.Duration
}

// Run implements Scenario.
func (s Sleep) Run() {
	time.Sleep(s.Duration)
}

// PartitionMachines is a scenario that groups the given addresses in two
// different sets and disables the network between the groups.
type PartitionMachines struct {
	Addresses []string
	Duration  time.Duration
}

// Run implements Scenario.
func (p PartitionMachines) Run() {
	var a, b []string

	for {
		a, b = nil, nil
		for _, addr := range p.Addresses {
			// #nosec G404 -- nemesis choices use the simulation's replayable random stream.
			if rand.Intn(2) == 0 {
				a = append(a, addr)
			} else {
				b = append(b, addr)
			}
		}
		if len(p.Addresses) <= 1 || (len(a) > 0 && len(b) > 0) {
			break
		}
	}

	log.Printf("partition machines: partitioning network into %v and %v", a, b)

	applyPartitionLinks(a, b, simnet.Partition)

	time.Sleep(p.Duration)

	log.Printf("partition machines: rejoining network between %v and %v", a, b)

	applyPartitionLinks(a, b, simnet.Repair)
}

func applyPartitionLinks(a, b []string, apply func(simnet.Link) error) {
	for _, from := range a {
		for _, to := range b {
			if err := apply(simnet.Link{From: netip.MustParseAddr(from), To: netip.MustParseAddr(to)}); err != nil {
				panic(err)
			}
		}
	}
}

// RestartRandomly restarts one randomly chosen machine.
type RestartRandomly struct {
	// Machines to choose from for restarting
	Machines []gosim.Machine
	// Time to wait between crashing machine and starting again
	Downtime time.Duration
}

// Run implements Scenario.
func (r RestartRandomly) Run() {
	if len(r.Machines) == 0 {
		panic("restart randomly: need at least one machine")
	}
	// #nosec G404 -- nemesis choices use the simulation's replayable random stream.
	m := r.Machines[rand.Intn(len(r.Machines))]

	label, err := m.Label()
	if err != nil {
		panic(err)
	}
	log.Printf("restart randomly: crashing machine %s", label)
	if err := simfs.SetCrashMode(m, simfs.SamplePending); err != nil {
		panic(err)
	}

	if err := m.Crash(); err != nil {
		panic(err)
	}
	time.Sleep(r.Downtime)

	log.Printf("restart randomly: restarting machine %s", label)
	if err := m.Restart(); err != nil {
		panic(err)
	}
}

type funcScenario func()

func (f funcScenario) Run() {
	f()
}

// Repeat repeats the given scenario a number of times.
func Repeat(scenario Scenario, times int) Scenario {
	return funcScenario(func() {
		for range times {
			scenario.Run()
		}
	})
}

// Sequence runs the given scenearios in sequence.
func Sequence(scenarios ...Scenario) Scenario {
	return funcScenario(func() {
		for _, s := range scenarios {
			s.Run()
		}
	})
}
