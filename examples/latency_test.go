//go:build sim

package examples_test

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

func TestLatency0To5ms(t *testing.T) {
	runLatencyExample(t, 0, 5*time.Millisecond)
}

func TestLatency50To100ms(t *testing.T) {
	runLatencyExample(t, 50*time.Millisecond, 100*time.Millisecond)
}

func TestLatency1To2s(t *testing.T) {
	runLatencyExample(t, time.Second, 2*time.Second)
}

func runLatencyExample(t *testing.T, minLatency, maxLatency time.Duration) {
	t.Helper()
	const duration = 15 * time.Second
	const serverAddress = "10.60.0.1:8080"

	if err := rt.SetTimeout(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	config := simnet.DefaultConfig()
	config.Loss = simnet.LossConfig{FailProbability: 0, RepairProbability: 1}
	// Establish TCP before applying latency. The simulated connection-opening
	// timeout is one second, shorter than the slowest scenario's round trip.
	config.Latency = simnet.LatencyConfig{Lambda: 5}
	if err := simnet.Configure(config); err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{})
	server, err := gosim.NewMachine(gosim.MachineConfig{
		Label: "latency-echo",
		Main: func() {
			listener, err := net.Listen("tcp", serverAddress)
			if err != nil {
				panic(err)
			}
			defer listener.Close()
			close(ready)
			conn, err := listener.Accept()
			if err != nil {
				panic(err)
			}
			defer conn.Close()
			if _, err := io.Copy(conn, conn); err != nil {
				panic(err)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := simnet.ConfigureMachine(server, simnet.MachineConfig{Addr: netip.MustParseAddr("10.60.0.1")}); err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
		if err := server.Destroy(); err != nil {
			t.Error(err)
		}
	})
	<-ready

	conn, err := net.DialTimeout("tcp", serverAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	config.Latency = simnet.LatencyConfig{Min: minLatency, Max: maxLatency, Lambda: 5}
	if err := simnet.Configure(config); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	end := started.Add(duration)
	if err := conn.SetDeadline(end); err != nil {
		t.Fatal(err)
	}

	var completed int
	var totalRTT, longestRTT time.Duration
	shortestRTT := duration
	for time.Until(end) > 2*maxLatency {
		sent := time.Now()
		if _, err := conn.Write([]byte("ping")); err != nil {
			t.Fatalf("connection failed after %d echoes: %v", completed, err)
		}
		var reply [4]byte
		if _, err := io.ReadFull(conn, reply[:]); err != nil {
			t.Fatalf("connection failed after %d echoes: %v", completed, err)
		}
		if string(reply[:]) != "ping" {
			t.Fatalf("unexpected echo: %q", reply)
		}
		rtt := time.Since(sent)
		if rtt < 2*minLatency || rtt > 2*maxLatency {
			t.Fatalf("round trip %s outside [%s, %s]", rtt, 2*minLatency, 2*maxLatency)
		}
		completed++
		totalRTT += rtt
		shortestRTT = min(shortestRTT, rtt)
		longestRTT = max(longestRTT, rtt)
		// Pace probes and keep one request in flight on the same connection.
		time.Sleep(min(100*time.Millisecond, time.Until(end)))
	}
	// Leave enough time for every started echo to finish, then keep the
	// connection open for the remainder of the 15-second observation window.
	time.Sleep(time.Until(end))
	if completed == 0 || time.Since(started) != duration {
		t.Fatalf("incomplete observation: echoes=%d elapsed=%s", completed, time.Since(started))
	}
	t.Logf("one-way latency=%s..%s, simulated duration=%s, echoes=%d, RTT min/mean/max=%s/%s/%s, connection drops=0",
		minLatency, maxLatency, duration, completed, shortestRTT, totalRTT/time.Duration(completed), longestRTT)
}
