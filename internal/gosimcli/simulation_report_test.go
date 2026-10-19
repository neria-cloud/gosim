package gosimcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSimulationReportProcess(t *testing.T) {
	mode := os.Getenv("GOSIM_REPORT_TEST_CHILD")
	if mode == "" {
		return
	}
	fmt.Println("=== RUN   TestExample (seed 1)")
	if mode == "crash" {
		fmt.Println("panic: fixture failure")
		os.Exit(2)
	}
	fmt.Println("=== RUN   TestExample/child (seed 1)")
	fmt.Println("    --- SKIP: TestExample/child (0.00s simulated)")
	fmt.Println("--- PASS: TestExample (3600.00s simulated)")
	if mode == "fail" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestSimulationReportClosesTestsAndPackage(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"pass", "fail", "crash"} {
		t.Run(mode, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			binary := exec.Command(executable, "-test.run=^TestSimulationReportProcess$")
			binary.Env = append(os.Environ(), "GOSIM_REPORT_TEST_CHILD="+mode)
			converter := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "tool", "test2json", "-t", "-p", "fixture/simulation")
			var streamed bytes.Buffer
			data, err := captureSimulationCommand(t.Context(), binary, converter, new(sync.Mutex), &streamed)
			if (err == nil) != (mode == "pass") {
				t.Fatalf("%s: %v", mode, err)
			}
			if !bytes.Equal(data, streamed.Bytes()) {
				t.Fatal("stream and persisted events differ")
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			starts, ends := map[string]int{}, map[string]string{}
			terminal := ""
			for decoder.More() {
				var event struct {
					Action, Test string
					Elapsed      float64
				}
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(event.Test, "seed") {
					t.Fatalf("seed became unmatched test name: %q", event.Test)
				}
				if event.Action == "run" {
					starts[event.Test]++
				}
				if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
					if event.Test == "" {
						terminal = event.Action
					} else {
						ends[event.Test] = event.Action
					}
					if event.Elapsed > 30 {
						t.Fatalf("simulated time reported as wall time: %v", event.Elapsed)
					}
				}
			}
			want := "pass"
			if mode != "pass" {
				want = "fail"
			}
			if terminal != want {
				t.Fatalf("package result %q, want %q\n%s", terminal, want, data)
			}
			if mode != "crash" && (starts["TestExample"] != 1 || starts["TestExample/child"] != 1 || ends["TestExample"] != "pass" || ends["TestExample/child"] != "skip") {
				t.Fatalf("mismatched events: %v %v\n%s", starts, ends, data)
			}
		})
	}
}

func TestSimulationReportConverterStartFailure(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary := exec.Command(executable, "-test.run=^TestSimulationReportProcess$")
	binary.Env = append(os.Environ(), "GOSIM_REPORT_TEST_CHILD=pass")
	converter := exec.Command(filepath.Join(t.TempDir(), "missing"))
	if _, err := captureSimulationCommand(t.Context(), binary, converter, new(sync.Mutex), nil); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("lost converter start failure", err)
	}
}
