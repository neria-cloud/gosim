package workerboot

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

const Protocol = 1

type WorkerManifest struct {
	Schema          int    `json:"schema"`
	Protocol        int    `json:"protocol"`
	Key             string `json:"key"`
	Toolchain       string `json:"toolchain"`
	ToolchainDigest string `json:"toolchain_digest"`
	StdlibDigest    string `json:"stdlib_digest"`
	HostOS          string `json:"host_os"`
	HostArch        string `json:"host_arch"`
	SourceDigest    string `json:"source_digest"`
	GraphDigest     string `json:"graph_digest"`
	ModuleVersion   string `json:"module_version"`
	BinaryDigest    string `json:"binary_digest"`
}

type Worker struct {
	Path     string
	Manifest WorkerManifest
}

func manifestFor(r buildspec.Resolution, s Source, f buildspec.Fingerprints) (WorkerManifest, error) {
	for _, digest := range []string{s.SourceDigest, s.GraphDigest, f.ToolchainDigest, f.StdlibDigest} {
		if !buildspec.ValidDigest(digest) {
			return WorkerManifest{}, fmt.Errorf("invalid worker input digest")
		}
	}
	if err := buildspec.ValidateVersion(r.Toolchain.Version); err != nil {
		return WorkerManifest{}, err
	}
	m := WorkerManifest{Schema: 1, Protocol: Protocol, Toolchain: r.Toolchain.Version, ToolchainDigest: f.ToolchainDigest, StdlibDigest: f.StdlibDigest, HostOS: r.Toolchain.HostOS, HostArch: r.Toolchain.HostArch, SourceDigest: s.SourceDigest, GraphDigest: s.GraphDigest, ModuleVersion: s.ModuleVersion}
	data, err := json.Marshal(struct {
		Manifest WorkerManifest
		Flags    []string
	}{m, []string{"-ldflags=-checklinkname=0", "-tags=linkname", "-trimpath"}})
	if err != nil {
		return m, err
	}
	m.Key = buildspec.Hash(data)
	return m, nil
}

func readWorker(dir string, want WorkerManifest) (Worker, error) {
	return readWorkerFile(filepath.Join(dir, "worker"), want)
}

func readWorkerFile(path string, want WorkerManifest) (Worker, error) {
	dir := filepath.Dir(path)
	for _, name := range []string{dir, filepath.Join(dir, "manifest.json"), path} {
		info, err := os.Lstat(name)
		if err != nil {
			return Worker{}, err
		}
		if (name == dir && !info.IsDir()) || (name != dir && !info.Mode().IsRegular()) {
			return Worker{}, fmt.Errorf("worker cache entry is not regular: %s", name)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Worker{}, err
	}
	var got WorkerManifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		return Worker{}, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Worker{}, fmt.Errorf("trailing worker manifest data")
	}
	if !buildspec.ValidDigest(got.BinaryDigest) {
		return Worker{}, fmt.Errorf("invalid worker binary digest")
	}
	want.BinaryDigest = got.BinaryDigest
	if !reflect.DeepEqual(want, got) {
		return Worker{}, fmt.Errorf("worker manifest differs from resolved inputs")
	}
	digest, err := buildcfg.HashFile(path)
	if err != nil {
		return Worker{}, err
	}
	if digest != got.BinaryDigest {
		return Worker{}, fmt.Errorf("worker binary content mismatch")
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return Worker{}, fmt.Errorf("read worker build information: %w", err)
	}
	if info.GoVersion != got.Toolchain || info.Path != WorkerPackage || info.Main.Path != buildcfg.GosimModule {
		return Worker{}, fmt.Errorf("worker binary compiler/module mismatch")
	}
	if err := validateSettings(info.Settings, got.HostOS, got.HostArch, true); err != nil {
		return Worker{}, err
	}
	return Worker{Path: path, Manifest: got}, nil
}

// OpenOverride validates an explicitly supplied worker against the same source
// and compiler inputs as automatic startup. A bare binary cannot prove which
// source it was built from: keep its generated manifest alongside it.
func OpenOverride(ctx context.Context, path string, r buildspec.Resolution, source Source, inputs buildspec.Fingerprints) (Worker, error) {
	path, err := exec.LookPath(path)
	if err != nil {
		return Worker{}, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Worker{}, err
	}
	want, err := manifestFor(r, source, inputs)
	if err != nil {
		return Worker{}, err
	}
	w, err := readWorkerFile(path, want)
	if err != nil {
		return Worker{}, fmt.Errorf("invalid GOSIMTOOL worker or missing matching manifest; unset GOSIMTOOL for automatic startup: %w", err)
	}
	if err := Verify(ctx, w, r); err != nil {
		return Worker{}, err
	}
	return w, nil
}

func Ensure(ctx context.Context, r buildspec.Resolution, source Source, inputs buildspec.Fingerprints, cacheRoot string) (worker Worker, result error) {
	if err := ctx.Err(); err != nil {
		return Worker{}, err
	}
	if err := buildspec.CheckEnvironment(r.Toolchain); err != nil {
		return Worker{}, err
	}
	want, err := manifestFor(r, source, inputs)
	if err != nil {
		return Worker{}, err
	}
	if cacheRoot == "" {
		cacheRoot, err = buildspec.CacheRoot()
		if err != nil {
			return Worker{}, err
		}
	}
	dir := filepath.Join(cacheRoot, "workers", "v1", want.Key)
	reuse := func() (Worker, error) {
		w, err := readWorker(dir, want)
		if err == nil {
			err = Verify(ctx, w, r)
		}
		return w, err
	}
	if w, err := reuse(); err == nil {
		return w, nil
	}
	unlock, err := buildspec.LockCache(ctx, dir+".lock")
	if err != nil {
		return Worker{}, err
	}
	defer func() { result = errors.Join(result, unlock()) }()
	if w, err := reuse(); err == nil {
		return w, nil
	}
	stage, err := os.MkdirTemp(filepath.Dir(dir), ".worker-*")
	if err != nil {
		return Worker{}, err
	}
	defer os.RemoveAll(stage)
	args := append([]string{"build"}, buildspec.ApplicationFlags(r)...)
	args = append(args, "-ldflags=-checklinkname=0", "-tags=linkname", "-trimpath", "-o", filepath.Join(stage, "worker"), WorkerPackage)
	cmd := buildspec.ApplicationCommand(ctx, r, args...)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err = buildspec.RunProcess(ctx, cmd)
	if err != nil {
		return Worker{}, fmt.Errorf("build Gosim worker with %s (set up dependencies with Go tooling): %w\n%s", r.Toolchain.Version, err, output.Bytes())
	}
	want.BinaryDigest, err = buildcfg.HashFile(filepath.Join(stage, "worker"))
	if err != nil {
		return Worker{}, err
	}
	data, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		return Worker{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), append(data, '\n'), 0o600); err != nil {
		return Worker{}, err
	}
	staged, err := readWorker(stage, want)
	if err != nil {
		return Worker{}, err
	}
	if err := Verify(ctx, staged, r); err != nil {
		return Worker{}, err
	}
	// A compiler/build can take time; never publish against inputs that changed.
	current, err := ResolveSource(ctx, r)
	if err != nil {
		return Worker{}, err
	}
	if current != source {
		return Worker{}, fmt.Errorf("Gosim source or worker dependency graph changed during build; retry")
	}
	currentInputs, err := buildcfg.Fingerprint(r.Toolchain)
	if err != nil {
		return Worker{}, err
	}
	if currentInputs != inputs {
		return Worker{}, fmt.Errorf("Go distribution changed during worker build; retry")
	}
	if err := ctx.Err(); err != nil {
		return Worker{}, err
	}
	// Replacing a corrupt entry never truncates an executable already running.
	// The per-key lock excludes another publisher; readers validate before use.
	if _, err := os.Lstat(dir); err == nil {
		old := stage + "-invalid"
		if err := os.Rename(dir, old); err != nil {
			return Worker{}, err
		}
		defer os.RemoveAll(old)
	} else if !os.IsNotExist(err) {
		return Worker{}, err
	}
	if err := os.Rename(stage, dir); err != nil {
		return Worker{}, err
	}
	return readWorker(dir, want)
}
