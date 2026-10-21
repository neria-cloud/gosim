package kernel

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
)

func TestSyscallCatalogueCoversManifest(t *testing.T) {
	data, err := os.ReadFile("../routegen/routes.json")
	counterCheck(t, err)
	var manifest struct {
		Routes []struct {
			ID                RouteID
			Name, Owner, Kind string
			Inputs            []struct{ Name, Type string }
		}
	}
	counterCheck(t, json.Unmarshal(data, &manifest))
	seen := map[RouteID]bool{}
	var maxID RouteID
	for _, route := range manifest.Routes {
		maxID = max(maxID, route.ID)
		slot := route.ID
		if route.Name == "FcntlLock" {
			slot = RouteFcntlInt
		}
		metadata := syscallCatalogue[slot]
		if route.Kind != "syscall" {
			if metadata.name != "" || syscallRouteHasFD(route.ID) {
				t.Errorf("non-syscall %s is counted", route.Name)
			}
			continue
		}
		seen[slot] = true
		wantGroup := route.Owner
		if wantGroup == "kernel" || route.Name == "FcntlLock" {
			wantGroup = "rt"
		}
		if metadata.name == "" || featureName(metadata.group) != wantGroup {
			t.Errorf("route %s metadata %+v", route.Name, metadata)
		}
		wantFD := false
		for _, input := range route.Inputs {
			wantFD = wantFD || input.Name == "FD" && input.Type == "int"
		}
		if syscallInfo(slot).DescriptorBased != wantFD || syscallRouteHasFD(route.ID) != wantFD {
			t.Errorf("FD metadata %s", route.Name)
		}
	}
	if routeLimit != maxID+1 {
		t.Fatalf("limit %d max %d", routeLimit, maxID)
	}
	seen[syscallDataSyncSlot] = true
	groups := map[string][]string{
		"rt":  strings.Fields("read write writev close fcntl getrandom getpid uname"),
		"fs":  strings.Fields("openat renameat getdents64 fallocate lseek pwrite64 pread64 fsync fdatasync fstat fstatat unlinkat ftruncate chdir getcwd mkdirat flock mmap munmap madvise statfs fstatfs fchmod fchmodat fchown fchownat"),
		"net": strings.Fields("socket bind listen accept4 connect getsockname getpeername setsockopt getsockopt"),
	}
	want := map[string]string{}
	for group, names := range groups {
		for _, name := range names {
			want[name] = group
		}
	}
	names := map[string]bool{}
	descriptorBased := 0
	for slot, metadata := range syscallCatalogue {
		if metadata.name == "" {
			continue
		}
		if !seen[RouteID(slot)] || names[metadata.name] || want[metadata.name] != featureName(metadata.group) {
			t.Errorf("unexpected/duplicate catalogue entry %d: %+v", slot, metadata)
		}
		names[metadata.name] = true
		if syscallInfo(RouteID(slot)).DescriptorBased {
			descriptorBased++
		}
	}
	if len(names) != 43 || descriptorBased != 27 {
		t.Fatalf("names %d descriptor-based %d", len(names), descriptorBased)
	}
	inv := Invocation{Route: syscallDataSyncSlot}
	if err := validateRoutePayload(&inv); err == nil {
		t.Fatal("extra counter slot is callable")
	}
}

func TestSyscallCatalogueFeatureSets(t *testing.T) {
	for _, test := range []struct {
		features contracts.FeatureSet
		size     int
	}{
		{contracts.FeatureSet(contracts.RT), 8},
		{contracts.FeatureSet(contracts.RT | contracts.FS), 34},
		{contracts.FeatureSet(contracts.RT | contracts.Net), 17},
		{contracts.FeatureSet(contracts.RT | contracts.FS | contracts.Net), 43},
	} {
		var log []string
		fs, net, _ := providersFor(&log)
		var providers []contracts.Provider
		if test.features.Has(contracts.FS) {
			providers = append(providers, fs)
		}
		if test.features.Has(contracts.Net) {
			providers = append(providers, net)
		}
		_, err := fixture(t, test.features, providers, func(k *Kernel) {
			ref := createFor(t, k, "catalogue", idleMain)
			entries, err := k.CountableSyscalls(ref)
			counterCheck(t, err)
			if len(entries) != test.size {
				t.Fatalf("features %d: got %d want %d", test.features, len(entries), test.size)
			}
			for _, entry := range entries {
				if entry.Group == "fs" && !test.features.Has(contracts.FS) || entry.Group == "net" && !test.features.Has(contracts.Net) {
					t.Fatal("disabled name", entry)
				}
			}
		})
		counterCheck(t, err)
	}
}
