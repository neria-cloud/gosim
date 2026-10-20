package workerboot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
)

type Invocation struct {
	Schema       int                    `json:"schema"`
	Resolution   buildspec.Resolution   `json:"resolution"`
	Manifest     WorkerManifest         `json:"manifest"`
	Fingerprints buildspec.Fingerprints `json:"-"`
}

const maxInvocationSizeMiB = 1

func ValidateInvocation(ctx context.Context, path string) (Invocation, error) {
	in, err := readInvocationFile(path)
	if err != nil {
		return in, err
	}
	if err := buildcfg.ValidateWorker(in.Resolution.Toolchain); err != nil {
		return in, err
	}
	observed, err := buildcfg.ResolveRunningProcess(ctx, "")
	if err != nil {
		return in, err
	}
	// The launcher passes the resolved cache through the child environment,
	// not the invocation protocol. Source/graph digests bind its contents.
	in.Resolution.Toolchain.ModuleCache = observed.Toolchain.ModuleCache
	expected := in.Resolution
	// A graph digest may be attached by application resolution, not by go env.
	expected.Context.GraphDigest = ""
	if !reflect.DeepEqual(expected, observed) {
		return in, fmt.Errorf("worker invocation differs from current compiler/module/workspace context")
	}
	source, err := ResolveSource(ctx, observed)
	if err != nil {
		return in, err
	}
	in.Fingerprints, err = buildcfg.Fingerprint(observed.Toolchain)
	if err != nil {
		return in, err
	}
	want, err := manifestFor(observed, source, in.Fingerprints)
	if err != nil {
		return in, err
	}
	executable, err := os.Executable()
	if err != nil {
		return in, err
	}
	want.BinaryDigest, err = buildcfg.HashFile(executable)
	if err != nil {
		return in, err
	}
	if !reflect.DeepEqual(want, in.Manifest) {
		return in, fmt.Errorf("worker/source inputs changed or invocation manifest is mismatched; rerun gosim")
	}
	return in, nil
}

func readInvocationFile(path string) (Invocation, error) {
	var in Invocation
	stat, err := os.Lstat(path)
	if err != nil {
		return in, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > maxInvocationSizeMiB*bytesPerKiB*bytesPerKiB {
		return in, fmt.Errorf("invalid worker invocation file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return in, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return in, fmt.Errorf("trailing worker invocation data")
	}
	if in.Schema != 1 || in.Resolution.Schema != 1 {
		return in, fmt.Errorf("unsupported worker invocation schema")
	}
	return in, nil
}
