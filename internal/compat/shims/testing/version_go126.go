//go:build go1.26

package testing

import "os"

type tbExtension interface{ ArtifactDir() string }

func testTempRoot() string { return os.Getenv("GOTMPDIR") }

// ArtifactDir implements Go 1.26's default temporary artifact directory mode.
// The simulation runner does not accept -test.artifacts persistence; files live
// in the simulated filesystem and follow normal test cleanup.
func (c *common) ArtifactDir() string {
	c.artifactDirOnce.Do(func() { c.artifactDir = c.TempDir() })
	return c.artifactDir
}
