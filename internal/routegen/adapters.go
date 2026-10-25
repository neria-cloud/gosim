package routegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"slices"
	"strconv"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

// A declaration is selected explicitly even when several upstream names map to
// the same service. No absent function can disappear from generated output.
var adapterOperations = map[string]string{
	"readlinkat": "Readlinkat", "Readlinkat": "Readlinkat", "symlinkat": "Symlinkat", "Symlinkat": "Symlinkat",
	"accept4": "Accept", "bind": "Bind", "Chdir": "Chdir", "Close": "Close", "connect": "Connect",
	"Fallocate": "Fallocate", "fcntl": "FcntlInt", "FcntlInt": "FcntlInt", "FcntlFlock": "FcntlLock",
	"Fdatasync": "DataSync", "Flock": "Flock", "Fstat": "Fstat", "Fstatfs": "FstatFS", "Statfs": "StatFS", "fstatat": "Statat", "Fstatat": "Statat", "Fsync": "Sync",
	"Fchmod": "Fchmod", "Fchmodat": "Chmodat", "Fchown": "Fchown", "Fchownat": "Chownat",
	"Ftruncate": "Truncate", "Getcwd": "Getcwd", "Getdents": "Getdents64", "getpeername": "PeerAddress", "Getpid": "Getpid",
	"Getrandom": "Getrandom", "GetRandom": "Getrandom", "vgetrandom": "VGetrandom", "getsockname": "LocalAddress", "getsockopt": "GetOption",
	"Listen": "Listen", "Madvise": "Advise", "Mkdirat": "Mkdirat", "mmap": "Map", "munmap": "Unmap", "openat": "Openat", "Openat": "Openat",
	"pread": "Pread", "pwrite": "Pwrite", "read": "Read", "Renameat": "Renameat", "Seek": "Seek", "setsockopt": "SetOption", "socket": "Socket",
	"Uname": "Uname", "unlinkat": "Unlinkat", "Unlinkat": "Unlinkat", "write": "Write", "Getrlimit": "Getrlimit",
	"RawSyscall": "Reject", "RawSyscall6": "Reject", "Syscall": "Reject", "Syscall6": "Reject", "rawSyscallNoError": "Reject",
	"rawVforkSyscall": "Reject", "SyscallNoError": "Reject", "RawSyscallNoError": "Reject",
}

func adapterOwner(operation string) string {
	if slices.Contains(strings.Fields("Symlinkat Readlinkat Openat Renameat Getdents64 Fallocate Seek Pwrite Pread Sync DataSync Fstat Statat StatFS FstatFS Fchmod Chmodat Fchown Chownat Unlinkat Truncate Chdir Getcwd Mkdirat Flock FcntlLock Map Unmap Advise"), operation) {
		return "fs"
	}
	if slices.Contains(strings.Fields("Socket Bind Listen Accept Connect LocalAddress PeerAddress SetOption GetOption"), operation) {
		return "net"
	}
	return "kernel"
}

func adapterSymbols(pkg string) []string {
	symbols := strings.Fields("accept4 bind Chdir Close connect Fallocate Fdatasync Flock Fstat Fstatfs Statfs Fsync Ftruncate Getcwd Getdents getpeername Getpid getsockname getsockopt Listen Madvise Mkdirat mmap munmap openat pread pwrite read Renameat Seek setsockopt socket Uname write fcntl FcntlFlock Getrlimit RawSyscall RawSyscall6 Syscall Syscall6")
	switch pkg {
	case "syscall":
		return append(symbols, "readlinkat", "symlinkat", "fstatat", "unlinkat", "rawSyscallNoError", "rawVforkSyscall", "Fchmod", "Fchmodat", "Fchown", "Fchownat")
	case "golang.org/x/sys/unix":
		return append(symbols, "Readlinkat", "Symlinkat", "Fstatat", "Unlinkat", "Getrandom", "vgetrandom", "FcntlInt", "SyscallNoError", "RawSyscallNoError", "Fchmod", "Fchmodat", "Fchown", "Fchownat")
	case "internal/syscall/unix":
		return strings.Fields("fcntl GetRandom vgetrandom Openat Unlinkat Fchmodat Fchownat Fstatat Mkdirat Renameat Readlinkat Symlinkat")
	}
	panic(pkg)
}

func adapterPrefix(pkg string) string {
	return map[string]string{"syscall": "Syscall_", "golang.org/x/sys/unix": "GolangOrgXSysUnix_", "internal/syscall/unix": "InternalSyscallUnix_"}[pkg]
}

func adapterHooks(profile spec.Profile, sources map[string]sourcePackage) ([]spec.Hook, error) {
	var hooks []spec.Hook
	targets := map[string]spec.Hook{}
	for _, h := range profile.Hooks {
		targets[h.Package+"."+h.Symbol] = h
	}
	for _, pkg := range []string{"syscall", "golang.org/x/sys/unix", "internal/syscall/unix"} {
		for _, symbol := range adapterSymbols(pkg) {
			fn := sources[pkg].Functions[symbol]
			if fn == nil {
				return nil, fmt.Errorf("missing adapter declaration %s.%s", pkg, symbol)
			}
			signature, err := signatureType(fn, false, pkg, "kernel")
			if err != nil {
				return nil, err
			}
			owner := adapterOwner(adapterOperations[symbol])
			hooks = append(hooks, spec.Hook{Package: pkg, Symbol: symbol, Signature: signature, Target: targets[pkg+"."+symbol].Target, Owner: owner})
		}
	}
	return hooks, nil
}

func signatureType(source *ast.FuncType, target bool, pkg, owner string) (string, error) {
	// Work on a fresh AST: the source index also backs manifest verification.
	expr, err := parser.ParseExpr(sourceText(source))
	if err != nil {
		return "", err
	}
	fn := expr.(*ast.FuncType)
	for _, fields := range []*ast.FieldList{fn.Params, fn.Results} {
		if fields == nil {
			continue
		}
		var expanded []*ast.Field
		for _, field := range fields.List {
			for range max(1, len(field.Names)) {
				expanded = append(expanded, &ast.Field{Type: field.Type})
			}
		}
		fields.List = expanded
	}
	if !target {
		return sourceText(fn), nil
	}
	for i, field := range fn.Params.List {
		field.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("p%d", i))}
	}
	// All substituted records are generated from the selected source. Their
	// layouts and GC pointer maps are verified before permitting ABI linknames.
	for _, fields := range []*ast.FieldList{fn.Params, fn.Results} {
		if fields == nil {
			continue
		}
		for _, field := range fields.List {
			field.Type = adapterType(field.Type, pkg, owner)
		}
	}
	return sourceText(fn), nil
}

func adapterType(expr ast.Expr, pkg, owner string) ast.Expr {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := x.X.(*ast.Ident); ok && ident.Name == "syscall" && slices.Contains(layoutNames, x.Sel.Name) {
			return adapterType(x.Sel, "syscall", owner)
		}
	case *ast.StarExpr:
		x.X = adapterType(x.X, pkg, owner)
	case *ast.Ident:
		if x.Name == "Errno" {
			return &ast.SelectorExpr{X: ast.NewIdent("syscall"), Sel: ast.NewIdent("Errno")}
		}
		if x.Name == "_Socklen" {
			return ast.NewIdent("uint32")
		}
		if x.Name == "GetRandomFlag" {
			return ast.NewIdent("uintptr")
		}
		if slices.Contains(layoutNames, x.Name) {
			name := x.Name
			if pkg == "golang.org/x/sys/unix" {
				name = "X" + name
			}
			if owner == "kernel" {
				return ast.NewIdent(name)
			}
			return &ast.SelectorExpr{X: ast.NewIdent("kh"), Sel: ast.NewIdent(name)}
		}
	}
	return expr
}

func renderAdapters(tc buildspec.Toolchain, profile spec.Profile, arch string, m Manifest, root string) (map[string][]byte, error) {
	if arch != "amd64" && arch != "arm64" {
		return nil, fmt.Errorf("unimplemented adapters %s", arch)
	}
	sources, err := loadSources(tc, arch)
	if err != nil {
		return nil, err
	}
	hooks, err := adapterHooks(profile, sources)
	if err != nil {
		return nil, err
	}
	expected := map[string]spec.Hook{}
	for _, h := range profile.Hooks {
		expected[h.Package+"."+h.Symbol] = h
	}
	for _, h := range hooks {
		if expected[h.Package+"."+h.Symbol] != h {
			return nil, fmt.Errorf("unreviewed adapter ABI %s.%s: %s", h.Package, h.Symbol, h.Signature)
		}
	}
	packages, err := adapterPackages(hooks)
	if err != nil {
		return nil, err
	}
	source := spec.SourceContext{OS: "linux", Arch: arch}
	layoutPath, err := selectedGeneratedPath(root, packages["kernel"], "layouts", source, profile.Support.Family)
	if err != nil {
		return nil, err
	}
	bounds, err := authoredConstraint(root, layoutPath)
	if err != nil {
		return nil, err
	}
	outputs := map[string][]byte{}
	layouts, err := renderLayouts(sources, arch, bounds)
	if err != nil {
		return nil, err
	}
	outputs[layoutPath] = layouts
	for _, owner := range []string{"kernel", "fs", "net"} {
		destination, err := selectedGeneratedPath(root, packages[owner], "syscalls", source, profile.Support.Family)
		if err != nil {
			return nil, err
		}
		bounds, err := authoredConstraint(root, destination)
		if err != nil {
			return nil, err
		}
		var b bytes.Buffer
		fmt.Fprintf(&b, "// Code generated by gosim-gen from reviewed Go and pinned x/sys sources. DO NOT EDIT.\n%s\npackage %s\n", bounds, owner)
		b.WriteString("import (\"strings\";\"syscall\";\"unsafe\"; k \"github.com/neria-cloud/gosim/internal/kernel\";\"github.com/neria-cloud/gosim/internal/contracts\";\"github.com/neria-cloud/gosim/rt/runtime/abi\";simrt \"github.com/neria-cloud/gosim/rt/runtime\"")
		if owner != "kernel" {
			b.WriteString("; kh " + strconv.Quote(packages["kernel"]))
		}
		b.WriteString(")\nvar _=strings.IndexByte;var _=syscall.EINVAL;var _ unsafe.Pointer;var _=abi.ByteView{};var _=simrt.RaceEnabled;var _=contracts.RT\n")
		for _, h := range hooks {
			if h.Owner != owner {
				continue
			}
			signature, err := signatureType(sources[h.Package].Functions[h.Symbol], true, h.Package, owner)
			if err != nil {
				return nil, err
			}
			name := adapterPrefix(h.Package) + h.Symbol
			body, err := adapterBody(h, m)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&b, "\nfunc %s%s {\n%s\n}\n", name, strings.TrimPrefix(signature, "func"), body)
		}
		helpers := adapterHelpers(owner)
		if arch == "arm64" && owner == "fs" {
			// Linux arm64 stat.nlink is 32 bits; reject loss before writing output.
			helpers = strings.ReplaceAll(helpers, "Nlink:stat.Nlink", "Nlink:uint32(stat.Nlink)")
			helpers = strings.ReplaceAll(helpers, "out:=kh.Stat_t", "if stat.Nlink>uint64(^uint32(0)){kh.Reject(\"stat.nlink overflow\")}\n out:=kh.Stat_t")
		}
		b.WriteString(helpers)
		code, err := format.Source(b.Bytes())
		if err != nil {
			return nil, fmt.Errorf("render %s adapters: %w", owner, err)
		}
		outputs[destination] = code
	}
	return outputs, nil
}

// Keep a stable digest of the complete architecture-specific source, in addition
// to transport/provenance fingerprints. No timestamps or absolute source paths.
func adaptersDigest(outputs map[string][]byte) string {
	data, _ := json.Marshal(outputs)
	return buildspec.Hash(data)
}
