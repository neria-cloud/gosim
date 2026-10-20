package routegen

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

var layoutNames = strings.Fields(
	"Timespec Stat_t Statfs_t Fsid Dirent Flock_t RawSockaddr " +
		"RawSockaddrInet4 RawSockaddrAny Utsname Rlimit",
)

//go:embed layout_amd64.json
var amd64Layout []byte

//go:embed layout_arm64.json
var arm64Layout []byte

type layoutManifest struct {
	Types   map[string]map[string]string `json:"types"`
	Numbers map[string]string            `json:"numbers"`
}

func renderLayouts(sources map[string]sourcePackage, arch, profileID string) ([]byte, error) {
	data := amd64Layout
	if arch == "arm64" {
		data = arm64Layout
	} else if arch != "amd64" {
		return nil, fmt.Errorf("unimplemented layouts %s", arch)
	}
	var expected layoutManifest
	if err := json.Unmarshal(data, &expected); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	fmt.Fprintf(
		&b, "// Code generated from pinned Linux source by gosim-gen. DO NOT EDIT.\n"+
			"//go:build %s && %s\n\npackage kernel\n",
		familyBounds(profileID), arch,
	)
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
		prefix := ""
		if pkg != "syscall" {
			prefix = "X"
		}
		for _, name := range layoutNames {
			expr := sources[pkg].Types[name]
			if expr == nil || sourceText(expr) != expected.Types[pkg][name] {
				return nil, fmt.Errorf("Linux %s ABI layout drift: %s.%s", arch, pkg, name)
			}
			// The approved records contain no pointers. Do not accidentally make a
			// pointer-free linkname representation compatible solely by byte size.
			bad := false
			ast.Inspect(expr, func(n ast.Node) bool {
				switch n.(type) {
				case *ast.StarExpr, *ast.InterfaceType, *ast.MapType, *ast.ChanType, *ast.FuncType:
					bad = true
				}
				return true
			})
			if bad {
				return nil, fmt.Errorf("pointer-bearing ABI record %s", name)
			}
			copied, err := parser.ParseExpr(sourceText(expr))
			if err != nil {
				return nil, err
			}
			ast.Inspect(copied, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && slices.Contains(layoutNames, id.Name) {
					id.Name = prefix + id.Name
				}
				return true
			})
			fmt.Fprintf(&b, "type %s%s %s\n", prefix, name, sourceText(copied))
		}
	}
	names := make([]string, 0, len(expected.Numbers))
	for name := range expected.Numbers {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
			expr := sources[pkg].Constants[name]
			// Go names arm64 syscall 79 FSTATAT; x/sys calls it NEWFSTATAT.
			if arch == "arm64" && pkg == "golang.org/x/sys/unix" && name == "SYS_FSTATAT" {
				expr = sources[pkg].Constants["SYS_NEWFSTATAT"]
			}
			if pkg == "syscall" && name == "SYS_GETRANDOM" {
				expr = sources["internal/syscall/unix"].Constants["getrandomTrap"]
			}
			if expr == nil ||
				constant.MakeFromLiteral(sourceText(expr), token.INT, 0).ExactString() != expected.Numbers[name] {
				return nil, fmt.Errorf("Linux syscall number drift: %s.%s", pkg, name)
			}
		}
		fmt.Fprintf(&b, "const Linux%s = %s\n", name, expected.Numbers[name])
	}
	return format.Source(b.Bytes())
}
