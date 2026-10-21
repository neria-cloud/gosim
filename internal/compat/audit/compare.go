package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildcfg"
	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

type (
	InspectedContext = spec.InspectedContext
	CompilerRecord   = spec.CompilerRecord
	SourceLocation   = spec.SourceLocation
	ContextResult    = spec.ContextResult
)

type ObserveRequest struct {
	Directory, Profile, Output string
	Arches                     []string
}
type ParentObservation struct {
	Schema           int                  `json:"schema"`
	Family           string               `json:"family"`
	Compiler         CompilerRecord       `json:"compiler"`
	DefinitionDigest string               `json:"definition_digest"`
	OriginDigest     string               `json:"origin_digest"`
	InspectorDigest  string               `json:"inspector_digest"`
	ModuleInputs     []buildspec.Revision `json:"module_inputs"`
	Complete         bool                 `json:"complete"`
	Observations     spec.ObservationSet  `json:"observations"`
}

func Observe(ctx context.Context, req ObserveRequest) (out ParentObservation, resultErr error) {
	compiler, err := buildcfg.DiscoverInspection(ctx, req.Directory, "")
	if err != nil {
		return out, err
	}
	tc := compiler.Toolchain()
	_, _, root, err := auditCaller(ctx, tc, req.Directory)
	if err != nil {
		return out, err
	}
	parent, origin, err := loadParent(root, req.Profile)
	if err != nil {
		return out, err
	}
	if tc.Version != origin.BaselineToolchain {
		return out, fmt.Errorf("parent observation requires recorded compiler %s, running %s", origin.BaselineToolchain, tc.Version)
	}
	contexts, err := observationContexts(tc, parent, req.Arches)
	if err != nil {
		return out, err
	}
	w, err := prepareAuditWorkspace(ctx, tc, req.Directory, parent, contexts)
	if err != nil {
		return out, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, w.Close())
		if resultErr != nil {
			out.Complete = false
		}
	}()
	out = ParentObservation{Schema: 1, Family: parent.Family, Compiler: CompilerRecord{Version: tc.Version, HostOS: tc.HostOS, HostArch: tc.HostArch, Fingerprints: compiler.Fingerprints()}, DefinitionDigest: parent.DefinitionDigest, OriginDigest: originDigest(origin), InspectorDigest: w.InspectorDigest, ModuleInputs: slices.Clone(w.ModuleInputs)}
	out.Observations, err = spec.CollectSourceObservations(ctx, tc, w.Directory, spec.ObservationRequest{Family: parent, Origin: origin, Contexts: contexts, IncludeAPIs: true, IncludeStdlibInventory: true})
	resultErr = errors.Join(err, compiler.Recheck(ctx), w.Recheck(ctx), recheckParent(root, parent, origin))
	if resultErr != nil {
		return out, resultErr
	}
	out.Complete = true
	resultErr = validateParentObservation(out, parent, origin, w.InspectorDigest, w.ModuleInputs, contexts)
	return out, resultErr
}

func loadParent(root, profile string) (spec.ResolvedFamily, spec.Origin, error) {
	dir := os.DirFS(filepath.Join(root, "internal/compat/profiles"))
	origin, err := spec.LoadOrigin(dir, profile)
	if err != nil {
		return spec.ResolvedFamily{}, origin, err
	}
	defs, err := spec.LoadDefinitions(dir)
	if err != nil {
		return spec.ResolvedFamily{}, origin, err
	}
	parent, err := spec.ResolveDefinitions(defs, origin.Family)
	return parent, origin, err
}

func originDigest(origin spec.Origin) string {
	data, err := json.Marshal(origin)
	if err != nil {
		panic(err)
	}
	return buildspec.Hash(data)
}

func recheckParent(root string, parent spec.ResolvedFamily, origin spec.Origin) error {
	after, afterOrigin, err := loadParent(root, parent.ProfileID)
	if err != nil {
		return err
	}
	if parent.DefinitionDigest != after.DefinitionDigest || originDigest(origin) != originDigest(afterOrigin) {
		return fmt.Errorf("parent definition or origin changed during observation")
	}
	return nil
}

func observationContexts(tc buildspec.Toolchain, parent spec.ResolvedFamily, arches []string) ([]spec.SourceContext, error) {
	if len(arches) == 0 {
		arches = []string{"amd64", "arm64"}
	} else {
		arches = slices.Clone(arches)
	}
	slices.Sort(arches)
	var contexts []spec.SourceContext
	for i, arch := range arches {
		if arch != "amd64" && arch != "arm64" || i > 0 && arch == arches[i-1] {
			return nil, fmt.Errorf("invalid/duplicate observation architecture %q", arch)
		}
		for _, race := range []bool{false, true} {
			contexts = append(contexts, spec.SourceContext{OS: "linux", Arch: arch, Race: race})
		}
	}
	native := false
	for _, d := range parent.Declarations {
		native = native || d.Hook.Context == "host" || d.Hook.Context == "host-race"
	}
	if native {
		for _, race := range []bool{false, true} {
			contexts = append(contexts, spec.SourceContext{OS: tc.HostOS, Arch: tc.HostArch, Race: race, Native: true})
		}
	}
	slices.SortFunc(contexts, func(a, b spec.SourceContext) int {
		return strings.Compare(contextKey(InspectedContext(a)), contextKey(InspectedContext(b)))
	})
	return contexts, nil
}

func contextKey(value InspectedContext) string { data, _ := json.Marshal(value); return string(data) }
func validateParentObservation(snapshot ParentObservation, parent spec.ResolvedFamily, origin spec.Origin, inspector string, modules []buildspec.Revision, contexts []spec.SourceContext) error {
	if snapshot.Schema != 1 || !snapshot.Complete || snapshot.Family != parent.Family {
		return fmt.Errorf("invalid/incomplete parent snapshot")
	}
	if snapshot.Compiler.Version != origin.BaselineToolchain {
		return fmt.Errorf("parent snapshot baseline mismatch")
	}
	if _, err := buildspec.ParseGoVersion(snapshot.Compiler.Version); err != nil {
		return err
	}
	if err := buildspec.ValidatePlatform(snapshot.Compiler.HostOS, snapshot.Compiler.HostArch, "linux", snapshot.Compiler.HostArch); err != nil {
		return err
	}
	for _, digest := range []string{snapshot.Compiler.Fingerprints.ToolchainDigest, snapshot.Compiler.Fingerprints.StdlibDigest, snapshot.DefinitionDigest, snapshot.OriginDigest, snapshot.InspectorDigest} {
		if !buildspec.ValidDigest(digest) {
			return fmt.Errorf("invalid parent snapshot digest")
		}
	}
	if snapshot.DefinitionDigest != parent.DefinitionDigest || snapshot.OriginDigest != originDigest(origin) || snapshot.InspectorDigest != inspector || !reflect.DeepEqual(snapshot.ModuleInputs, modules) {
		return fmt.Errorf("stale parent snapshot: definition, origin, inspector or module inputs differ")
	}
	if !reflect.DeepEqual(snapshot.Observations.APIPackages, spec.ObservationAPIPackages(parent)) {
		return fmt.Errorf("parent snapshot API inventory mismatch")
	}
	if len(snapshot.Observations.Contexts) != len(contexts) {
		return fmt.Errorf("parent snapshot context inventory mismatch")
	}
	wanted := map[InspectedContext]bool{}
	for _, c := range contexts {
		wanted[InspectedContext(c)] = true
	}
	seen := map[InspectedContext]bool{}
	for _, c := range snapshot.Observations.Contexts {
		if !wanted[c.Context] || seen[c.Context] || !c.Complete || len(c.Errors) != 0 {
			return fmt.Errorf("incomplete/unexpected parent context")
		}
		if c.Context.Native && (c.Context.OS != snapshot.Compiler.HostOS || c.Context.Arch != snapshot.Compiler.HostArch) {
			return fmt.Errorf("parent native context differs from recorded compiler host")
		}
		seen[c.Context] = true
	}
	keys := map[string]bool{}
	for _, r := range snapshot.Observations.Records {
		if !wanted[r.Context] || !slices.Contains([]string{"declaration", "api", "layout", "constant", "package", "source"}, r.Kind) || len(r.Key) == 0 {
			return fmt.Errorf("invalid parent observation record")
		}
		wantKey := map[string]int{"declaration": 4, "api": 2, "layout": 3, "constant": 3, "package": 1, "source": 1}[r.Kind]
		if len(r.Key) != wantKey {
			return fmt.Errorf("invalid parent observation key")
		}
		key := r.Kind + contextKey(r.Context) + recordKey(r.Key)
		if keys[key] {
			return fmt.Errorf("duplicate parent observation record")
		}
		keys[key] = true
		if r.Present && (!json.Valid(r.Value) || string(r.Value) == "null") || !r.Present && len(r.Value) != 0 {
			return fmt.Errorf("invalid parent observation presence/value")
		}
	}
	if len(keys) == 0 {
		return fmt.Errorf("empty parent observations")
	}
	for _, c := range contexts {
		context := InspectedContext(c)
		for _, expected := range expectedContracts(parent, context) {
			if !keys[expected.Kind+contextKey(context)+recordKey(expected.Key)] {
				return fmt.Errorf("parent contract inventory incomplete")
			}
		}
		_, packages, _ := spec.ObservationInventory(parent, c, true)
		for _, pkg := range packages {
			if !keys["package"+contextKey(context)+recordKey([]string{pkg})] {
				return fmt.Errorf("parent package inventory incomplete: %s", pkg)
			}
		}
		for _, file := range origin.Files {
			if !keys["source"+contextKey(context)+recordKey([]string{file.Path})] {
				return fmt.Errorf("parent source inventory incomplete")
			}
		}
	}
	return nil
}

func recordKey(key []string) string { data, _ := json.Marshal(key); return string(data) }

type CompareRequest struct {
	Directory, ParentProfile, Output, ParentObservation string
	Arches                                              []string
}

type Finding struct {
	Kind      string           `json:"kind"`
	Key       []string         `json:"key"`
	Context   InspectedContext `json:"context"`
	Before    json.RawMessage  `json:"before,omitempty"`
	After     json.RawMessage  `json:"after,omitempty"`
	Locations []SourceLocation `json:"locations,omitempty"`
	Severity  string           `json:"severity"`
	Message   string           `json:"message"`
}

type ComparisonReport struct {
	Schema                 int                  `json:"schema"`
	Candidate              CompilerRecord       `json:"candidate"`
	ParentFamily           string               `json:"parent_family"`
	ParentDefinitionDigest string               `json:"parent_definition_digest"`
	ParentOriginDigest     string               `json:"parent_origin_digest"`
	ParentObservationUsed  bool                 `json:"parent_observation_used"`
	APIDiff                string               `json:"api_diff"` // inspected, skipped-no-parent, incomplete
	InspectorDigest        string               `json:"inspector_digest"`
	ModuleInputs           []buildspec.Revision `json:"module_inputs"`
	Complete               bool                 `json:"complete"`
	Contexts               []ContextResult      `json:"contexts"`
	Findings               []Finding            `json:"findings"`
}

type DeltaProposal struct {
	Section             string          `json:"section"`
	Operation           string          `json:"operation"`
	Key                 []string        `json:"key"`
	Value               json.RawMessage `json:"value,omitempty"`
	NeedsImplementation bool            `json:"needs_implementation"`
	Reason              string          `json:"reason"`
}

type DraftDelta struct {
	Schema     int             `json:"schema"`
	Kind       string          `json:"kind"` // gosim-profile-draft
	Family     string          `json:"family"`
	Parent     string          `json:"parent"`
	Complete   bool            `json:"complete"`
	Proposals  []DeltaProposal `json:"proposals"`
	Unresolved []Finding       `json:"unresolved"`
}

func Compare(ctx context.Context, req CompareRequest) (report ComparisonReport, draft DraftDelta, resultErr error) {
	compiler, err := buildcfg.DiscoverInspection(ctx, req.Directory, "")
	if err != nil {
		return report, draft, err
	}
	tc := compiler.Toolchain()
	_, _, root, err := auditCaller(ctx, tc, req.Directory)
	if err != nil {
		return report, draft, err
	}
	parent, origin, err := loadParent(root, req.ParentProfile)
	if err != nil {
		return report, draft, err
	}
	candidate, _ := buildspec.ParseGoVersion(tc.Version)
	previous, err := buildspec.ParseGoVersion("go" + parent.Family + ".0")
	if err != nil || candidate.Major != previous.Major || candidate.Minor != previous.Minor+1 {
		return report, draft, fmt.Errorf("candidate %s must be the immediate next minor after %s", tc.Version, parent.Family)
	}
	contexts, err := observationContexts(tc, parent, req.Arches)
	if err != nil {
		return report, draft, err
	}
	w, err := prepareAuditWorkspace(ctx, tc, req.Directory, parent, contexts)
	if err != nil {
		return report, draft, err
	}
	defer func() {
		if err := w.Close(); err != nil {
			resultErr = errors.Join(resultErr, err)
			report.Complete, draft.Complete = false, false
		}
	}()
	var before *ParentObservation
	if req.ParentObservation != "" {
		data, err := os.ReadFile(req.ParentObservation)
		if err != nil {
			return report, draft, err
		}
		before = new(ParentObservation)
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(before); err != nil {
			return report, draft, err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return report, draft, fmt.Errorf("trailing parent snapshot data")
		}
		if err := validateParentObservation(*before, parent, origin, w.InspectorDigest, w.ModuleInputs, contexts); err != nil {
			return report, draft, err
		}
	}
	after, scanErr := spec.CollectSourceObservations(ctx, tc, w.Directory, spec.ObservationRequest{Family: parent, Origin: origin, Contexts: contexts, IncludeAPIs: before != nil, IncludeStdlibInventory: true})
	findings, proposals, diffErr := CompareObservations(parent, origin, before, after)
	resultErr = errors.Join(scanErr, diffErr, compiler.Recheck(ctx), w.Recheck(ctx), recheckParent(root, parent, origin))
	complete := resultErr == nil && len(after.Contexts) == len(contexts)
	for _, c := range after.Contexts {
		complete = complete && c.Complete && len(c.Errors) == 0
	}
	apiDiff := "skipped-no-parent"
	if before != nil {
		apiDiff = "inspected"
		if scanErr != nil || diffErr != nil || !complete {
			apiDiff = "incomplete"
		}
	}
	if !complete && resultErr == nil {
		resultErr = fmt.Errorf("incomplete candidate observations")
	}
	report = ComparisonReport{Schema: 1, Candidate: CompilerRecord{Version: tc.Version, HostOS: tc.HostOS, HostArch: tc.HostArch, Fingerprints: compiler.Fingerprints()}, ParentFamily: parent.Family, ParentDefinitionDigest: parent.DefinitionDigest, ParentOriginDigest: originDigest(origin), ParentObservationUsed: before != nil, APIDiff: apiDiff, InspectorDigest: w.InspectorDigest, ModuleInputs: slices.Clone(w.ModuleInputs), Complete: complete, Contexts: after.Contexts, Findings: findings}
	draft = DraftDelta{Schema: 1, Kind: "gosim-profile-draft", Family: fmt.Sprintf("%d.%d", candidate.Major, candidate.Minor), Parent: parent.Family, Complete: complete, Proposals: proposals, Unresolved: []Finding{}}
	for _, group := range groupFindings(findings) {
		if group.Finding.Severity != "information" {
			f := group.Finding
			if len(group.Contexts) > 1 {
				f.Message += "; contexts: " + string(auditJSON(group.Contexts))
			}
			draft.Unresolved = append(draft.Unresolved, f)
		}
	}
	return report, draft, resultErr
}

// expectedContracts derives expectations from the fully folded parent, never a
// single delta. Proposal payloads preserve the complete typed metadata record.
func expectedContracts(parent spec.ResolvedFamily, c InspectedContext) map[string]spec.Observation {
	result := map[string]spec.Observation{}
	add := func(kind string, key []string, value any) {
		r := spec.Observation{Kind: kind, Key: key, Context: c, Present: true, Value: auditJSON(value)}
		result[kind+recordKey(key)] = r
	}
	declarations, _, _ := spec.ObservationInventory(parent, spec.SourceContext(c), false)
	for _, d := range declarations {
		scope := "*"
		if len(d.Arches) == 1 {
			scope = d.Arches[0]
		}
		add("declaration", []string{d.Hook.Context, d.Hook.Package, d.Hook.Symbol, scope}, d.Hook.Signature)
	}
	if !c.Native {
		arch := parent.Architectures[c.Arch]
		for _, l := range arch.Layouts {
			add("layout", []string{c.Arch, l.Package, l.Type}, l)
		}
		for _, k := range arch.Constants {
			add("constant", []string{c.Arch, k.Package, k.Symbol}, k.Value)
		}
	}
	return result
}

func auditJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func equalJSON(a, b json.RawMessage) bool {
	var x, y any
	da, db := json.NewDecoder(bytes.NewReader(a)), json.NewDecoder(bytes.NewReader(b))
	da.UseNumber()
	db.UseNumber()
	return da.Decode(&x) == nil && db.Decode(&y) == nil && reflect.DeepEqual(x, y)
}

func CompareObservations(parent spec.ResolvedFamily, origin spec.Origin, before *ParentObservation, after spec.ObservationSet) ([]Finding, []DeltaProposal, error) {
	findings, proposals := []Finding{}, []DeltaProposal{}
	var resultErr error
	if before != nil && !reflect.DeepEqual(after.APIPackages, before.Observations.APIPackages) {
		return findings, proposals, fmt.Errorf("candidate API inventory mismatch")
	}
	prior := map[string]spec.Observation{}
	if before != nil {
		for _, r := range before.Observations.Records {
			prior[contextKey(r.Context)+r.Kind+recordKey(r.Key)] = r
		}
	}
	origins := map[string]string{}
	for _, f := range origin.Files {
		origins[f.Path] = f.Digest
	}
	// A declaration may occur in several contexts; never propose conflicting edits.
	candidates, ambiguous, observed := map[string]DeltaProposal{}, map[string]bool{}, map[string]string{}
	emit := func(kind, severity, message string, old, now spec.Observation) {
		findings = append(findings, Finding{Kind: kind, Key: now.Key, Context: now.Context, Before: old.Value, After: now.Value, Locations: now.Locations, Severity: severity, Message: message})
	}
	seenContexts := map[InspectedContext]bool{}
	for _, c := range after.Contexts {
		if seenContexts[c.Context] {
			return findings, proposals, fmt.Errorf("duplicate comparison context")
		}
		seenContexts[c.Context] = true
		if !c.Complete || len(c.Errors) > 0 {
			emit("inspection-error", "incomplete", strings.Join(c.Errors, "; "), spec.Observation{}, spec.Observation{Context: c.Context, Key: []string{}})
			resultErr = errors.Join(resultErr, fmt.Errorf("incomplete context %s", contextKey(c.Context)))
			continue
		}
		records := map[string]spec.Observation{}
		for _, r := range after.Records {
			if r.Context != c.Context {
				continue
			}
			key := r.Kind + recordKey(r.Key)
			if _, exists := records[key]; exists {
				return findings, proposals, fmt.Errorf("duplicate observation %s", key)
			}
			if len(r.Key) != map[string]int{"declaration": 4, "api": 2, "layout": 3, "constant": 3, "package": 1, "source": 1}[r.Kind] || len(r.Key) == 0 || r.Present && (!json.Valid(r.Value) || string(r.Value) == "null") || !r.Present && len(r.Value) != 0 {
				return findings, proposals, fmt.Errorf("invalid observation %s", key)
			}
			records[key] = r
		}
		for key, want := range expectedContracts(parent, c.Context) {
			got, ok := records[key]
			if !ok {
				resultErr = errors.Join(resultErr, fmt.Errorf("missing observation record %s", key))
				emit("inspection-error", "incomplete", "Required contract was not inspected", want, want)
				continue
			}
			identity := got.Kind + recordKey(got.Key)
			state := fmt.Sprintf("%t:%s", got.Present, got.Value)
			if old, ok := observed[identity]; ok && old != state {
				ambiguous[identity] = true
			}
			observed[identity] = state
			if got.Present && equalJSON(want.Value, got.Value) {
				continue
			}
			kind := got.Kind
			if kind == "declaration" {
				kind = "signature"
			}
			if !got.Present {
				kind = "missing"
			}
			severity, message := "incompatible", "Declared contract differs; implementation review required"
			if kind == "signature" && spellingOnly(want.Value, got.Value) {
				severity, message = "information", "Equivalent signature spelling; update metadata only"
			}
			emit(kind, severity, message, want, got)
			if proposal, ok := contractProposal(parent, got); ok {
				candidates[identity] = proposal
			}
		}
		for _, got := range records {
			switch got.Kind {
			case "source":
				old, exists := prior[contextKey(c.Context)+"source"+recordKey(got.Key)]
				if !exists {
					if hash, ok := origins[got.Key[0]]; ok {
						old = spec.Observation{Present: true, Value: auditJSON(hash)}
						exists = true
					}
				}
				if exists && got.Present == old.Present && (!got.Present || equalJSON(got.Value, old.Value)) {
					continue
				}
				if exists || before != nil {
					emit("source", "review", "Source inventory/hash changed; behavior requires review", old, got)
				}
			case "package":
				old, existed := prior[contextKey(c.Context)+"package"+recordKey(got.Key)]
				policy, _ := (spec.Profile{Packages: parent.Packages}).Policy(got.Key[0], true)
				if !got.Present {
					emit("package", "incompatible", "Required package is absent", old, got)
				} else if before != nil && (!existed || !old.Present) {
					emit("package", "review", "Package added to candidate inventory", old, got)
				} else if policy.Path == "" {
					emit("package", "information", "Existing package has no reviewed policy", old, got)
				}
			case "api":
				if before == nil {
					continue
				}
				old, exists := prior[contextKey(c.Context)+"api"+recordKey(got.Key)]
				if !got.Present {
					continue
				}
				if !exists || !old.Present {
					emit("api-added", "review", "Exported API added", old, got)
				} else if !equalJSON(old.Value, got.Value) {
					emit("api-changed", "review", "Exported API changed", old, got)
				}
			}
		}
		if before != nil {
			for _, old := range before.Observations.Records {
				if old.Context != c.Context || !old.Present || old.Kind != "api" && old.Kind != "source" && old.Kind != "package" {
					continue
				}
				got, exists := records[old.Kind+recordKey(old.Key)]
				if exists && got.Present {
					continue
				}
				if (old.Kind == "source" || old.Kind == "package") && exists {
					continue
				}
				kind, message := "api-removed", "Exported API removed"
				if old.Kind == "source" {
					kind, message = "source", "Previously selected source is no longer selected; review behavior"
				}
				if old.Kind == "package" {
					kind, message = "package", "Package removed from candidate inventory"
				}
				emit(kind, "review", message, old, spec.Observation{Kind: old.Kind, Key: old.Key, Context: c.Context})
			}
		}
	}
	if len(after.Contexts) == 0 {
		resultErr = fmt.Errorf("no comparison contexts")
	}
	if resultErr == nil {
		for key, p := range candidates {
			if !ambiguous[key] {
				proposals = append(proposals, p)
			}
		}
	}
	slices.SortFunc(findings, func(a, b Finding) int { return strings.Compare(string(auditJSON(a)), string(auditJSON(b))) })
	slices.SortFunc(proposals, func(a, b DeltaProposal) int { return strings.Compare(string(auditJSON(a)), string(auditJSON(b))) })
	return findings, proposals, resultErr
}

func contractProposal(parent spec.ResolvedFamily, r spec.Observation) (DeltaProposal, bool) {
	p := DeltaProposal{Operation: "replace", Key: slices.Clone(r.Key), NeedsImplementation: true, Reason: "Observed contract change; adapt and review implementation before adoption"}
	switch r.Kind {
	case "declaration":
		p.Section = "declarations"
		for _, d := range parent.Declarations {
			scope := "*"
			if len(d.Arches) == 1 {
				scope = d.Arches[0]
			}
			if recordKey([]string{d.Hook.Context, d.Hook.Package, d.Hook.Symbol, scope}) != recordKey(r.Key) {
				continue
			}
			if !r.Present {
				p.Operation = "remove"
				return p, true
			}
			if spellingOnly(auditJSON(d.Hook.Signature), r.Value) {
				p.NeedsImplementation = false
				p.Reason = "Equivalent signature spelling; update metadata only"
			}
			if json.Unmarshal(r.Value, &d.Hook.Signature) != nil {
				return p, false
			}
			p.Value = auditJSON(d)
			return p, true
		}
	case "layout":
		p.Section = "architectures." + r.Context.Arch + ".layouts"
		p.Key = slices.Clone(r.Key[1:])
		var value spec.LayoutContract
		if r.Present && json.Unmarshal(r.Value, &value) == nil && value.Package == r.Key[1] && value.Type == r.Key[2] {
			p.Value = auditJSON(value)
			return p, true
		}
	case "constant":
		p.Section = "architectures." + r.Context.Arch + ".constants"
		p.Key = slices.Clone(r.Key[1:])
		var value string
		if r.Present && json.Unmarshal(r.Value, &value) == nil && value != "not a constant" {
			p.Value = auditJSON(spec.ConstantContract{Package: r.Key[1], Symbol: r.Key[2], Value: value})
			return p, true
		}
	}
	return p, false
}

// Universe-only types can be compared without inventing package name bindings.
// Unknown named types retain the conservative implementation-review requirement.
func spellingOnly(a, b json.RawMessage) bool {
	var x, y string
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	left, err := types.Eval(token.NewFileSet(), nil, token.NoPos, x)
	if err != nil {
		return false
	}
	right, err := types.Eval(token.NewFileSet(), nil, token.NoPos, y)
	return err == nil && left.IsType() && right.IsType() && types.Identical(left.Type, right.Type)
}
