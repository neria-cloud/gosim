package spec

import (
	"fmt"
	"slices"
	"strings"
)

// Policy resolves an audited exact path or /... subtree. Unknown application
// packages are translated; unknown standard-library packages require review.
// No missing rule ever selects native execution.
func (p Profile) Policy(path string, standard bool) (PackagePolicy, error) {
	if path == "" || strings.ContainsAny(path, "\\ \t\n") || strings.HasPrefix(path, ".") {
		return PackagePolicy{}, fmt.Errorf("invalid package path %q", path)
	}
	var found PackagePolicy
	best := -1
	for _, policy := range p.Packages {
		prefix, subtree := strings.CutSuffix(policy.Path, "/...")
		if path == policy.Path || subtree && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
			score := len(prefix) * 2
			if !subtree {
				score++
			}
			if score > best {
				found, best = policy, score
			}
		}
	}
	if best < 0 {
		if standard {
			return PackagePolicy{}, fmt.Errorf("unreviewed standard-library package %s in %s", path, p.ID)
		}
		return PackagePolicy{Path: path, Mode: "translate", Reason: "Application code stays in the translated type universe; private ABI and cgo checks still apply."}, nil
	}
	if found.Mode == "reject" {
		return found, fmt.Errorf("unsupported application package %s in %s: %s", path, p.ID, found.Reason)
	}
	return found, nil
}

type SymbolRewrite struct {
	Symbol  string `json:"symbol"`
	Package string `json:"package"`
	Target  string `json:"target"`
}

// Bridge either substitutes a complete translated package or permits an exact
// set of native symbols. Rewrites take precedence over native symbol admission.
type Bridge struct {
	Package       string          `json:"package"`
	Replacement   string          `json:"replacement,omitempty"`
	NativeIsland  bool            `json:"native_island,omitempty"`
	NativeSymbols []string        `json:"native_symbols,omitempty"`
	Rewrites      []SymbolRewrite `json:"rewrites,omitempty"`
	Reason        string          `json:"reason"`
}

type InitProbe struct {
	Package     string             `json:"package"`
	Symbol      string             `json:"symbol"`
	Phase       string             `json:"phase"`
	Disposition string             `json:"disposition"`
	Feature     string             `json:"feature"`
	Operation   string             `json:"operation"`
	Response    string             `json:"response"`
	Source      string             `json:"source"`
	Arguments   map[string][]int64 `json:"arguments,omitempty"`
}

type ModulePin struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Sum      string `json:"sum"`
	GoModSum string `json:"go_mod_sum"`
	Reason   string `json:"reason"`
}

type Capability struct {
	Package string `json:"package"`
	Symbol  string `json:"symbol"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type NativeImport struct {
	From    string `json:"from"`
	Package string `json:"package"`
	Reason  string `json:"reason"`
}

type SharedGlobal struct {
	Package string `json:"package"`
	Symbol  string `json:"symbol"`
	Reason  string `json:"reason"`
}

type PolicyManifest struct {
	SharedGlobals []SharedGlobal
	NativeImports []NativeImport
	Bridges       []Bridge
	Probes        []InitProbe
	Modules       []ModulePin
	Capabilities  []Capability
}

func (m PolicyManifest) Validate(p Profile) error {
	shared := make(map[string]bool)
	for _, global := range m.SharedGlobals {
		key := global.Package + "." + global.Symbol
		if global.Package != module+"/internal/control" || global.Symbol != "binding" || global.Reason == "" || shared[key] {
			return fmt.Errorf("invalid shared translated global %s", key)
		}
		shared[key] = true
	}

	native := make(map[string]bool)
	for _, entry := range m.NativeImports {
		key := entry.From + ":" + entry.Package
		if !strings.HasPrefix(entry.From, module+"/internal/compat/"+p.ID+"/") || entry.Package == "" || entry.Reason == "" || native[key] {
			return fmt.Errorf("invalid native import %s", key)
		}
		native[key] = true
	}
	bridges := make(map[string]bool)
	for _, bridge := range m.Bridges {
		policy, err := p.Policy(bridge.Package, true)
		if err != nil || policy.Mode != "bridge" || bridges[bridge.Package] || bridge.Reason == "" {
			return fmt.Errorf("invalid bridge %s", bridge.Package)
		}
		if bridge.NativeIsland && (bridge.Package != module+"/rt/runtime" && bridge.Package != module+"/rt/runtime/abi" || bridge.Replacement != "" || len(bridge.NativeSymbols) > 0 || len(bridge.Rewrites) > 0) {
			return fmt.Errorf("invalid native island %s", bridge.Package)
		}
		if bridge.Replacement != "" && (!strings.HasPrefix(bridge.Replacement, module+"/internal/compat/"+p.ID+"/") || len(bridge.NativeSymbols) > 0 || len(bridge.Rewrites) > 0) {
			return fmt.Errorf("invalid bridge replacement %s", bridge.Package)
		}
		symbols := make(map[string]bool)
		for _, symbol := range bridge.NativeSymbols {
			if symbol == "" || symbols[symbol] {
				return fmt.Errorf("invalid native symbol %s.%s", bridge.Package, symbol)
			}
			symbols[symbol] = true
		}
		for _, rewrite := range bridge.Rewrites {
			if rewrite.Symbol == "" || rewrite.Package == "" || rewrite.Target == "" || symbols[rewrite.Symbol] {
				return fmt.Errorf("invalid rewrite %s.%s", bridge.Package, rewrite.Symbol)
			}
			if err := p.checkPackage(rewrite.Package); err != nil {
				return err
			}
			symbols[rewrite.Symbol] = true
		}
		bridges[bridge.Package] = true
	}
	for _, policy := range p.Packages {
		if policy.Mode == "bridge" && !bridges[policy.Path] {
			return fmt.Errorf("bridge %s lacks a boundary contract", policy.Path)
		}
	}
	probes := make(map[string]bool)
	for _, probe := range m.Probes {
		key := probe.Package + "." + probe.Symbol
		if probe.Package == "" || probe.Symbol == "" || probes[key] || !slices.Contains([]string{"init", "first-use", "operation"}, probe.Phase) || !slices.Contains([]string{"modeled", "guarded", "reject"}, probe.Disposition) || !slices.Contains([]string{"rt", "fs", "net"}, probe.Feature) || probe.Operation == "" || probe.Response == "" || !strings.HasPrefix(probe.Source, "src/") {
			return fmt.Errorf("invalid init probe %s", key)
		}
		if probe.Disposition == "guarded" && probe.Feature == "rt" {
			return fmt.Errorf("domain probe %s lacks an optional feature", key)
		}
		for name, values := range probe.Arguments {
			if name == "" || len(values) == 0 {
				return fmt.Errorf("invalid probe argument %s.%s", key, name)
			}
			seen := make(map[int64]bool)
			for _, value := range values {
				if seen[value] {
					return fmt.Errorf("duplicate probe argument %s.%s", key, name)
				}
				seen[value] = true
			}
		}
		probes[key] = true
	}
	modules := make(map[string]bool)
	for _, pin := range m.Modules {
		if pin.Path == "" || pin.Version == "" || !strings.HasPrefix(pin.Sum, "h1:") || !strings.HasPrefix(pin.GoModSum, "h1:") || pin.Reason == "" || modules[pin.Path] {
			return fmt.Errorf("invalid module pin %s", pin.Path)
		}
		modules[pin.Path] = true
	}
	capabilities := make(map[string]bool)
	for _, c := range m.Capabilities {
		key := c.Package + "." + c.Symbol
		if c.Package == "" || c.Symbol == "" || !slices.Contains([]string{"implemented", "limited", "stub", "rejected"}, c.Status) || c.Reason == "" || capabilities[key] {
			return fmt.Errorf("invalid capability %s", key)
		}
		capabilities[key] = true
	}
	return nil
}

// ResolveSymbol prevents a bridge import from granting unrestricted native
// execution. An empty symbol is the package import itself (including embed's
// blank import for byte/string embedding), never permission to use its values.
func (m PolicyManifest) ResolveSymbol(p Profile, path, symbol string, standard bool) (SymbolRewrite, error) {
	policy, err := p.Policy(path, standard)
	if err != nil {
		return SymbolRewrite{}, err
	}
	for _, c := range m.Capabilities {
		if c.Package == path && c.Symbol == symbol && c.Status == "rejected" {
			return SymbolRewrite{}, fmt.Errorf("unsupported %s.%s: %s", path, symbol, c.Reason)
		}
	}
	if policy.Mode != "bridge" {
		return SymbolRewrite{Symbol: symbol, Package: path, Target: symbol}, nil
	}
	for _, bridge := range m.Bridges {
		if bridge.Package != path {
			continue
		}
		if bridge.NativeIsland {
			return SymbolRewrite{Symbol: symbol, Package: path, Target: symbol}, nil
		}
		if symbol == "" {
			return SymbolRewrite{Package: bridge.Replacement}, nil
		}
		if bridge.Replacement != "" {
			return SymbolRewrite{Symbol: symbol, Package: bridge.Replacement, Target: symbol}, nil
		}
		for _, rewrite := range bridge.Rewrites {
			if rewrite.Symbol == symbol {
				return rewrite, nil
			}
		}
		if slices.Contains(bridge.NativeSymbols, symbol) {
			return SymbolRewrite{Symbol: symbol, Package: path, Target: symbol}, nil
		}
		return SymbolRewrite{}, fmt.Errorf("unreviewed native boundary %s.%s in %s", path, symbol, p.ID)
	}
	return SymbolRewrite{}, fmt.Errorf("missing bridge contract for %s", path)
}

// Probe is an exact named lookup. Missing operations cannot become successful
// init stubs, and a disabled domain cannot be enabled by importing its package.
func (m PolicyManifest) Probe(pkg, symbol string, features []string, arguments map[string]int64) (InitProbe, error) {
	for _, probe := range m.Probes {
		if probe.Package != pkg || probe.Symbol != symbol {
			continue
		}
		if len(arguments) != len(probe.Arguments) {
			return probe, fmt.Errorf("invalid initialization arguments for %s.%s", pkg, symbol)
		}
		for name, values := range probe.Arguments {
			value, ok := arguments[name]
			if !ok || !slices.Contains(values, value) {
				return probe, fmt.Errorf("unsupported initialization argument %s.%s.%s", pkg, symbol, name)
			}
		}
		if probe.Disposition == "reject" {
			return probe, fmt.Errorf("unsupported operation %s: %s", probe.Operation, probe.Response)
		}
		if probe.Disposition == "guarded" && !slices.Contains(features, probe.Feature) {
			return probe, fmt.Errorf("FeatureDisabled: feature=%s operation=%s", probe.Feature, probe.Operation)
		}
		return probe, nil
	}
	return InitProbe{}, fmt.Errorf("unclassified initialization operation %s.%s", pkg, symbol)
}

// AllowNativeImport checks explicit notranslate imports. An application cannot
// opt an I/O/time/container package out of translation with a source comment.
func (m PolicyManifest) AllowNativeImport(p Profile, from, path string, standard bool) error {
	policy, err := p.Policy(path, standard)
	if err == nil && policy.Mode == "native-pure" {
		return nil
	}
	for _, entry := range m.NativeImports {
		if entry.From == from && entry.Package == path {
			return nil
		}
	}
	return fmt.Errorf("unreviewed native import %s from %s", path, from)
}
