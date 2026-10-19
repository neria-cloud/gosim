//go:build sim

package behavior_test

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/tests/testsupport"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
)

func networkCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func networkLink(a, b string) simnet.Link {
	return simnet.Link{From: netip.MustParseAddr(a), To: netip.MustParseAddr(b)}
}

func TestNetworkConfigValidation(t *testing.T) {
	c := simnet.DefaultConfig()
	c.Latency.Lambda = 2.5
	c.Loss = simnet.LossConfig{FailProbability: 0.125, RepairProbability: 0.375}
	networkCheck(t, simnet.Configure(c))
	latency := simnet.LatencyConfig{Max: time.Second, Lambda: 0.25}
	loss := simnet.LossConfig{FailProbability: 0.125, RepairProbability: 0.375}
	networkCheck(t, simnet.SetLinkPolicy(networkLink(aAddr, bAddr), simnet.LinkPolicy{Latency: &latency, Loss: &loss}))
	for _, operation := range []func() error{
		func() error { return simnet.Configure(simnet.Config{}) },
		func() error { bad := c; bad.Latency.Max = -1; return simnet.Configure(bad) },
		func() error { bad := c; bad.Latency.Lambda = math.NaN(); return simnet.Configure(bad) },
		func() error { bad := c; bad.Loss.FailProbability = 2; return simnet.Configure(bad) },
		func() error { bad := c; bad.Loss.RepairProbability = -1; return simnet.Configure(bad) },
		func() error { return simnet.Partition(simnet.Link{}) },
		func() error {
			return simnet.Partition(simnet.Link{From: netip.MustParseAddr("::1"), To: netip.MustParseAddr(bAddr)})
		},
		func() error {
			return simnet.SetLinkPolicy(networkLink(aAddr, bAddr), simnet.LinkPolicy{Latency: &simnet.LatencyConfig{Min: time.Second, Max: time.Millisecond, Lambda: 5}})
		},
	} {
		if err := operation(); !errors.Is(err, gosim.ErrInvalidConfig) {
			t.Fatal(err)
		}
	}
	networkCheck(t, simnet.ResetLinkPolicy(networkLink(aAddr, bAddr)))
	networkCheck(t, simnet.Configure(simnet.DefaultConfig()))
}

type networkProbe struct {
	peer   int
	fresh  bool
	result chan networkProbeResult
}
type networkProbeResult struct {
	elapsed time.Duration
	err     error
}

func TestNetworkConfigFiveHosts(t *testing.T) {
	addresses := make([]string, 5)
	commands := make([]chan networkProbe, 5)
	machines := make([]gosim.Machine, 5)
	ready := make(chan struct{}, 5)
	start := make(chan struct{})
	for i := range addresses {
		addresses[i] = fmt.Sprintf("10.23.0.%d", i+1)
		commands[i] = make(chan networkProbe)
	}
	for i := range addresses {
		i := i
		machines[i] = testsupport.StartConfigured(t, gosim.MachineConfig{Label: fmt.Sprintf("h%d", i+1), Main: func() {
			<-start
			listener, err := net.Listen("tcp", addresses[i]+":1234")
			networkCheck(t, err)
			defer listener.Close()
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					go func() { defer conn.Close(); _, _ = io.Copy(conn, conn) }()
				}
			}()
			ready <- struct{}{}
			conns := map[int]net.Conn{}
			defer func() {
				for _, conn := range conns {
					_ = conn.Close()
				}
			}()
			for command := range commands[i] {
				conn := conns[command.peer]
				if command.fresh && conn != nil {
					_ = conn.Close()
					conn = nil
					delete(conns, command.peer)
				}
				if conn == nil {
					conn, err = net.DialTimeout("tcp", addresses[command.peer]+":1234", 2*time.Second)
					if err != nil {
						command.result <- networkProbeResult{err: err}
						continue
					}
					conns[command.peer] = conn
				}
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				started := time.Now()
				_, err = conn.Write([]byte("ping"))
				var data [4]byte
				if err == nil {
					_, err = io.ReadFull(conn, data[:])
				}
				if err == nil && string(data[:]) != "ping" {
					err = fmt.Errorf("wrong echo %q", data)
				}
				if err != nil {
					_ = conn.Close()
					delete(conns, command.peer)
				}
				command.result <- networkProbeResult{elapsed: time.Since(started), err: err}
			}
		}}, netip.MustParseAddr(addresses[i]))
	}
	close(start)
	for range addresses {
		<-ready
	}
	probe := func(i, j int, fresh bool) networkProbeResult {
		t.Helper()
		result := make(chan networkProbeResult, 1)
		commands[i] <- networkProbe{peer: j, fresh: fresh, result: result}
		return <-result
	}
	// Establish all ten connections using the automatic defaults.
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			networkCheck(t, probe(i, j, false).err)
		}
	}
	a, b := addresses[1], addresses[4]
	networkCheck(t, simnet.SetLinkPolicy(networkLink(a, b), simnet.LinkPolicy{Latency: &simnet.LatencyConfig{Min: 200 * time.Millisecond, Max: 200 * time.Millisecond, Lambda: 5}}))
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			result := probe(i, j, false)
			networkCheck(t, result.err)
			if i == 1 && j == 4 {
				if result.elapsed < 400*time.Millisecond {
					t.Fatal("existing connection ignored latency", result.elapsed)
				}
			} else if result.elapsed > 200*time.Millisecond {
				t.Fatal("unrelated pair delayed", i, j, result.elapsed)
			}
		}
	}
	// A symmetric partition blocks both directions, including after repair.
	networkCheck(t, simnet.Partition(networkLink(a, b)))
	for _, pair := range [][2]int{{1, 4}, {4, 1}} {
		if probe(pair[0], pair[1], true).err == nil {
			t.Fatal("explicit partition did not block direction", pair)
		}
	}
	networkCheck(t, simnet.Repair(networkLink(a, b)))
	networkCheck(t, probe(1, 4, true).err)
	networkCheck(t, probe(4, 1, true).err)
	networkCheck(t, simnet.Partition(networkLink(a, b)))
	for _, pair := range [][2]int{{1, 4}, {4, 1}} {
		if probe(pair[0], pair[1], true).err == nil {
			t.Fatal("repeated partition did not block direction", pair)
		}
	}
	networkCheck(t, simnet.Repair(networkLink(a, b)))
	networkCheck(t, probe(1, 4, true).err)
	networkCheck(t, probe(4, 1, true).err)
	networkCheck(t, simnet.SetLinkPolicy(networkLink(a, b), simnet.LinkPolicy{Loss: &simnet.LossConfig{FailProbability: 1, RepairProbability: 0}}))
	if result := probe(1, 4, false); result.err == nil {
		t.Fatal("forced failure did not drop traffic")
	}
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			if i != 1 || j != 4 {
				networkCheck(t, probe(i, j, false).err)
			}
		}
	}
	networkCheck(t, simnet.ResetLinkPolicy(networkLink(a, b)))
	networkCheck(t, simnet.Repair(networkLink(a, b)))
	networkCheck(t, probe(1, 4, true).err)
	// Exercise intermediate probabilities and traffic-triggered repairs. A
	// dropped stream may require reconnection; each attempt has a bounded timeout.
	networkCheck(t, simnet.SetLinkPolicy(networkLink(a, b), simnet.LinkPolicy{Loss: &simnet.LossConfig{FailProbability: 0.125, RepairProbability: 0.5}}))
	for range 6 {
		_ = probe(1, 4, true)
	}
	networkCheck(t, simnet.ResetLinkPolicy(networkLink(a, b)))
	networkCheck(t, simnet.Repair(networkLink(a, b)))
	networkCheck(t, probe(1, 4, true).err)
	for _, command := range commands {
		close(command)
	}
	for _, machine := range machines {
		testsupport.Wait(t, machine)
	}
}
