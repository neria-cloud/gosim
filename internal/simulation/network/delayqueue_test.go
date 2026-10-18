package network

import (
	"net/netip"
	"testing"
	"time"
)

func TestQueuePurgeKeepsTotalOrderAndClearsReferences(t *testing.T) {
	q := newDelayQueue()
	defer q.timer.Stop()
	other := HostPair{testPair.SourceHost, netip.MustParseAddr("10.0.0.3")}
	now := time.Now().UnixNano()
	for i := 1; i <= 6; i++ {
		pair := other
		if i == 2 {
			pair = testPair
		}
		if i == 4 {
			pair = testPair.Flip()
		}
		p := allocPacket(PacketKindStreamData, ConnId{Hosts: pair})
		p.ArrivalTime = timeIndex{time: now, index: uint64(i)}
		q.enqueue(p)
	}
	backing := q.items[:cap(q.items)]
	if count := q.dropPair(canonicalPair(testPair)); count != 2 {
		t.Fatal(count)
	}
	for _, p := range backing[len(q.items):] {
		if p != nil {
			t.Fatal("backing array retains packet")
		}
	}
	for _, want := range []uint64{1, 3, 5, 6} {
		p := q.dequeue()
		if p.ArrivalTime.index != want {
			t.Fatal("heap rebuild reordered equal arrivals", p.ArrivalTime.index, want)
		}
		freePacket(p)
	}
	if len(q.items) != 0 {
		t.Fatal("queue not drained")
	}
	for _, p := range backing {
		if p != nil {
			t.Fatal("dequeue retains reference")
		}
	}
	// Removing the final future packet must stop its timer.
	p := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	p.ArrivalTime = timeIndex{time: time.Now().Add(time.Hour).UnixNano(), index: 7}
	q.enqueue(p)
	if q.dropPair(canonicalPair(testPair)) != 1 || q.timer.Stop() {
		t.Fatal("empty queue timer still active")
	}
}

func TestQueuePurgeRecalculatesWakeup(t *testing.T) {
	q := newDelayQueue()
	defer q.timer.Stop()
	// Removing an already-due head must restart the stopped timer for its
	// future survivor. Inspect timer activity without wall-clock sleeps.
	other := HostPair{testPair.SourceHost, netip.MustParseAddr("10.0.0.3")}
	future := allocPacket(PacketKindStreamData, ConnId{Hosts: other})
	future.ArrivalTime = timeIndex{time: time.Now().Add(time.Hour).UnixNano(), index: 1}
	q.enqueue(future)
	head := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	head.ArrivalTime = timeIndex{time: time.Now().UnixNano(), index: 2}
	q.enqueue(head)
	if q.timer.Stop() {
		t.Fatal("due head should stop timer")
	}
	if q.dropPair(canonicalPair(testPair)) != 1 || q.items[0] != future || !q.timer.Stop() {
		t.Fatal("purge did not schedule surviving head")
	}
	q.dropPair(canonicalPair(other))
	done := make(chan *Packet, 1)
	go func() { done <- q.dequeue() }()
	due := allocPacket(PacketKindStreamData, ConnId{Hosts: testPair})
	due.ArrivalTime = timeIndex{time: time.Now().UnixNano(), index: 2}
	q.enqueue(due)
	select {
	case got := <-done:
		if got != due {
			t.Fatal("wrong dequeue")
		}
		freePacket(got)
	case <-time.After(time.Second):
		t.Fatal("queue failed to wake after purge")
	}
}
