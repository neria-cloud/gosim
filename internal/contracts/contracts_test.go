package contracts

import (
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

func TestFeatureSets(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{{"rt"}, {"fs", "rt"}, {"net", "rt"}, {"net", "rt", "fs"}} {
		features, err := ParseFeatures(names)
		if err != nil || !features.Has(RT) {
			t.Fatalf("features %v: %v", names, err)
		}
		featureNames := features.Names()
		if featureNames[0] != "rt" {
			t.Fatal("features are not canonical")
		}
		featureNames[0] = "mutated"
		if features.Names()[0] != "rt" {
			t.Fatal("feature names retain caller memory")
		}
		if features.Has(0) || features.Has(8) || features.Has(RT|FS) {
			t.Fatal("invalid feature accepted")
		}
	}
	for _, names := range [][]string{nil, {}, {"fs"}, {"net"}, {"fs", "net"}, {"rt", "rt"}, {"rt", "unknown"}} {
		if _, err := ParseFeatures(names); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("accepted %v: %v", names, err)
		}
	}
	if err := FeatureSet(9).Validate(); err == nil {
		t.Fatal("unknown feature bit accepted")
	}
}

func TestProviderOrderingAndOwnership(t *testing.T) {
	t.Parallel()
	fsProvider, netProvider := &fakeProvider{feature: FS}, &fakeProvider{feature: Net}
	input := []Provider{netProvider, fsProvider}
	ordered, err := OrderProviders(FeatureSet(RT|FS|Net), input)
	if err != nil || ordered[0] != fsProvider || ordered[1] != netProvider {
		t.Fatalf("bad order %v, %v", ordered, err)
	}
	ordered[0] = nil
	if input[0] == nil {
		t.Fatal("ordering aliases caller slice")
	}
	for _, input := range [][]Provider{nil, {fsProvider}, {fsProvider, fsProvider}, {nil}, {&fakeProvider{feature: RT}}, {&fakeProvider{feature: 8}}} {
		if _, err := OrderProviders(FeatureSet(RT|FS|Net), input); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("accepted providers %v", input)
		}
	}
	if _, err := OrderProviders(FeatureSet(RT), []Provider{fsProvider}); err == nil {
		t.Fatal("disabled provider accepted")
	}
	if _, err := OrderProviders(FeatureSet(RT), nil); err != nil {
		t.Fatal(err)
	}
	files, sockets := Bindings{Files: &fakeFiles{}}, Bindings{Sockets: &fakeSockets{}}
	if err := ValidateBindings(FS, files); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBindings(Net, sockets); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBindings(FS, sockets); err == nil {
		t.Fatal("wrong service owner accepted")
	}
	if err := ValidateBindings(Net, Bindings{}); err == nil {
		t.Fatal("empty service accepted")
	}
	combined, err := MergeBindings([]Bindings{files, sockets})
	if err != nil || combined.Files == nil || combined.Sockets == nil {
		t.Fatal("lost service bindings")
	}
	if err := ValidateBindings(FS, combined); err == nil {
		t.Fatal("cross-domain ownership accepted")
	}
	if partial, err := MergeBindings([]Bindings{files, sockets, files}); err == nil || partial.Files != nil || partial.Sockets != nil {
		t.Fatal("duplicate binding returned partial success")
	}
	for _, owner := range []Feature{FS, Net} {
		session := &fakeSession{provider: &fakeProvider{feature: owner}}
		if err := ValidateControls(owner, session.Controls()); err != nil {
			t.Fatal(err)
		}
		other := FS
		if owner == FS {
			other = Net
		}
		if err := ValidateControls(other, session.Controls()); err == nil {
			t.Fatal("wrong control owner accepted")
		}
	}
}

func TestRecordingLifecycleAndPortContracts(t *testing.T) {
	t.Parallel()
	var calls []string
	ports := &fakePorts{}
	provider := &fakeProvider{feature: FS, log: &calls}
	session, err := provider.Open(ports.ports())
	if err != nil {
		t.Fatal(err)
	}
	staged, err := session.Prepare(StartSpec{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	staged.Abort()
	staged, err = session.Prepare(StartSpec{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	instance := staged.Publish()
	instance.Quiesce()
	if err := instance.Settle(StopCrash); err != nil {
		t.Fatal(err)
	}
	instance.Detach()
	session.Destroy(MachineRef{})
	session.Close()
	want := []string{"open", "prepare", "abort", "prepare", "publish", "quiesce", "settle", "detach", "destroy", "close"}
	if !slices.Equal(calls, want) {
		t.Fatalf("wrong call sequence %v", calls)
	}
	provider.failOpen = true
	if _, err := provider.Open(ports.ports()); !errors.Is(err, errInjected) {
		t.Fatal("open injection lost")
	}
	provider.failPrepare = true
	if _, err := session.Prepare(StartSpec{}, nil); !errors.Is(err, errInjected) {
		t.Fatal("prepare injection lost")
	}
	fields := []Field{{Key: "kind", Value: "original"}}
	ports.Emit(EpochRef{}, "event", fields)
	fields[0].Value = "mutated"
	if ports.fields[0].Value != "original" {
		t.Fatal("trace retained borrowed fields")
	}
	epoch := EpochRef{Machine: MachineRef{RunID: 1, ID: 1}, Epoch: 1}
	if _, err := ports.Schedule(epoch, 10, func() {}); err != nil {
		t.Fatal(err)
	}
	ports.CancelEpoch(epoch)
	if len(ports.events) != 0 {
		t.Fatal("fake event cancellation violated contract")
	}
	if err := ValidatePorts(Ports{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal("missing ports accepted")
	}
}

func TestControlValueValidationAndCopying(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"", "relative", "/nul\x00path"} {
		if err := ValidateFSConfig(FSConfig{WorkingDir: dir}); err == nil {
			t.Fatalf("accepted working dir %q", dir)
		}
	}
	if err := ValidateFSConfig(FSConfig{WorkingDir: "/app"}); err != nil {
		t.Fatal(err)
	}
	for _, probability := range []float64{-1, 2, math.Inf(1), math.NaN()} {
		if err := ValidateFSFault(FSFaultConfig{Probability: probability}); err == nil {
			t.Fatal("invalid fs probability")
		}
		if err := ValidateNetConfig(NetConfig{Lambda: 1, Fail: probability}); err == nil {
			t.Fatal("invalid net probability")
		}
		if err := ValidateNetLinkPolicy(NetLinkPolicy{SetLoss: true, Repair: probability}); err == nil {
			t.Fatal("invalid selected loss override")
		}
	}
	for _, config := range []NetConfig{{}, {MinNS: -1, Lambda: 1}, {MinNS: 2, MaxNS: 1, Lambda: 1}, {Lambda: math.NaN()}, {Lambda: math.Inf(1)}, {Lambda: 1, TCPBufferSize: -1}, {Lambda: 1, TCPBufferSize: 16*1024*1024 + 1}, {Lambda: 1, ConnectionTimeoutNS: -1}} {
		if err := ValidateNetConfig(config); err == nil {
			t.Fatalf("accepted invalid network config %+v", config)
		}
	}
	if err := ValidateNetConfig(NetConfig{MaxNS: 10, Lambda: 5, Repair: 1}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNetLink(NetLink{Direction: 2}); err == nil {
		t.Fatal("invalid direction")
	}
	if err := ValidateNetLinkPolicy(NetLinkPolicy{Lambda: math.NaN()}); err != nil {
		t.Fatal("unselected override fields inspected")
	}
	policy := NetLinkPolicy{SetLatency: true, MaxNS: 10, Lambda: 5}
	policyCopy := policy
	policyCopy.MaxNS = 20
	if policy.MaxNS != 10 {
		t.Fatal("control policy borrowed caller state")
	}
}

func TestProductionDependencyBoundary(t *testing.T) {
	t.Parallel()
	cmd := buildspec.GoCommand(buildspec.Toolchain{Executable: filepath.Join(runtime.GOROOT(), "bin", "go"), GOROOT: runtime.GOROOT(), Version: runtime.Version(), HostOS: runtime.GOOS, HostArch: runtime.GOARCH}, "", "list", "-deps", "github.com/neria-cloud/gosim/internal/contracts")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range strings.Fields(string(output)) {
		if strings.HasPrefix(pkg, "github.com/neria-cloud/gosim/") && pkg != "github.com/neria-cloud/gosim/internal/contracts" && pkg != "github.com/neria-cloud/gosim/rt/runtime/abi" {
			t.Fatalf("contracts depend on implementation %s", pkg)
		}
	}
}
