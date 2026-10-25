package features

import (
	"fmt"
	"go/build"
	"testing"
)

func TestGS01DirectoryTestSelection(t *testing.T) {
	for _, minor := range []int{25, 26} {
		for _, host := range []string{"linux", "darwin"} {
			for _, sim := range []bool{false, true} {
				t.Run(fmt.Sprintf("go1.%d/%s/sim=%t", minor, host, sim), func(t *testing.T) {
					ctx := build.Default
					ctx.GOOS = host
					ctx.GOARCH = "arm64"
					ctx.BuildTags = []string{"linkname"}
					ctx.ReleaseTags = nil
					for n := 1; n <= minor; n++ {
						ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", n))
					}
					if sim {
						ctx.GOOS = "linux"
						ctx.BuildTags = append(ctx.BuildTags, "sim")
					}
					for name, want := range map[string]bool{
						"directory_descriptors_test.go":     true,
						"directory_linux_test.go":           sim || host == "linux",
						"directory_reference_linux_test.go": !sim && host == "linux",
						"directory_darwin_test.go":          !sim && host == "darwin",
						"directory_sim_test.go":             sim,
						"directory_crash_test.go":           sim,
					} {
						got, err := ctx.MatchFile("../fs", name)
						if err != nil || got != want {
							t.Fatalf("%s: selected=%t want=%t err=%v", name, got, want, err)
						}
					}
				})
			}
		}
	}
}
