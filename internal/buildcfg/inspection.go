package buildcfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

// InspectionCompiler binds an observation to an exact installation. It is not
// an authorization capability; source collectors check their inputs themselves.
type InspectionCompiler struct {
	toolchain    buildspec.Toolchain
	fingerprints buildspec.Fingerprints
}

func (c InspectionCompiler) Toolchain() buildspec.Toolchain       { return c.toolchain }
func (c InspectionCompiler) Fingerprints() buildspec.Fingerprints { return c.fingerprints }

func DiscoverInspection(ctx context.Context, directory, expectedVersion string) (InspectionCompiler, error) {
	if expectedVersion != "" && expectedVersion != runtime.Version() {
		return InspectionCompiler{}, fmt.Errorf("inspection requires running compiler %s, requested %s", runtime.Version(), expectedVersion)
	}
	root, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		return InspectionCompiler{}, err
	}
	tc := buildspec.Toolchain{Executable: filepath.Join(root, "bin", "go"), GOROOT: root, Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return InspectionCompiler{}, err
	}
	cmd := buildspec.SourceInspectionCommand(ctx, tc, directory, "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH", "GOMOD", "GOWORK", "GOMODCACHE")
	// Discovery observes the caller's workspace; source commands later use a
	// private module with GOWORK=off. Empty GOWORK retains Go's auto-discovery.
	cmd.Env = append(withoutEnv(cmd.Env, "GOWORK"), "GOWORK="+os.Getenv("GOWORK"))
	r, err := resolveDiscovery(ctx, cmd, func(version string) error { _, err := buildspec.ParseGoVersion(version); return err })
	if err != nil {
		return InspectionCompiler{}, err
	}
	return inspectCompiler(ctx, r.Toolchain)
}

func inspectCompiler(ctx context.Context, tc buildspec.Toolchain) (InspectionCompiler, error) {
	if err := validateInspectionIdentity(ctx, tc); err != nil {
		return InspectionCompiler{}, err
	}
	fp, err := fingerprintInstalledContext(ctx, tc.GOROOT)
	if err != nil {
		return InspectionCompiler{}, err
	}
	return InspectionCompiler{toolchain: tc, fingerprints: fp}, nil
}

func validateInspectionIdentity(ctx context.Context, tc buildspec.Toolchain) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := buildspec.ParseGoVersion(tc.Version); err != nil {
		return err
	}
	if err := validateWorkerIdentity(tc); err != nil {
		return err
	}
	if tc.Executable != filepath.Join(tc.GOROOT, "bin", "go") {
		return fmt.Errorf("inspection executable is outside selected distribution")
	}
	out, err := buildspec.SourceInspectionCommand(ctx, tc, "", "version").Output()
	if err != nil {
		return fmt.Errorf("inspect compiler identity: %w", err)
	}
	want := "go version " + tc.Version + " " + tc.HostOS + "/" + tc.HostArch
	if strings.TrimSpace(string(out)) != want {
		return fmt.Errorf("inspection executable identity mismatch: got %q, want %q", out, want)
	}
	return nil
}

func (c InspectionCompiler) Recheck(ctx context.Context) error {
	after, err := inspectCompiler(ctx, c.toolchain)
	if err != nil {
		return err
	}
	if after.fingerprints != c.fingerprints {
		return fmt.Errorf("compiler installation changed during inspection")
	}
	return nil
}
