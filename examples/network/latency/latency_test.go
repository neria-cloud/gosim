//go:build sim

package latency_test

import (
	"encoding/json"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

type message struct {
	Kind         string
	Sequence     int
	PingSent     time.Time
	PingReceived time.Time
	PongSent     time.Time
}

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
	// Allow the handshake to complete even in the slowest latency scenario.
	config.ConnectionTimeout = 5 * time.Second
	config.Latency = simnet.LatencyConfig{Min: minLatency, Max: maxLatency, Lambda: 5}
	if err := simnet.Configure(config); err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{})
	server, err := gosim.NewMachine(gosim.MachineConfig{
		Label: "latency-pong",
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
			decoder, encoder := json.NewDecoder(conn), json.NewEncoder(conn)
			for {
				var ping message
				if err := decoder.Decode(&ping); err != nil {
					if err == io.EOF {
						return
					}
					panic(err)
				}
				received := time.Now()
				if ping.Kind != "ping" {
					panic("expected ping")
				}
				pong := message{
					Kind:         "pong",
					Sequence:     ping.Sequence,
					PingSent:     ping.PingSent,
					PingReceived: received,
					PongSent:     time.Now(),
				}
				if err := encoder.Encode(pong); err != nil {
					panic(err)
				}
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

	conn, err := net.DialTimeout("tcp", serverAddress, config.ConnectionTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	started := time.Now()
	end := started.Add(duration)
	if err := conn.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	encoder, decoder := json.NewEncoder(conn), json.NewDecoder(conn)
	t.Logf("BEGIN ping/pong: one-way latency=%s..%s duration=%s connection drops=0",
		minLatency, maxLatency, duration)

	var completed int
	var totalRTT, longestRTT time.Duration
	shortestRTT := duration
	for time.Until(end) > 2*maxLatency {
		sent := time.Now()
		ping := message{Kind: "ping", Sequence: completed + 1, PingSent: sent}
		t.Logf("PING #%d sent=%s; waiting for PONG", ping.Sequence, sent.Format(time.RFC3339Nano))
		if err := encoder.Encode(ping); err != nil {
			t.Fatalf("connection failed after %d exchanges: %v", completed, err)
		}
		var pong message
		if err := decoder.Decode(&pong); err != nil {
			t.Fatalf("connection failed after %d exchanges: %v", completed, err)
		}
		received := time.Now()
		if pong.Kind != "pong" || pong.Sequence != ping.Sequence || !pong.PingSent.Equal(sent) {
			t.Fatalf("unexpected pong: %+v", pong)
		}
		outbound := pong.PingReceived.Sub(sent)
		inbound := received.Sub(pong.PongSent)
		if outbound < minLatency || outbound > maxLatency || inbound < minLatency || inbound > maxLatency {
			t.Fatalf("one-way latency outside [%s, %s]: ping=%s pong=%s", minLatency, maxLatency, outbound, inbound)
		}
		rtt := received.Sub(sent)
		if rtt < 2*minLatency || rtt > 2*maxLatency {
			t.Fatalf("round trip %s outside [%s, %s]", rtt, 2*minLatency, 2*maxLatency)
		}
		t.Logf("PING #%d sent=%s received=%s latency=%s",
			ping.Sequence, sent.Format(time.RFC3339Nano), pong.PingReceived.Format(time.RFC3339Nano), outbound)
		t.Logf("PONG #%d sent=%s received=%s latency=%s round-trip=%s",
			pong.Sequence, pong.PongSent.Format(time.RFC3339Nano), received.Format(time.RFC3339Nano), inbound, rtt)
		completed++
		totalRTT += rtt
		shortestRTT = min(shortestRTT, rtt)
		longestRTT = max(longestRTT, rtt)
		// Pace probes and keep one request in flight on the same connection.
		time.Sleep(min(time.Second, time.Until(end)))
	}
	// Leave enough time for every started exchange to finish, then keep the
	// connection open for the remainder of the 15-second observation window.
	time.Sleep(time.Until(end))
	if completed == 0 || time.Since(started) != duration {
		t.Fatalf("incomplete observation: exchanges=%d elapsed=%s", completed, time.Since(started))
	}
	t.Logf(
		"END one-way latency=%s..%s, simulated duration=%s, exchanges=%d, RTT min/mean/max=%s/%s/%s, connection drops=0",
		minLatency,
		maxLatency,
		duration,
		completed,
		shortestRTT,
		totalRTT/time.Duration(completed),
		longestRTT,
	)
}
