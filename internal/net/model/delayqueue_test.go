package model

import (
	"net/netip"
	"testing"
	"time"
)

func TestQueuePurgeKeepsTotalOrderAndClearsReferences(t *testing.T) {
	events := newTestEvents()
	var delivered []uint64
	q := newDelayQueue(events, events, func(p *Packet) { delivered = append(delivered, p.ArrivalTime.index); freePacket(p) })
	other := HostPair{testPair.SourceHost, netip.MustParseAddr("10.0.0.3")}
	for i := 1; i <= 6; i++ {
		pair := other
		if i == 2 {
			pair = testPair
		}
		if i == 4 {
			pair = testPair.Flip()
		}
		p := allocPacket(PacketKindStreamData, ConnId{Hosts: pair})
		p.ArrivalTime = timeIndex{time: 10, index: uint64(i)}
		q.enqueue(p)
	}
	backing := q.items[:cap(q.items)]
	if q.dropPair(edgeKey(testPair)) != 1 || q.dropPair(edgeKey(testPair.Flip())) != 1 {
		t.Fatal("directional purge")
	}
	for _, p := range backing[len(q.items):] {
		if p != nil {
			t.Fatal("retained packet")
		}
	}
	events.advance(10)
	for i, want := range []uint64{1, 3, 5, 6} {
		if len(delivered) != 4 || delivered[i] != want {
			t.Fatal("unstable heap order", delivered)
		}
	}
	for _, p := range backing {
		if p != nil {
			t.Fatal("dequeue retained reference")
		}
	}
	if len(events.pending) != 0 {
		t.Fatal("empty queue scheduled")
	}
	p := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	p.ArrivalTime = timeIndex{time: int64(time.Hour), index: 7}
	q.enqueue(p)
	if q.dropPair(edgeKey(testPair)) != 1 || len(events.pending) != 0 {
		t.Fatal("empty queue callback retained")
	}
}

func TestQueuePurgeRecalculatesWakeup(t *testing.T) {
	events := newTestEvents()
	var delivered []*Packet
	q := newDelayQueue(events, events, func(p *Packet) { delivered = append(delivered, p) })
	other := HostPair{testPair.SourceHost, netip.MustParseAddr("10.0.0.3")}
	future := allocPacket(PacketKindStreamData, ConnId{Hosts: other})
	future.ArrivalTime = timeIndex{time: 100, index: 1}
	q.enqueue(future)
	head := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	head.ArrivalTime = timeIndex{time: 0, index: 2}
	q.enqueue(head)
	stale := events.pending[q.event].fn
	if q.dropPair(edgeKey(testPair)) != 1 || events.pending[q.event].at != 100 {
		t.Fatal("did not rearm survivor")
	}
	stale()
	if len(delivered) != 0 {
		t.Fatal("retired callback delivered")
	}
	events.advance(99)
	if len(delivered) != 0 {
		t.Fatal("early delivery")
	}
	events.advance(100)
	if len(delivered) != 1 || delivered[0] != future {
		t.Fatal("missing survivor")
	}
	due := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	due.ArrivalTime = timeIndex{time: 100, index: 3}
	q.enqueue(due)
	events.advance(100)
	if len(delivered) != 2 || delivered[1] != due {
		t.Fatal("empty queue failed to wake")
	}
}
