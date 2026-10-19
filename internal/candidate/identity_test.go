package candidate

import (
	"testing"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

func checkArtifactIdentitySeparation(t *testing.T, artifact gosimtool.Artifact) {
	t.Helper()
	for name, change := range map[string]func(*buildspec.Identity){
		"toolchain/profile": func(id *buildspec.Identity) {
			if id.Profile == "go125" {
				id.Toolchain, id.Profile = "go1.26.9", "go126"
			} else {
				id.Toolchain, id.Profile = "go1.25.14", "go125"
			}
		},
		"profile revision": func(id *buildspec.Identity) { id.AdapterRevision = buildspec.Hash([]byte("changed adapter")) },
		"source":           func(id *buildspec.Identity) { id.TranslatorDigest = buildspec.Hash([]byte("changed source")) },
		"runtime":          func(id *buildspec.Identity) { id.RuntimeRevision = buildspec.Hash([]byte("changed runtime")) },
		"race":             func(id *buildspec.Identity) { id.Race = !id.Race },
		"debug policy":     func(id *buildspec.Identity) { id.DebugPolicyDigest = buildspec.Hash([]byte("changed policy")) },
		"features": func(id *buildspec.Identity) {
			if len(id.Features) > 1 {
				id.Features = []string{"rt"}
				id.ProviderRevisions = nil
			} else {
				id.Features = []string{"rt", "fs"}
				id.ProviderRevisions = []buildspec.Revision{{Name: "fs", Digest: buildspec.Hash([]byte("filesystem"))}}
			}
		},
	} {
		id := artifact.Manifest.Identity
		change(&id)
		if _, err := buildspec.NormalizeIdentity(id); err != nil {
			t.Fatalf("invalid identity mutation %s: %v", name, err)
		}
		if _, err := gosimtool.VerifyArtifact(artifact.Directory, id, artifact.Manifest.PackageDigests); err == nil {
			t.Fatalf("reused artifact after changing %s", name)
		}
	}
}
