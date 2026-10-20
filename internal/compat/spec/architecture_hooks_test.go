package spec_test

import (
	"strings"
	"testing"

	p125 "github.com/neria-cloud/gosim/internal/compat/go125/profile"
	p126 "github.com/neria-cloud/gosim/internal/compat/go126/profile"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func TestArchitectureHookSelection(t *testing.T) {
	t.Parallel()
	for _, family := range []struct {
		name       string
		descriptor func(string) (spec.Profile, error)
	}{{"go125", p125.DescriptorForArch}, {"go126", p126.DescriptorForArch}} {
		t.Run(family.name, func(t *testing.T) {
			for _, arch := range []string{"amd64", "arm64"} {
				p, err := family.descriptor(arch)
				if err != nil {
					t.Fatal(err)
				}
				if err := p.Validate(); err != nil {
					t.Fatal(err)
				}
				hooks := map[string]spec.Hook{}
				for _, h := range p.Hooks {
					hooks[h.Package+"."+h.Symbol] = h
					if !strings.Contains(h.Target, "/"+family.name+"/hooks/") {
						t.Fatal("non-translated or cross-family hook", h)
					}
				}
				for _, symbol := range []string{"cpuid", "xgetbv", "getGOAMD64level", "getisar0", "getMIDR", "getpfr0"} {
					_, present := hooks["internal/cpu."+symbol]
					x86 := symbol == "cpuid" || symbol == "xgetbv" || symbol == "getGOAMD64level"
					if present != (x86 == (arch == "amd64")) {
						t.Fatalf("%s: wrong CPU declaration %s", arch, symbol)
					}
				}
				for _, pkg := range []string{"syscall", "golang.org/x/sys/unix"} {
					_, present := hooks[pkg+".gettimeofday"]
					if present != (arch == "amd64") {
						t.Fatal("incorrect private gettimeofday architecture", pkg, arch)
					}
				}
				for _, symbol := range []string{"IndexByteString", "Equal", "IndexByte", "MakeNoZero", "Compare", "abigen_runtime_cmpstring", "Count", "CountString", "Index", "IndexString"} {
					if _, ok := hooks["internal/bytealg."+symbol]; !ok {
						t.Fatal("missing byte helper", symbol)
					}
				}
				// The descriptor is freshly decoded; architecture and caller mutations must
				// not contaminate later selections or fingerprints.
				p.Hooks[0].Symbol = "mutated"
				fresh, err := family.descriptor(arch)
				if err != nil || fresh.Hooks[0].Symbol == "mutated" {
					t.Fatal("shared hook backing storage")
				}
			}
			for _, bad := range []string{"", "386", "ARM64", "amd64/../arm64"} {
				if _, err := family.descriptor(bad); err == nil {
					t.Fatal("accepted unsupported architecture", bad)
				}
			}
		})
	}
}
