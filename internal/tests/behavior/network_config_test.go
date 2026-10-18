//go:build sim

package behavior_test

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
)

func networkCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetworkConfigValidation(t *testing.T) {
	c := gosim.DefaultNetworkConfig()
	c.Latency.Lambda = 2.5
	c.Loss.FailProbability = 0.125
	c.Loss.RepairProbability = 0.375
	networkCheck(t, gosim.ConfigureNetwork(c))
	// These noninteger values must survive the pointer-based syscall unchanged.
	networkCheck(t, gosim.SetMessageLatencyCurve(0.5))
	networkCheck(t, gosim.SetLinkMessageLatencyCurve(aAddr, bAddr, 0.25))
	networkCheck(t, gosim.SetFailRate(0.125))
	networkCheck(t, gosim.SetRepairRate(0.375))
	networkCheck(t, gosim.SetLinkFailRate(aAddr, bAddr, 0.125))
	networkCheck(t, gosim.SetLinkRepairRate(aAddr, bAddr, 0.375))
	for _, op := range []func() error{
		func() error { return gosim.ConfigureNetwork(gosim.NetworkConfig{}) },
		func() error { return gosim.SetMaxMessageLatency(-1) },
		func() error { return gosim.SetMessageLatencyCurve(math.NaN()) },
		func() error { return gosim.SetFailRate(2) }, func() error { return gosim.SetRepairRate(-1) },
		func() error { return gosim.SetConnected("bad", bAddr, false) },
		func() error { return gosim.SetDelay("::1", bAddr, 0) },
		func() error { return gosim.SetLinkLatency(aAddr, bAddr, -1) },
	} {
		if err := op(); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(err)
		}
	}
	networkCheck(t, gosim.SetLinkLatency(aAddr, bAddr, time.Second))
	if err := gosim.SetLinkMaxMessageLatency(aAddr, bAddr, time.Millisecond); !errors.Is(err, syscall.EINVAL) {
		t.Fatal(err)
	}
	networkCheck(t, gosim.ResetLinkLatency(aAddr, bAddr))
	networkCheck(t, gosim.ResetLinkLoss(aAddr, bAddr))
	networkCheck(t, gosim.ConfigureNetwork(gosim.DefaultNetworkConfig()))
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
		machines[i] = gosim.NewMachine(gosim.MachineConfig{Label: fmt.Sprintf("h%d", i+1), Addr: netip.MustParseAddr(addresses[i]), MainFunc: func() {
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
		}})
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
	networkCheck(t, gosim.SetLinkLatency(a, b, 200*time.Millisecond))
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
	// Both public two-way entry points must block both directions.
	networkCheck(t, gosim.Partition(a, b))
	for _, pair := range [][2]int{{1, 4}, {4, 1}} {
		if probe(pair[0], pair[1], true).err == nil {
			t.Fatal("explicit partition did not block direction", pair)
		}
	}
	networkCheck(t, gosim.SetConnected(a, b, true))
	networkCheck(t, probe(1, 4, true).err)
	networkCheck(t, probe(4, 1, true).err)
	networkCheck(t, gosim.SetConnected(a, b, false))
	for _, pair := range [][2]int{{1, 4}, {4, 1}} {
		if probe(pair[0], pair[1], true).err == nil {
			t.Fatal("legacy partition did not block direction", pair)
		}
	}
	networkCheck(t, gosim.Repair(a, b))
	networkCheck(t, probe(1, 4, true).err)
	networkCheck(t, probe(4, 1, true).err)
	networkCheck(t, gosim.SetLinkFailRate(a, b, 1))
	networkCheck(t, gosim.SetLinkRepairRate(a, b, 0))
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
	networkCheck(t, gosim.ResetLinkLatency(a, b))
	networkCheck(t, gosim.ResetLinkLoss(a, b))
	networkCheck(t, gosim.Repair(a, b))
	networkCheck(t, probe(1, 4, true).err)
	// Exercise intermediate probabilities and traffic-triggered repairs. A
	// dropped stream may require reconnection; each attempt has a bounded timeout.
	networkCheck(t, gosim.SetLinkFailRate(a, b, 0.125))
	networkCheck(t, gosim.SetLinkRepairRate(a, b, 0.5))
	for range 6 {
		_ = probe(1, 4, true)
	}
	networkCheck(t, gosim.ResetLinkLoss(a, b))
	networkCheck(t, gosim.Repair(a, b))
	networkCheck(t, probe(1, 4, true).err)
	for _, command := range commands {
		close(command)
	}
	for _, machine := range machines {
		machine.Wait()
	}
}
