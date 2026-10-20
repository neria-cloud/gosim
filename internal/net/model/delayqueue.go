package model

import (
	"container/heap"
	"math"

	"github.com/neria-cloud/gosim/internal/contracts"
)

const initialDelayQueueCapacity = 16

type timeIndex struct {
	// XXX: can we pack these together somehow?
	time  int64
	index uint64
}

func (ti timeIndex) Less(other timeIndex) bool {
	if ti.time != other.time {
		return ti.time < other.time
	}
	return ti.index < other.index
}

type timedHeap []*Packet

func (h timedHeap) Len() int           { return len(h) }
func (h timedHeap) Less(i, j int) bool { return h[i].ArrivalTime.Less(h[j].ArrivalTime) }
func (h timedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *timedHeap) Push(x any) {
	panic("don't use")
	// *h = append(*h, x.(timedPacket))
}

func (h *timedHeap) Pop() any {
	panic("don't use")
	// old := *h
	// n := len(old)
	// x := old[n-1]
	// *h = old[0 : n-1]
	// return x
}

type delayQueue struct {
	items      timedHeap
	clock      contracts.Clock
	events     contracts.Events
	deliver    func(*Packet)
	event      contracts.EventID
	generation uint64
}

func newDelayQueue(clock contracts.Clock, events contracts.Events, deliver func(*Packet)) *delayQueue {
	return &delayQueue{
		clock: clock, events: events, deliver: deliver,
		items: make(timedHeap, 0, initialDelayQueueCapacity),
	}
}

func (q *delayQueue) updateTimer() {
	if q.event != 0 {
		q.events.Cancel(q.event)
		q.event = 0
	}
	if q.generation == math.MaxUint64 {
		panic("network queue generation exhausted")
	}
	q.generation++
	if len(q.items) == 0 {
		return
	}
	generation := q.generation
	head := q.items[0]
	id, err := q.events.Schedule(head.DestinationEpoch, head.ArrivalTime.time, func() {
		if generation != q.generation {
			return
		}
		q.event = 0
		for len(q.items) > 0 && q.items[0].ArrivalTime.time <= q.clock.NowNanos() {
			p := q.pop()
			q.deliver(p)
			// Delivery may enqueue a reply and rearm the queue. It remains owner-serialized.
		}
		q.updateTimer()
	})
	if err != nil {
		panic(err)
	}
	q.event = id
}

func (q *delayQueue) enqueue(p *Packet) {
	q.items = append(q.items, p)
	heap.Fix(&q.items, len(q.items)-1)
	q.updateTimer()
}

func (q *delayQueue) pop() *Packet {
	p := q.items[0]
	last := len(q.items) - 1
	q.items[0] = q.items[last]
	q.items[last] = nil
	q.items = q.items[:last]
	if last > 0 {
		heap.Fix(&q.items, 0)
	}
	return p
}

// drop preserves surviving arrival and sequence values, and clears retained pointers.
func (q *delayQueue) drop(matches func(*Packet) bool) int {
	oldLen, write := len(q.items), 0
	for _, p := range q.items {
		if matches(p) {
			freePacket(p)
		} else {
			q.items[write] = p
			write++
		}
	}
	clear(q.items[write:oldLen])
	q.items = q.items[:write]
	heap.Init(&q.items)
	q.updateTimer()
	return oldLen - write
}

func (q *delayQueue) dropPair(key linkKey) int {
	return q.drop(func(p *Packet) bool { return edgeKey(p.ConnId.Hosts) == key })
}
