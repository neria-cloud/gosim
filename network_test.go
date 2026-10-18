//go:build !sim

package gosim

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

func TestNetworkDefaultsAndOutsideSimulation(t *testing.T) {
	c := DefaultNetworkConfig()
	if c.Latency.Min != 0 || c.Latency.Max != 100*time.Millisecond || c.Latency.Lambda != 5 || c.Loss.FailProbability != 0 || c.Loss.RepairProbability != 1 {
		t.Fatal(c)
	}
	c.Latency.Lambda = 1
	if DefaultNetworkConfig().Latency.Lambda != 5 {
		t.Fatal("default is aliased")
	}
	operations := []func() error{
		func() error { return ConfigureNetwork(c) },
		func() error { return SetMaxMessageLatency(time.Second) },
		func() error { return SetMessageLatencyCurve(2.5) },
		func() error { return SetFailRate(0.125) }, func() error { return SetRepairRate(0.5) },
		func() error { return SetLinkLatency("a", "b", 0) }, func() error { return SetLinkMaxMessageLatency("a", "b", 0) },
		func() error { return SetLinkMessageLatencyCurve("a", "b", 2.5) }, func() error { return SetLinkFailRate("a", "b", 0.125) },
		func() error { return SetLinkRepairRate("a", "b", 0.5) }, func() error { return ResetLinkLatency("a", "b") }, func() error { return ResetLinkLoss("a", "b") },
		func() error { return Partition("a", "b") }, func() error { return Repair("a", "b") },
		func() error { return PartitionOneWay("a", "b") }, func() error { return RepairOneWay("a", "b") },
		func() error { return SetDelay("a", "b", 0) }, func() error { return SetConnected("a", "b", true) },
	}
	for i, op := range operations {
		if err := op(); !errors.Is(err, syscall.ENOSYS) {
			t.Fatalf("operation %d: %v", i, err)
		}
	}
}
