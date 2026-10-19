package routegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// This list follows target section 7's service contracts, independently of the
// generator's adapter symbol table. Shared FcntlInt is descriptor-dispatched.
func TestTargetServiceRouteCoverage(t *testing.T) {
	t.Parallel()
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]Route{}
	for _, r := range m.Routes {
		routes[r.Name] = r
	}
	for _, group := range []struct{ owner, prefix, methods string }{
		{"fs", "", "Openat Renameat Getdents64 Fallocate Seek Pwrite Pread Sync Fstat Statat Unlinkat Truncate Chdir Getcwd Mkdirat Flock FcntlLock Map Unmap Advise"},
		{"net", "", "Socket Bind Listen Accept Connect LocalAddress PeerAddress SetOption GetOption"},
		{"fs", "FS", "ConfigureMachine SetCrashMode SetSyncFault InspectInode Enumerate NextSnapshot CloseIterator Restore CloseSnapshot"},
		{"net", "Net", "Configure ConfigureMachine Address SetLinkPolicy ResetLinkPolicy SetConnectivity"},
		{"kernel", "", "Read Write Close FcntlInt Getrandom Getpid Uname PollOpen PollClose PollWait PollReset PollDeadline PollUnblock"},
	} {
		for _, method := range strings.Fields(group.methods) {
			name := group.prefix + method
			r, ok := routes[name]
			if !ok || r.Owner != group.owner {
				t.Error("missing or misowned target service", name, group.owner)
			}
		}
	}
	// Every recorded route must actually have a dispatcher case. This also covers
	// lifecycle, self-exit and feature gates outside the section 7 syscall table.
	file, err := parser.ParseFile(token.NewFileSet(), "../kernel/dispatch_gen.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	handled := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, item := range clause.List {
			if name, ok := item.(*ast.Ident); ok && strings.HasPrefix(name.Name, "Route") {
				route := strings.TrimPrefix(name.Name, "Route")
				if handled[route] {
					t.Error("duplicate dispatch", route)
				}
				handled[route] = true
				if _, ok := routes[route]; !ok {
					t.Error("unrecorded dispatch", route)
				}
			}
		}
		return true
	})
	for name := range routes {
		if !handled[name] {
			t.Error("route has no dispatcher", name)
		}
	}
}
