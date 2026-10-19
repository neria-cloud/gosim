package candidate

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/race"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func checkDisabledFixtures(t *testing.T, cfg buildspec.Config) {
	t.Helper()
	for _, feature := range []string{"fs", "net"} {
		if slices.Contains(cfg.Features, feature) {
			continue
		}
		built, err := Build(cfg, buildspec.BuildOptions{Race: race.Enabled}, []string{"./internal/tests/features/testdata/disabled" + feature})
		if err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"init", "test"} {
			t.Run("disabled-"+feature+"-"+phase, func(t *testing.T) {
				probe := filepath.Join(t.TempDir(), "forbidden")
				var listener *net.TCPListener
				if feature == "net" {
					address, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					listener, err = net.ListenTCP("tcp", address)
					if err != nil {
						t.Fatal(err)
					}
					defer listener.Close()
					probe = listener.Addr().String()
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				result, err := Run(ctx, built.Artifacts[0], simrt.RunConfig{Test: "TestDisabled", Seed: 42, Simtrace: "syscall", ExtraEnv: []string{"DISABLED_PHASE=" + phase, "HOST_PROBE=" + probe}})
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"FeatureDisabled", "feature=" + feature, "machine=1 epoch=1", "operation="} {
					if !result.Failed || !strings.Contains(result.Err, want) {
						t.Fatalf("missing %q in failure: %+v", want, result)
					}
				}
				if strings.Contains(result.Err, "returned") || strings.Contains(result.Err, "unwound") || strings.Contains(result.Err, "operation= ") {
					t.Fatalf("application recovered disabled call: %+v", result)
				}
				pc := regexp.MustCompile(`pc=(0x[0-9a-f]+)`).FindStringSubmatch(result.Err)
				if len(pc) != 2 || pc[1] == "0x0" {
					t.Fatalf("missing caller PC: %s", result.Err)
				}
				callerFound := false
				for _, line := range bytes.Split(result.LogOutput, []byte{'\n'}) {
					var record struct {
						Message string `json:"msg"`
						Source  struct {
							Function, File string
							Line           int
						} `json:"source"`
					}
					if json.Unmarshal(line, &record) == nil && record.Message == "call Require" && record.Source.Function != "" && record.Source.File != "" && record.Source.Line > 0 {
						callerFound = true
					}
				}
				if !callerFound {
					t.Fatalf("missing resolved caller in trace: %s", result.LogOutput)
				}
				if feature == "fs" {
					if _, err := os.Stat(probe); !os.IsNotExist(err) {
						t.Fatal("disabled filesystem reached host", err)
					}
				} else {
					listener.SetDeadline(time.Now().Add(10 * time.Millisecond))
					conn, err := listener.Accept()
					if err == nil {
						conn.Close()
						t.Fatal("disabled network contacted host")
					}
					if err, ok := err.(net.Error); !ok || !err.Timeout() {
						t.Fatal("host contact probe failed", err)
					}
				}
			})
		}
	}
}
