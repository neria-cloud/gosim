//go:build !sim

package behavior_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/neria-cloud/gosim/metatesting"
)

func TestNetworkTrace(t *testing.T) {
	t.Parallel()
	runner := metatesting.ForPackage(t, metatesting.Options{})
	run, err := runner.Run(t, &metatesting.RunConfig{Test: "TestNetworkConfigValidation", Seed: 1, Simtrace: "network"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Failed {
		t.Fatal("traced validation failed")
	}
	foundGlobal, foundLocal := false, false
	for _, line := range bytes.Split(run.LogOutput, []byte{'\n'}) {
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var record struct {
			Message           string `json:"msg"`
			TraceKind         string `json:"traceKind"`
			Lambda            string `json:"latency.lambda"`
			FailProbability   string `json:"loss.failProbability"`
			RepairProbability string `json:"loss.repairProbability"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record.TraceKind != "network" {
			continue
		}
		if record.Message == "network.configure_global" && record.Lambda == "2.5" &&
			record.FailProbability == "0.125" && record.RepairProbability == "0.375" {
			foundGlobal = true
		}
		if record.Message == "network.configure_link" && record.Lambda == "0.25" &&
			record.FailProbability == "0.125" && record.RepairProbability == "0.375" {
			foundLocal = true
		}
	}
	if !foundGlobal || !foundLocal {
		t.Fatalf("fractional values not preserved by ABI/tracing: global=%v local=%v", foundGlobal, foundLocal)
	}
	config := &metatesting.RunConfig{Test: "TestNetworkConfigFiveHosts", Seed: 1, Simtrace: "network"}
	first, err := runner.Run(t, config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Run(t, config)
	if err != nil {
		t.Fatal(err)
	}
	if first.Failed || second.Failed {
		t.Fatal("traced cluster failed")
	}
	if !bytes.Equal(first.Checksum, second.Checksum) || !bytes.Equal(first.LogOutput, second.LogOutput) {
		t.Fatal("traced network replay differs")
	}
	for _, event := range []string{
		"network.enqueue", "network.random_partition", "network.manual_partition",
		"network.drop", "network.manual_repair", "network.deliver_admitted",
	} {
		if !bytes.Contains(first.LogOutput, []byte(event)) {
			t.Errorf("missing %s", event)
		}
	}
	if bytes.Contains(first.LogOutput, []byte("ping")) {
		t.Fatal("network trace contains application payload")
	}
}

func TestRestartDiscardsQueuedOldPacket(t *testing.T) {
	t.Parallel()
	runner := metatesting.ForPackage(t, metatesting.Options{})
	result, err := runner.Run(t, &metatesting.RunConfig{
		Test: "TestRestartReusesEndpointWithoutOldConnection", Seed: 1, Simtrace: "network",
	})
	if err != nil || result.Failed {
		t.Fatalf("restart trace: %v %+v", err, result)
	}
	found := false
	for _, line := range bytes.Split(result.LogOutput, []byte{'\n'}) {
		var record map[string]any
		if json.Unmarshal(line, &record) == nil && record["msg"] == "network.enqueue" &&
			record["kind"] == "3" && record["sampled"] == "1s" && record["from"] == "10.50.0.2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture did not enqueue delayed old data: %s", result.LogOutput)
	}
}
