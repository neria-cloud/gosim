package model

import (
	"net/netip"
	"testing"
	"time"
)

func TestConnectionTimeoutCapturedAtOpen(t *testing.T) {
	t.Parallel()
	for _, timeout := range []time.Duration{0, 250 * time.Millisecond, 3 * time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			t.Parallel()
			n, events, client, _ := liveNetwork(t)
			// Keep the opening packet in flight beyond both deadlines.
			if err := n.SetDelay(testPair, time.Minute); err != nil {
				t.Fatal(err)
			}
			config := DefaultConfig()
			config.ConnectionTimeout = timeout
			control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: config})
			effective := timeout
			if effective == 0 {
				effective = time.Second
			}
			first, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
			if err != nil {
				t.Fatal(err)
			}
			config.ConnectionTimeout = 5 * time.Second
			control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: config})
			second, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
			if err != nil {
				t.Fatal(err)
			}
			events.advance(int64(effective) - 1)
			if first.closed || second.closed {
				t.Fatal("connection timed out early")
			}
			events.advance(int64(effective))
			if !first.closed || second.closed {
				t.Fatal("reconfiguration changed a pending connection's timeout")
			}
			events.advance(int64(5 * time.Second))
			if !second.closed {
				t.Fatal("new connection ignored updated timeout")
			}
		})
	}
}

func TestConnectionTimeoutCanceledAfterOpen(t *testing.T) {
	t.Parallel()
	n, events, client, server := liveNetwork(t)
	config := DefaultConfig()
	config.ConnectionTimeout = 5 * time.Second
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: config})
	for _, pair := range []HostPair{testPair, testPair.Flip()} {
		if err := n.SetDelay(pair, 2*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := server.OpenListener(80); err != nil {
		t.Fatal(err)
	}
	stream, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
	if err != nil {
		t.Fatal(err)
	}
	events.advance(int64(4 * time.Second))
	if !stream.opened || stream.closed || stream.openFailedTimer != nil {
		t.Fatal("high-latency connection did not open and cancel its timeout")
	}
	events.advance(int64(6 * time.Second))
	if stream.closed {
		t.Fatal("opening timeout closed an established connection")
	}
}
