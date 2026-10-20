package model

import (
	"bytes"
	"net/netip"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestTCPBufferSizeBackpressureAndReconfiguration(t *testing.T) {
	t.Parallel()
	n, events, client, server := liveNetwork(t)
	config := DefaultConfig()
	config.TCPBufferSize = 0 // Preserve the historical default.
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: config})
	listener, err := server.OpenListener(80)
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
	if err != nil {
		t.Fatal(err)
	}
	// Change the configuration while the first handshake is still pending.

	config.TCPBufferSize = 64 * 1024
	control(t, n, ControlRequest{Kind: ReplaceGlobal, Config: config})
	second, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
	if err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	peer, err := server.Accept(listener)
	if err != nil {
		t.Fatal(err)
	}
	secondPeer, err := server.Accept(listener)
	if err != nil {
		t.Fatal(err)
	}

	for _, pair := range []struct {
		a, b *Stream
		size int
	}{{first, peer, 1024}, {second, secondPeer, 64 * 1024}} {
		data := bytes.Repeat([]byte{0xff}, pair.size+1)
		sent, err := client.StreamSend(pair.a, abi.ByteView{Ptr: data})
		if err != nil || sent != pair.size {
			t.Fatalf("send: %d %v, capacity %d", sent, err, pair.size)
		}
		if _, err := client.StreamSend(pair.a, abi.ByteView{Ptr: data[:1]}); err != syscall.EWOULDBLOCK {
			t.Fatalf("full window: %v", err)
		}
		events.advance(events.now)
		out := make([]byte, len(data))
		received, err := server.StreamRecv(pair.b, abi.ByteView{Ptr: out})
		if err != nil || received != pair.size || !bytes.Equal(out[:received], data[:sent]) {
			t.Fatalf("receive: %d %v", received, err)
		}
		events.advance(events.now)
		// Draining the peer reopens the same connection's original window.
		sent, err = client.StreamSend(pair.a, abi.ByteView{Ptr: data})
		if err != nil || sent != pair.size {
			t.Fatalf("reopened window: %d %v", sent, err)
		}
	}
	oldConfig := n.config
	config.TCPBufferSize = -1
	if err := n.ApplyControl(ControlRequest{Kind: ReplaceGlobal, Config: config}); err != syscall.EINVAL || n.config != oldConfig {
		t.Fatal("invalid reconfiguration changed the network", err)
	}
}
