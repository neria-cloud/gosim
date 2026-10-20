package model

import (
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestAckTimeoutSurvivesWritesAndRepair(t *testing.T) {
	t.Parallel()
	n, events, client, server := liveNetwork(t)
	stream, peer, listener := connect(t, events, client, server)
	if err := n.SetConnected(testPair, false); err != nil {
		t.Fatal(err)
	}
	for second := 0; second < 5; second++ {
		events.advance(int64(time.Duration(second) * time.Second))
		if second == 1 {
			if err := n.SetConnected(testPair, true); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.StreamSend(stream, abi.ByteView{Ptr: []byte{byte(second)}}); err != nil {
			t.Fatal(err)
		}
		events.advance(events.now)
	}
	events.advance(int64(5 * time.Second))
	if !stream.closed {
		t.Fatal("new writes postponed the missing ACK deadline")
	}
	if peer.incomingAvailable != 0 {
		t.Fatal("data after a dropped offset was delivered")
	}
	replacement, err := client.OpenStream(netip.AddrPortFrom(testPair.DestHost, 80))
	if err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	replacementPeer, err := server.Accept(listener)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.StreamSend(replacement, abi.ByteView{Ptr: []byte{42}}); err != nil {
		t.Fatal(err)
	}
	events.advance(events.now)
	data := make([]byte, 1)
	if count, err := server.StreamRecv(replacementPeer, abi.ByteView{Ptr: data}); err != nil || count != 1 ||
		data[0] != 42 {
		t.Fatalf("replacement connection: %d %v %v", count, err, data)
	}
}

func TestAckProgressKeepsBusyStreamAlive(t *testing.T) {
	t.Parallel()
	n, events, client, server := liveNetwork(t)
	stream, _, _ := connect(t, events, client, server)
	if err := n.SetDelay(testPair, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := n.SetDelay(testPair.Flip(), time.Second); err != nil {
		t.Fatal(err)
	}
	for second := 0; second < 12; second++ {
		events.advance(int64(time.Duration(second) * time.Second))
		if _, err := client.StreamSend(stream, abi.ByteView{Ptr: []byte{byte(second)}}); err != nil {
			t.Fatalf("second %d: %v", second, err)
		}
	}
	events.advance(int64(20 * time.Second))
	if stream.closed || stream.ackPos != stream.sendPos {
		t.Fatalf("healthy stream: closed=%v ack=%d send=%d", stream.closed, stream.ackPos, stream.sendPos)
	}
}
