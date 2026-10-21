package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

type FamilyDefinition struct {
	Schema    int          `json:"schema"`
	Family    string       `json:"family"`
	Parent    string       `json:"parent,omitempty"`
	Supported bool         `json:"supported"`
	Delta     ProfileDelta `json:"delta"`
}
type RecordDelta[T any] struct {
	Add     []T        `json:"add,omitempty"`
	Replace []T        `json:"replace,omitempty"`
	Remove  [][]string `json:"remove,omitempty"`
}
type Declaration struct {
	Hook   Hook     `json:"hook"`
	Arches []string `json:"arches,omitempty"`
}
type PackageRef struct {
	Path string `json:"path"`
}
type ArchitectureDelta struct {
	Layouts   RecordDelta[LayoutContract]   `json:"layouts,omitempty"`
	Constants RecordDelta[ConstantContract] `json:"constants,omitempty"`
}
type ProfileDelta struct {
	Declarations    RecordDelta[Declaration]     `json:"declarations,omitempty"`
	Packages        RecordDelta[PackagePolicy]   `json:"packages,omitempty"`
	Bridges         RecordDelta[Bridge]          `json:"bridges,omitempty"`
	NativeImports   RecordDelta[NativeImport]    `json:"native_imports,omitempty"`
	SharedGlobals   RecordDelta[SharedGlobal]    `json:"shared_globals,omitempty"`
	Probes          RecordDelta[InitProbe]       `json:"probes,omitempty"`
	Modules         RecordDelta[ModulePin]       `json:"modules,omitempty"`
	Capabilities    RecordDelta[Capability]      `json:"capabilities,omitempty"`
	RuntimePackages RecordDelta[PackageRef]      `json:"runtime_packages,omitempty"`
	Architectures   map[string]ArchitectureDelta `json:"architectures,omitempty"`
}

// DecodeDefinition accepts only the authored definition format, never audit drafts.
func DecodeDefinition(data []byte) (FamilyDefinition, error) {
	var result FamilyDefinition
	scanner := json.NewDecoder(bytes.NewReader(data))
	scanner.UseNumber()
	value, err := uniqueJSONValue(scanner)
	if err != nil {
		return result, err
	}
	if _, err := scanner.Token(); err != io.EOF {
		return result, fmt.Errorf("trailing definition data: %v", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return result, fmt.Errorf("definition must be an object")
	}
	for _, name := range []string{"schema", "family", "supported", "delta"} {
		if object[name] == nil {
			return result, fmt.Errorf("missing definition field %s", name)
		}
	}
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&result); err != nil {
		return result, err
	}
	if result.Schema != 1 {
		return result, fmt.Errorf("unsupported definition schema %d", result.Schema)
	}
	if _, err := familyMinor(result.Family); err != nil {
		return result, err
	}
	if result.Parent != "" {
		if _, err := familyMinor(result.Parent); err != nil {
			return result, err
		}
	}
	if err := normalizeDefinitionValue(object); err != nil {
		return result, err
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return result, err
	}
	result = FamilyDefinition{}
	err = json.Unmarshal(normalized, &result)
	return result, err
}

func familyMinor(family string) (int, error) {
	v, err := buildspec.ParseGoVersion("go" + family + ".0")
	if err != nil || v.Major != 1 || v.Minor < 25 {
		return 0, fmt.Errorf("invalid compatibility family %q", family)
	}
	return v.Minor, nil
}

// uniqueJSONValue preserves numbers exactly while rejecting duplicate keys at any depth.
func uniqueJSONValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		result := map[string]any{}
		for d.More() {
			name, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := name.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key")
			}
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("duplicate JSON key %q", key)
			}
			value, err := uniqueJSONValue(d)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		_, err := d.Token()
		return result, err
	case json.Delim('['):
		result := []any{}
		for d.More() {
			value, err := uniqueJSONValue(d)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		_, err := d.Token()
		return result, err
	default:
		return token, nil
	}
}

// LoadDefinitions reads only immediate goNNN.json entries from a profile directory.
func LoadDefinitions(fsys fs.FS) (map[string]FamilyDefinition, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	result := map[string]FamilyDefinition{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".origin.json") || name == "evidence.json" {
			continue
		}
		if !strings.HasPrefix(name, "go") {
			return nil, fmt.Errorf("unexpected definition file %q", name)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		definition, err := DecodeDefinition(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if name != "go"+strings.ReplaceAll(definition.Family, ".", "")+".json" {
			return nil, fmt.Errorf("definition filename/family mismatch: %s", name)
		}
		if _, exists := result[definition.Family]; exists {
			return nil, fmt.Errorf("duplicate family %s", definition.Family)
		}
		result[definition.Family] = definition
	}
	return result, nil
}

var definitionKeys = map[string][]string{
	"declarations": {"hook.context", "hook.package", "hook.symbol", "arches"},
	"packages":     {"path"}, "runtime_packages": {"path"}, "modules": {"path"},
	"bridges": {"package"}, "native_imports": {"from", "package"},
	"shared_globals": {"package", "symbol"}, "capabilities": {"package", "symbol"},
	"probes":  {"package", "symbol", "phase", "operation"},
	"layouts": {"package", "type"}, "constants": {"package", "symbol"},
}

func canonicalDefinitionPath(value string) bool {
	if strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return false
	}
	if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "\\ \t\r\n:") || path.Clean(value) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Walk the strictly typed JSON shape so every nested path and delta uses one policy.
// Ordered payloads (layout fields, signatures, rewrite target names) are untouched.
func normalizeDefinitionValue(value any) error {
	switch v := value.(type) {
	case []any:
		for _, entry := range v {
			if err := normalizeDefinitionValue(entry); err != nil {
				return err
			}
		}
	case map[string]any:
		for name, entry := range v {
			if name == "arguments" {
				arguments, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("arguments must be an object")
				}
				for key, value := range arguments {
					list, ok := value.([]any)
					if key == "" || !ok || len(list) == 0 {
						return fmt.Errorf("invalid probe arguments")
					}
					seen := map[int64]bool{}
					for _, item := range list {
						number, ok := item.(json.Number)
						n, err := number.Int64()
						if !ok || err != nil || seen[n] {
							return fmt.Errorf("invalid/duplicate probe argument")
						}
						seen[n] = true
					}
					slices.SortFunc(list, func(a, b any) int {
						x, _ := strconv.ParseInt(string(a.(json.Number)), 10, 64)
						y, _ := strconv.ParseInt(string(b.(json.Number)), 10, 64)
						if x < y {
							return -1
						}
						if x > y {
							return 1
						}
						return 0
					})
				}
				continue
			}
			if fields, ok := definitionKeys[name]; ok {
				if err := normalizeRecordOperations(name, entry, fields); err != nil {
					return err
				}
			}
			switch name {
			case "path", "package", "from", "replacement", "source":
				text, ok := entry.(string)
				if !ok || !canonicalDefinitionPath(text) {
					return fmt.Errorf("invalid %s path %q", name, entry)
				}
			case "target":
				// Hook targets are qualified paths; SymbolRewrite targets are symbol names.
				if text, ok := entry.(string); !ok || !canonicalDefinitionPath(text) {
					return fmt.Errorf("invalid target %q", entry)
				}
			case "architectures":
				arches, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("architectures must be an object")
				}
				for arch := range arches {
					if arch != "amd64" && arch != "arm64" {
						return fmt.Errorf("unsupported architecture %q", arch)
					}
				}
			case "rewrites":
				list, ok := entry.([]any)
				if !ok {
					return fmt.Errorf("rewrites must be an array")
				}
				slices.SortFunc(list, func(a, b any) int {
					x, _ := a.(map[string]any)
					y, _ := b.(map[string]any)
					xs, _ := x["symbol"].(string)
					ys, _ := y["symbol"].(string)
					return strings.Compare(xs, ys)
				})
			case "arches", "native_symbols":
				list, ok := entry.([]any)
				if !ok {
					return fmt.Errorf("%s must be an array", name)
				}
				seen := map[string]bool{}
				for _, item := range list {
					text, ok := item.(string)
					if !ok || text == "" || seen[text] || name == "arches" && text != "amd64" && text != "arm64" {
						return fmt.Errorf("invalid/duplicate %s entry %q", name, item)
					}
					seen[text] = true
				}
				slices.SortFunc(list, func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
				if name == "arches" && len(list) != 1 {
					delete(v, name)
				}
			}
			if err := normalizeDefinitionValue(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeRecordOperations(section string, value any, fields []string) error {
	operations, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s delta must be an object", section)
	}
	for operation, value := range operations {
		list, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s.%s must be an array", section, operation)
		}
		type keyedRecord struct {
			key   string
			value any
		}
		ordered := make([]keyedRecord, 0, len(list))
		for _, item := range list {
			var key []string
			if operation == "remove" {
				tuple, ok := item.([]any)
				if !ok || len(tuple) != len(fields) {
					return fmt.Errorf("invalid %s removal tuple", section)
				}
				for _, part := range tuple {
					text, ok := part.(string)
					if !ok {
						return fmt.Errorf("non-string removal key")
					}
					key = append(key, text)
				}
			} else {
				record, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("%s record must be an object", section)
				}
				for _, field := range fields {
					var current any = record
					for _, name := range strings.Split(field, ".") {
						object, _ := current.(map[string]any)
						current = object[name]
					}
					text, _ := current.(string)
					if field == "arches" {
						text = "*"
						if a, ok := current.([]any); ok && len(a) == 1 {
							text, _ = a[0].(string)
						}
					}
					key = append(key, text)
				}
			}
			ordered = append(ordered, keyedRecord{tupleKey(key), item})
			for i, field := range fields {
				part := key[i]
				switch field {
				case "hook.context":
					if part != "" && part != "host" && part != "host-race" {
						return fmt.Errorf("invalid declaration context %q", part)
					}
				case "arches":
					if part != "*" && part != "amd64" && part != "arm64" {
						return fmt.Errorf("invalid declaration scope %q", part)
					}
				case "path", "package", "from", "hook.package":
					if !canonicalDefinitionPath(part) {
						return fmt.Errorf("invalid %s key path %q", section, part)
					}
				default:
					if part == "" {
						return fmt.Errorf("missing %s key %s", section, field)
					}
				}
			}
		}
		slices.SortFunc(ordered, func(a, b keyedRecord) int { return strings.Compare(a.key, b.key) })
		for i, record := range ordered {
			list[i] = record.value
		}

	}
	return nil
}
