//go:build sim

package behavior_test

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
)

func TestNetworkConnectionTimeout(t *testing.T) {
	for _, test := range []struct {
		name                   string
		timeout, dial, elapsed time.Duration
		success                bool
	}{
		{"zero_uses_default", 0, 10 * time.Second, time.Second, false},
		{"shorter_model_timeout", 250 * time.Millisecond, 10 * time.Second, 250 * time.Millisecond, false},
		{"high_latency_handshake", 5 * time.Second, 10 * time.Second, 4 * time.Second, true},
		{"caller_deadline", 5 * time.Second, 500 * time.Millisecond, 500 * time.Millisecond, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := simnet.DefaultConfig()
			config.ConnectionTimeout = test.timeout
			config.Latency = simnet.LatencyConfig{Min: 2 * time.Second, Max: 2 * time.Second, Lambda: 1}
			networkCheck(t, simnet.Configure(config))
			ready := make(chan struct{})
			server, err := gosim.NewMachine(gosim.MachineConfig{
				Main: func() {
					listener, err := net.Listen("tcp", "10.61.0.1:8080")
					if err != nil {
						panic(err)
					}
					defer listener.Close()
					close(ready)
					// The model completes the handshake into the listener's
					// backlog. Keep it alive until the test stops this machine.
					select {}
				},
			})
			networkCheck(t, err)
			t.Cleanup(func() {
				networkCheck(t, server.Stop())
				networkCheck(t, server.Destroy())
			})
			networkCheck(t, simnet.ConfigureMachine(server, simnet.MachineConfig{Addr: netip.MustParseAddr("10.61.0.1")}))
			networkCheck(t, server.Start())
			<-ready
			started := time.Now()
			conn, err := net.DialTimeout("tcp", "10.61.0.1:8080", test.dial)
			elapsed := time.Since(started)
			if conn != nil {
				defer conn.Close()
			}
			if (err == nil) != test.success || elapsed != test.elapsed {
				t.Fatalf("dial: elapsed=%s err=%v; want elapsed=%s success=%v", elapsed, err, test.elapsed, test.success)
			}
			if test.name == "caller_deadline" {
				if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
					t.Fatalf("caller deadline did not return a timeout: %v", err)
				}
			}
		})
	}
}
