package buildcfg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/testutil"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestInspectionCompilerIdentityAndRecheck(t *testing.T) {
	ctx := context.Background()
	c, err := DiscoverInspection(ctx, t.TempDir(), runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	if c.Toolchain().Version != runtime.Version() || !buildspec.ValidDigest(c.Fingerprints().StdlibDigest) || !buildspec.ValidDigest(c.Fingerprints().ToolchainDigest) {
		t.Fatal(c)
	}
	if err := c.Recheck(ctx); err != nil {
		t.Fatal(err)
	}
	changed := c
	changed.fingerprints.StdlibDigest = strings.Repeat("0", 64)
	if err := changed.Recheck(ctx); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
	if _, err := DiscoverInspection(ctx, ".", testutil.UnregisteredGo().Version); err == nil {
		t.Fatal("accepted override mismatch")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := c.Recheck(cancelled); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := (InspectionCompiler{}).Recheck(ctx); err == nil {
		t.Fatal("accepted zero state")
	}
	t.Setenv("GOFLAGS", "-overlay=unapproved.json")
	if _, err := DiscoverInspection(ctx, ".", ""); err == nil {
		t.Fatal("ignored environment")
	}
}

func TestInspectionFixturesDoNotSpoofRunningCompiler(t *testing.T) {
	ctx := context.Background()
	root, _, _ := fakeDiscoveryGo(t, testutil.UnregisteredGo().Version)
	tc := buildspec.Toolchain{Executable: filepath.Join(root, "bin", "go"), GOROOT: root, Version: testutil.UnregisteredGo().Version, HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	if _, err := buildspec.SourceInspectionCommand(ctx, tc, root, "version").Output(); err != nil {
		t.Fatal(err)
	}
	if _, err := Fingerprint(tc); err == nil {
		t.Fatal("ordinary fingerprint accepted future family")
	}
	if _, err := inspectCompiler(ctx, tc); err == nil || !strings.Contains(err.Error(), "worker is") {
		t.Fatal("fake future must fail runtime equality", err)
	}
	tc.Version = runtime.Version()
	if _, err := inspectCompiler(ctx, tc); err == nil || !strings.Contains(err.Error(), "GOROOT") {
		t.Fatal("fake current must fail installation equality", err)
	}
	// Shared decoding also verifies the executable against its env identity.
	data, err := os.ReadFile(tc.Executable)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "go version "+testutil.UnregisteredGo().Version, "go version invalid", 1))
	if err := os.WriteFile(tc.Executable, data, 0o700); err != nil {
		t.Fatal(err)
	}
	tc.Version = testutil.UnregisteredGo().Version
	cmd := buildspec.SourceInspectionCommand(ctx, tc, root, "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH")
	if _, err := resolveDiscovery(ctx, cmd, func(v string) error { _, err := buildspec.ParseGoVersion(v); return err }); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatal(err)
	}
	// Normal discovery still rejects unknown stable versions before fingerprinting.
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := Discover(ctx, root, testutil.UnregisteredGo().Version); err == nil {
		t.Fatal("ordinary discovery admitted future")
	}
}
