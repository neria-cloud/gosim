// Package routegen owns the reviewed transport schema. It runs only in host
// tooling; translated code imports generated kernel payloads, never this package.
package routegen

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"slices"

	"github.com/neria-cloud/gosim/internal/buildspec"
)

//go:embed routes.json
var manifest []byte

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type Route struct {
	ID        uint32   `json:"id"`
	Name      string   `json:"name"`
	Owner     string   `json:"owner"`
	Kind      string   `json:"kind"`
	Inputs    []Field  `json:"inputs"`
	Results   []Field  `json:"results"`
	Legacy    []string `json:"legacy"`
	Migration string   `json:"migration,omitempty"`
}
type Manifest struct {
	Schema int     `json:"schema"`
	Routes []Route `json:"routes"`
}

func Load() (Manifest, error) { return Parse(manifest) }
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("trailing route manifest data")
	}
	return m, m.Validate()
}

func (m Manifest) Validate() error {
	if m.Schema != 1 || len(m.Routes) == 0 {
		return fmt.Errorf("unsupported/empty route schema %d", m.Schema)
	}
	names, ids, legacy := map[string]bool{}, map[uint32]bool{}, map[string]bool{}
	var previous uint32
	for _, r := range m.Routes {
		if r.ID == 0 || r.ID <= previous || ids[r.ID] || !exported(r.Name) || names[r.Name] || !slices.Contains([]string{"kernel", "fs", "net"}, r.Owner) || !slices.Contains([]string{"syscall", "poll", "wait", "control", "self"}, r.Kind) {
			return fmt.Errorf("invalid or duplicate route %d %s", r.ID, r.Name)
		}
		if (r.Kind == "poll" || r.Kind == "wait" || r.Kind == "self") && r.Owner != "kernel" {
			return fmt.Errorf("invalid route owner %s", r.Name)
		}
		for i, fields := range [][]Field{r.Inputs, r.Results} {
			seen := map[string]bool{}
			for _, f := range fields {
				if !exported(f.Name) || seen[f.Name] || !allowedType(f.Type) {
					return fmt.Errorf("invalid field %s.%s %s", r.Name, f.Name, f.Type)
				}
				if f.Type == "func()" && (i != 0 || f.Name != "Main" || (r.Name != "Create" && r.Name != "SetMain")) {
					return fmt.Errorf("arbitrary callback in route %s", r.Name)
				}
				seen[f.Name] = true
			}
		}
		for _, old := range r.Legacy {
			if !exported(old) || legacy[old] || r.Migration == "" {
				return fmt.Errorf("invalid legacy route disposition %s", old)
			}
			legacy[old] = true
		}
		names[r.Name], ids[r.ID], previous = true, true, r.ID
	}
	return nil
}

func exported(name string) bool { return token.IsIdentifier(name) && token.IsExported(name) }
func allowedType(typ string) bool {
	return slices.Contains([]string{
		"int", "int64", "uint8", "uint32", "uint64", "uintptr", "bool", "string", "[]string", "[4]byte", "func()",
		"abi.ByteView", "abi.ValueView[contracts.Lock]", "contracts.MachineRef", "contracts.StopKind", "contracts.Stat", "contracts.Address",
		"contracts.FSConfig", "contracts.FSFaultConfig", "contracts.InodeInfo", "contracts.NetConfig", "contracts.NetLink", "contracts.NetLinkPolicy",
		"ProcessInfo", "ExitRecord", "State", "contracts.Feature",
	}, typ)
}

func (m Manifest) Digest() string {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	} // schema contains only JSON value fields
	return buildspec.Hash(data)
}
