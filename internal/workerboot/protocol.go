package workerboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Info struct {
	Protocol  int                  `json:"protocol"`
	Toolchain string               `json:"toolchain"`
	Compiler  string               `json:"compiler"`
	HostOS    string               `json:"host_os"`
	HostArch  string               `json:"host_arch"`
	GOROOT    string               `json:"go_root"`
	BuildPath string               `json:"build_path"`
	Module    debug.Module         `json:"module"`
	Settings  []debug.BuildSetting `json:"settings"`
}

func CurrentInfo() (Info, error) {
	info := Info{
		Protocol:  Protocol,
		Toolchain: runtime.Version(),
		Compiler:  runtime.Compiler,
		HostOS:    runtime.GOOS,
		HostArch:  runtime.GOARCH,
		GOROOT:    runtime.GOROOT(),
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info, errors.New("worker has no Go build information")
	}
	info.BuildPath, info.Module, info.Settings = build.Path, build.Main, build.Settings
	return info, nil
}

func validateSettings(settings []debug.BuildSetting, hostOS, hostArch string, trimpath bool) error {
	values := map[string]string{}
	for _, s := range settings {
		values[s.Key] = s.Value
	}
	for key, want := range map[string]string{"GOOS": hostOS, "GOARCH": hostArch, "-tags": "linkname"} {
		if values[key] != want {
			return fmt.Errorf("worker build setting %s=%q, expected %q", key, values[key], want)
		}
	}
	if trimpath && values["-trimpath"] != "true" {
		return errors.New("cached worker must use -trimpath")
	}
	// cmd/go omits all -ldflags from build info with -trimpath (Go issue
	// 52372). The cache key binds our build command and the manifest binds its
	// output bytes. Validate linker flags whenever the compiler records them.
	if flags := values["-ldflags"]; flags != "-checklinkname=0" && (flags != "" || values["-trimpath"] != "true") {
		return fmt.Errorf("unapproved worker linker flags %q", flags)
	}
	for _, key := range []string{"-race", "-asan", "-msan", "GOEXPERIMENT", "-gcflags", "-asmflags", "-toolexec"} {
		if values[key] != "" && values[key] != "false" {
			return fmt.Errorf("unapproved worker build setting %s=%q", key, values[key])
		}
	}
	return nil
}

type boundedOutput struct {
	buffer bytes.Buffer
	max    int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.buffer.Len()+len(p) > b.max {
		return 0, errors.New("worker handshake exceeded output limit")
	}
	return b.buffer.Write(p)
}

// Do not embed bytes.Buffer: its promoted ReadFrom would let os/exec's io.Copy
// bypass Write and the handshake size limit.
func (b *boundedOutput) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedOutput) String() string { return b.buffer.String() }

func Probe(ctx context.Context, path string, r buildspec.Resolution) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--worker-info")
	cmd.Env = buildspec.ApplicationCommand(ctx, r, "env").Env
	cmd.Dir = r.Context.Directory
	output := &boundedOutput{max: 64 * 1024}
	diagnostics := &boundedOutput{max: 64 * 1024}
	cmd.Stdout, cmd.Stderr = output, diagnostics
	if err := buildspec.RunProcess(ctx, cmd); err != nil {
		return Info{}, fmt.Errorf("worker handshake: %w\n%s", err, diagnostics.String())
	}
	var info Info
	dec := json.NewDecoder(bytes.NewReader(output.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&info); err != nil {
		return info, fmt.Errorf("invalid worker handshake: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return info, errors.New("trailing worker handshake data")
	}
	tc := r.Toolchain
	root, err := filepath.EvalSymlinks(info.GOROOT)
	if err != nil {
		return info, err
	}
	if info.Protocol != Protocol || info.Compiler != "gc" || info.Toolchain != tc.Version || info.HostOS != tc.HostOS ||
		info.HostArch != tc.HostArch ||
		root != tc.GOROOT ||
		info.BuildPath != WorkerPackage ||
		info.Module.Path != buildcfg.GosimModule {
		return info, errors.New("worker protocol/compiler/platform differs from resolved Go")
	}
	return info, nil
}

func Verify(ctx context.Context, w Worker, r buildspec.Resolution) error {
	info, err := Probe(ctx, w.Path, r)
	if err != nil {
		return err
	}
	if info.Toolchain != w.Manifest.Toolchain || info.HostOS != w.Manifest.HostOS ||
		info.HostArch != w.Manifest.HostArch {
		return errors.New("worker handshake differs from cache manifest")
	}
	if !matchesSelectedModuleVersion(w.Manifest.ModuleVersion, info.Module.Version) {
		return fmt.Errorf(
			"worker module version %q differs from application selection %q",
			info.Module.Version,
			w.Manifest.ModuleVersion,
		)
	}
	return validateSettings(info.Settings, info.HostOS, info.HostArch, true)
}

func matchesSelectedModuleVersion(selected, built string) bool {
	// Go lists a local main module without a version, but may stamp its
	// binary with a version derived from VCS. Its source and binary digests
	// are bound by the worker manifest instead.
	return selected == "" || selected == built
}

func WriteInfo(w io.Writer) error {
	info, err := CurrentInfo()
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(info)
}
