package routegen

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/spec"
)

const adapterModule = "github.com/neria-cloud/gosim"

func adapterPackages(hooks []spec.Hook) (map[string]string, error) {
	result := map[string]string{}
	for _, h := range hooks {
		start := strings.LastIndex(h.Target, "/") + 1
		dot := strings.IndexByte(h.Target[start:], '.')
		if dot < 1 {
			return nil, fmt.Errorf("invalid adapter target %q", h.Target)
		}
		pkg, symbol := h.Target[:start+dot], h.Target[start+dot+1:]
		if spec.ClassifyCompatPackage(pkg) != spec.CompatHooks || symbol != adapterPrefix(h.Package)+h.Symbol {
			return nil, fmt.Errorf("unreviewed adapter destination %s", h.Target)
		}
		if old := result[h.Owner]; old != "" && old != pkg {
			return nil, fmt.Errorf("ambiguous adapter package for %s", h.Owner)
		}
		result[h.Owner] = pkg
	}
	for _, owner := range []string{"kernel", "fs", "net"} {
		if result[owner] == "" {
			return nil, fmt.Errorf("missing adapter package for %s", owner)
		}
	}
	return result, nil
}

func selectedGeneratedPath(root, packagePath, stem string, source spec.SourceContext, family string) (string, error) {
	rel, ok := strings.CutPrefix(packagePath, adapterModule+"/")
	if !ok || spec.ClassifyCompatPackage(packagePath) != spec.CompatHooks {
		return "", fmt.Errorf("invalid generated package %s", packagePath)
	}
	version, err := buildspec.ParseGoVersion("go" + family + ".0")
	if err != nil {
		return "", err
	}
	ctx := build.Context{GOOS: source.OS, GOARCH: source.Arch, Compiler: "gc", BuildTags: strings.Split(buildspec.TranslatedSourceTags(source.Race), ",")}
	for minor := 1; minor <= version.Minor; minor++ {
		ctx.ReleaseTags = append(ctx.ReleaseTags, fmt.Sprintf("go1.%d", minor))
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(absolute, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	selected := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name != stem+".go" && !strings.HasPrefix(name, stem+"_") {
			continue
		}
		matches, err := ctx.MatchFile(dir, name)
		if err != nil {
			return "", err
		}
		if !matches {
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("multiple selected %s destinations in %s", stem, packagePath)
		}
		selected = filepath.ToSlash(filepath.Join(rel, name))
	}
	if selected == "" {
		return "", fmt.Errorf("no selected %s destination in %s for %s/%s", stem, packagePath, family, source.Arch)
	}
	return selected, nil
}

func authoredConstraint(root, relative string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "//go:build ") {
			return line + "\n", nil
		}
	}
	return "", nil
}
