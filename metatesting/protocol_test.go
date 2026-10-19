//go:build !sim

package metatesting

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("GOSIM_PROTOCOL_TEST_CHILD"); mode != "" {
		protocolChild(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func protocolChild(mode string) {
	_ = os.WriteFile(os.Getenv("GOSIM_PROTOCOL_TEST_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	var hello simrt.MetatestHandshake
	_ = json.Unmarshal([]byte(os.Getenv("GOSIM_PROTOCOL_TEST_HELLO")), &hello)
	switch mode {
	case "protocol":
		hello.Protocol = 1
	case "identity":
		hello.IdentityDigest = strings.Repeat("b", 64)
	case "patch":
		hello.Toolchain = "go1.25.13"
	case "timeout":
		io.Copy(io.Discard, os.Stdin)
		return
	}
	input, output := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	if err := output.Encode(hello); err != nil {
		return
	}
	var peer simrt.MetatestHandshake
	if err := input.Decode(&peer); err != nil || peer != hello {
		return
	}
	for {
		var request RunConfig
		if err := input.Decode(&request); err != nil {
			return
		}
		switch mode {
		case "crash":
			os.Exit(7)
		case "blocked":
			_ = os.WriteFile(os.Getenv("GOSIM_PROTOCOL_TEST_PID")+".request", nil, 0o600)
			io.Copy(io.Discard, os.Stdin)
			return
		}
		if request.Test == "listtests" {
			_ = output.Encode([]string{"TestHello"})
			continue
		}
		result := RunResult{Seed: request.Seed, BuildDigest: hello.IdentityDigest, ReplayRevision: simrt.ReplayRevision}
		if mode == "result" {
			result.BuildDigest = "wrong"
		}
		_ = output.Encode(result)
	}
}

func protocolFixture(t *testing.T, mode string) (string, Manifest, string) {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d := strings.Repeat("a", 64)
	profile := "go126"
	if strings.HasPrefix(runtime.Version(), "go1.25.") {
		profile = "go125"
	}
	id := buildspec.Identity{Schema: 1, Toolchain: runtime.Version(), Profile: profile, DistributionDigest: d, StdlibDigest: d, AdapterRevision: d, RuntimeRevision: d, HostOS: runtime.GOOS, HostArch: runtime.GOARCH, SimulationOS: "linux", SimulationArch: runtime.GOARCH, Features: []string{"rt"}, Tags: []string{"sim", "linkname"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d, Certification: "candidate"}
	digest, err := buildspec.Digest(id)
	if err != nil {
		t.Fatal(err)
	}
	hello, _ := json.Marshal(simrt.MetatestHandshake{Protocol: 2, IdentityDigest: digest, Toolchain: runtime.Version()})
	pid := filepath.Join(t.TempDir(), "pid")
	t.Setenv("GOSIM_PROTOCOL_TEST_CHILD", mode)
	t.Setenv("GOSIM_PROTOCOL_TEST_PID", pid)
	t.Setenv("GOSIM_PROTOCOL_TEST_HELLO", string(hello))
	return path, Manifest{Schema: 1, Identity: id, Binary: gosimtool.ArtifactFile{Path: filepath.Base(path), SHA256: buildspec.Hash(binary)}}, pid
}

func assertReaped(t *testing.T, pidfile string) {
	t.Helper()
	data, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	var state syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &state, syscall.WNOHANG, nil); !errors.Is(err, syscall.ECHILD) {
		t.Fatalf("worker %d was not reaped: %v", pid, err)
	}
}

func TestProtocolRejectsAndReaps(t *testing.T) {
	for _, mode := range []string{"protocol", "identity", "patch", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			path, manifest, pid := protocolFixture(t, mode)
			runner, err := startRunner(path, manifest, 500*time.Millisecond)
			if err == nil || runner != nil {
				t.Fatal("accepted bad handshake", mode)
			}
			assertReaped(t, pid)
		})
	}
	t.Run("binary", func(t *testing.T) {
		path, manifest, pid := protocolFixture(t, "ok")
		manifest.Binary.SHA256 = strings.Repeat("b", 64)
		if _, err := newRunner(path, manifest); err == nil {
			t.Fatal("accepted changed binary")
		}
		if _, err := os.Stat(pid); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("launched changed binary", err)
		}
	})
}

func TestProtocolConcurrentRequests(t *testing.T) {
	path, manifest, pid := protocolFixture(t, "ok")
	runner, err := newRunner(path, manifest)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			if i%2 == 0 {
				tests, err := runner.ListTests()
				if err != nil || len(tests) != 1 {
					t.Errorf("list: %v %v", tests, err)
				}
			} else {
				result, err := runner.Run(t, &RunConfig{Test: "TestHello", Seed: int64(i)})
				if err != nil {
					t.Error(err)
				} else if result.Seed != int64(i) {
					t.Error("crossed responses", result.Seed, i)
				}
			}
		})
	}
	wg.Wait()
	for range 4 {
		wg.Go(func() {
			if err := runner.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if _, err := runner.ListTests(); err == nil {
		t.Fatal("closed runner accepted request")
	}
	assertReaped(t, pid)
}

func TestProtocolFailureAndEarlyClose(t *testing.T) {
	for _, mode := range []string{"crash", "result", "blocked"} {
		t.Run(mode, func(t *testing.T) {
			path, manifest, pid := protocolFixture(t, mode)
			runner, err := newRunner(path, manifest)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := runner.Run(t, &RunConfig{Test: "TestHello", Seed: 1}); done <- err }()
			if mode == "blocked" {
				deadline := time.Now().Add(5 * time.Second)
				for {
					if _, err := os.Stat(pid + ".request"); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("child never received request")
					}
					time.Sleep(time.Millisecond)
				}
				if err := runner.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("accepted broken request")
				}
			case <-time.After(6 * time.Second):
				t.Fatal("request did not unblock")
			}
			if runner.cmd.ProcessState == nil {
				t.Fatal("failed request did not wait for child")
			}
			assertReaped(t, pid)
			if err := runner.Close(); mode == "crash" && err == nil {
				t.Fatal(fmt.Errorf("lost crash status"))
			}
		})
	}
}
