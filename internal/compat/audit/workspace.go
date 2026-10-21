package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

type inspectionWorkspace struct {
	Root, Directory, InspectorDigest string
	ModuleInputs                     []buildspec.Revision
	caller                           buildspec.Resolution
	used                             map[string]string
	privateFiles                     string
	cleanup                          func() error
}

func prepareAuditWorkspace(ctx context.Context, tc buildspec.Toolchain, directory string, parent spec.ResolvedFamily, contexts []spec.SourceContext) (w *inspectionWorkspace, resultErr error) {
	r, modules, root, err := auditCaller(ctx, tc, directory)
	if err != nil {
		return nil, err
	}
	before, err := moduleInputs(ctx, r, modules, nil)
	if err != nil {
		return nil, err
	}
	inspector, err := inspectorDigest(ctx, root)
	if err != nil {
		return nil, err
	}
	dir, cleanup, err := spec.PrepareSourceInspectionContext(ctx, r)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, cleanup())
		}
	}()
	w = &inspectionWorkspace{Root: root, Directory: dir, InspectorDigest: inspector, caller: r, used: map[string]string{}, cleanup: cleanup}
	// Resolve source-only dependencies in the private copy before freezing its
	// generated go directive and sums. Never persist those compiler-specific bytes.
	if err := w.preparePackages(ctx, tc, spec.SourceContext{OS: tc.HostOS, Arch: tc.HostArch, Native: true}, []string{frameworkModule + "/cmd/gosim-gen", frameworkModule + "/internal/compat/audit"}); err != nil {
		return nil, err
	}
	for _, selection := range contexts {
		_, paths, _ := spec.ObservationInventory(parent, selection, true)
		if err := w.preparePackages(ctx, tc, selection, append(paths, "std")); err != nil {
			return nil, err
		}
	}
	selected, err := buildspec.ResolveInspectionModuleGraph(ctx, w.privateResolution())
	if err != nil {
		return nil, err
	}
	if err := buildspec.MatchModuleGraph(modules, selected, "gosim-hook-source"); err != nil {
		return nil, err
	}
	after, err := moduleInputs(ctx, r, modules, nil)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(before, after) {
		return nil, fmt.Errorf("caller module inputs changed during preparation")
	}
	w.ModuleInputs, err = moduleInputs(ctx, r, modules, w.used)
	if err != nil {
		return nil, err
	}
	w.privateFiles, err = privateModuleDigest(dir)
	if err != nil {
		return nil, err
	}
	if err := w.Recheck(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func commandEnv(cmd *exec.Cmd, key, value string) {
	for i, entry := range cmd.Env {
		if strings.HasPrefix(entry, key+"=") {
			cmd.Env[i] = key + "=" + value
			return
		}
	}
	cmd.Env = append(cmd.Env, key+"="+value)
}

func (w *inspectionWorkspace) preparePackages(ctx context.Context, tc buildspec.Toolchain, selection spec.SourceContext, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	tags := buildspec.TranslatedSourceTags(selection.Race)
	cgo := "0"
	if selection.Native {
		tags = "linkname"
		if selection.Race {
			tags += ",race"
			cgo = "1"
		}
	}
	args := append([]string{"list", "-mod=mod", "-deps", "-json", "-e", "-tags=" + tags}, paths...)
	cmd := buildspec.SourceInspectionCommand(ctx, tc, w.Directory, args...)
	commandEnv(cmd, "GOOS", selection.OS)
	commandEnv(cmd, "GOARCH", selection.Arch)
	commandEnv(cmd, "CGO_ENABLED", cgo)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("prepare inspection sources: %w\n%s", err, diagnostics.Bytes())
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var pkg struct{ Module *buildspec.Module }
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if pkg.Module != nil {
			m := *pkg.Module
			dir := m.Effective().Dir
			if old, ok := w.used[m.Path]; ok && old != dir {
				return fmt.Errorf("inconsistent source module directory %s", m.Path)
			}
			w.used[m.Path] = dir
		}
	}
	return nil
}

func (w *inspectionWorkspace) privateResolution() buildspec.Resolution {
	r := w.caller
	r.Context = buildspec.ModuleContext{Directory: w.Directory, GoMod: filepath.Join(w.Directory, "go.mod"), GoWork: "off"}
	return r
}

func privateModuleDigest(dir string) (string, error) {
	var inputs []buildspec.Revision
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := readRegular(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		inputs = append(inputs, buildspec.Revision{Name: name, Digest: buildspec.Hash(data)})
	}
	return digestRevisions("private-inspection-module-v1", inputs), nil
}

func (w *inspectionWorkspace) Recheck(ctx context.Context) error {
	inspector, err := inspectorDigest(ctx, w.Root)
	if err != nil {
		return err
	}
	if inspector != w.InspectorDigest {
		return fmt.Errorf("inspector implementation changed during observation")
	}
	modules, err := buildspec.ResolveInspectionModuleGraph(ctx, w.caller)
	if err != nil {
		return err
	}
	inputs, err := moduleInputs(ctx, w.caller, modules, w.used)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(inputs, w.ModuleInputs) {
		return fmt.Errorf("inspection module inputs changed during observation")
	}
	private, err := privateModuleDigest(w.Directory)
	if err != nil {
		return err
	}
	if private != w.privateFiles {
		return fmt.Errorf("private inspection module changed during observation")
	}
	selected, err := buildspec.ResolveInspectionModuleGraph(ctx, w.privateResolution())
	if err != nil {
		return err
	}
	return buildspec.MatchModuleGraph(modules, selected, "gosim-hook-source")
}

func (w *inspectionWorkspace) Close() error { return w.cleanup() }

func auditCaller(ctx context.Context, tc buildspec.Toolchain, directory string) (buildspec.Resolution, []buildspec.Module, string, error) {
	var r buildspec.Resolution
	directory, err := filepath.Abs(directory)
	if err != nil {
		return r, nil, "", err
	}
	r = buildspec.Resolution{Toolchain: tc, Context: buildspec.ModuleContext{Directory: directory}}
	data, err := buildspec.SourceInspectionApplicationCommand(ctx, r, "env", "-json", "GOMOD", "GOWORK").Output()
	if err != nil {
		return r, nil, "", err
	}
	var info struct{ GOMOD, GOWORK string }
	if err := json.Unmarshal(data, &info); err != nil {
		return r, nil, "", err
	}
	r.Context.GoMod, r.Context.GoWork = info.GOMOD, info.GOWORK
	if r.Context.GoWork == "" {
		r.Context.GoWork = "off"
	}
	modules, err := buildspec.ResolveInspectionModuleGraph(ctx, r)
	if err != nil {
		return r, nil, "", err
	}
	root := ""
	for _, m := range modules {
		if m.Path == frameworkModule {
			root = m.Effective().Dir
		}
	}
	if root == "" {
		return r, nil, "", fmt.Errorf("selected graph has no Gosim source module")
	}
	return r, modules, root, nil
}
