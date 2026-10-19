// Package compose renders translated composition without importing providers or
// executable adapters into the host generator.
package compose

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/compat/selected"
	"github.com/neria-cloud/gosim/internal/gosimtool"
)

type GeneratedComposition struct {
	SourceDir, IdentityDigest string
	Files                     []string
}

type (
	sourceImport    struct{ Alias, Path string }
	compositionData struct {
		Imports                                                                                          []sourceImport
		Constructors                                                                                     []string
		Features, IdentityDigest, Toolchain, Profile, HostOS, HostArch, AdapterRevision, RuntimeRevision string
		Certified                                                                                        bool
	}
)

// PackagePath is the reserved original-source import path used by the generated
// input module. The loader materializes it before loading any application source.
func PackagePath(identityDigest string) (string, error) {
	if !buildspec.ValidDigest(identityDigest) {
		return "", fmt.Errorf("invalid composition identity digest")
	}
	return gosimtool.Module + "/.gosim/v2/" + identityDigest + "/input/gosimcomposition", nil
}

func render(selection buildspec.Selection) ([]byte, []string, error) {
	if _, _, err := selected.Resolve(selection); err != nil {
		return nil, nil, err
	}
	id := selection.Identity
	digest, err := buildspec.Digest(id)
	if err != nil {
		return nil, nil, err
	}
	data := compositionData{IdentityDigest: digest, Toolchain: id.Toolchain, Profile: id.Profile, HostOS: id.HostOS, HostArch: id.HostArch, AdapterRevision: id.AdapterRevision, RuntimeRevision: id.RuntimeRevision, Certified: id.Certification == "certified", Features: "contracts.RT"}
	imports := map[string]string{
		gosimtool.Module + "/internal/contracts": "contracts", gosimtool.Module + "/internal/control": "control",
		gosimtool.Module + "/internal/kernel": "kernel", gosimtool.Module + "/rt/runtime": "simrt", gosimtool.Module + "/rt/runtime/abi": "abi",
		gosimtool.Module + "/internal/compat/" + id.Profile + "/shims/testing": "simtesting",
		gosimtool.Module + "/internal/compat/" + id.Profile + "/hooks/kernel":  "processhooks",
	}
	for _, path := range selection.RuntimePackages {
		if _, ok := imports[path]; !ok {
			imports[path] = "_"
		}
	}
	for _, feature := range []string{"fs", "net"} {
		if !slices.Contains(id.Features, feature) {
			continue
		}
		name := "FS"
		if feature == "net" {
			name = "Net"
		}
		alias := feature + "provider"
		imports[gosimtool.Module+"/internal/"+feature] = alias
		data.Constructors = append(data.Constructors, alias+".NewProvider()")
		data.Features += " | contracts." + name
	}
	var paths []string
	for path := range imports {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		data.Imports = append(data.Imports, sourceImport{imports[path], path})
	}
	var output bytes.Buffer
	if err := driverTemplate.Execute(&output, data); err != nil {
		return nil, nil, err
	}
	source, err := format.Source(output.Bytes())
	return source, paths, err
}

// Generate writes only into a caller-owned staging directory. Publication of its
// one source file is atomic and exclusive; identical concurrent generations can
// reuse it, while a different source is never overwritten.
func Generate(selection buildspec.Selection, dir string) (GeneratedComposition, error) {
	source, _, err := render(selection)
	if err != nil {
		return GeneratedComposition{}, err
	}
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return GeneratedComposition{}, fmt.Errorf("composition requires an absolute clean staging directory")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return GeneratedComposition{}, err
	}
	if !info.IsDir() {
		return GeneratedComposition{}, fmt.Errorf("composition staging root is not a real directory")
	}
	digest, _ := buildspec.Digest(selection.Identity)
	output := GeneratedComposition{SourceDir: dir, IdentityDigest: digest, Files: []string{"composition.go"}}
	path := filepath.Join(dir, "composition.go")
	if _, err := os.Lstat(path); err == nil {
		return output, ValidateImports(output, selection)
	} else if !errors.Is(err, os.ErrNotExist) {
		return GeneratedComposition{}, err
	}
	temp, err := os.CreateTemp(dir, ".composition-*")
	if err != nil {
		return GeneratedComposition{}, err
	}
	defer os.Remove(temp.Name())
	writeErr := temp.Chmod(0o644)
	if writeErr == nil {
		_, writeErr = temp.Write(source)
	}
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return GeneratedComposition{}, err
	}
	if err := os.Link(temp.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return GeneratedComposition{}, err
	}
	if err := ValidateImports(output, selection); err != nil {
		return GeneratedComposition{}, err
	}
	return output, nil
}

// ValidateImports verifies the recorded source and exact direct import set.
// Transitive original/translated graphs are checked by the candidate build;
// reading a template alone cannot establish provider isolation after rewriting.
func ValidateImports(output GeneratedComposition, selection buildspec.Selection) error {
	source, expectedImports, err := render(selection)
	if err != nil {
		return err
	}
	digest, _ := buildspec.Digest(selection.Identity)
	if output.IdentityDigest != digest || !slices.Equal(output.Files, []string{"composition.go"}) || !filepath.IsAbs(output.SourceDir) || filepath.Clean(output.SourceDir) != output.SourceDir {
		return fmt.Errorf("invalid generated composition record")
	}
	info, err := os.Lstat(output.SourceDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("composition root is not a real directory")
	}
	entries, err := os.ReadDir(output.SourceDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".go") && entry.Name() != "composition.go" {
			return fmt.Errorf("unrecorded composition source %s", entry.Name())
		}
	}
	path := filepath.Join(output.SourceDir, "composition.go")
	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("composition source is not a regular file")
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, actual, parser.ImportsOnly)
	if err != nil {
		return err
	}
	var imports []string
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		imports = append(imports, path)
	}
	slices.Sort(imports)
	if !slices.Equal(imports, expectedImports) {
		return fmt.Errorf("generated composition import set differs from selection")
	}
	if !bytes.Equal(actual, source) {
		return fmt.Errorf("generated composition source differs from selected driver")
	}
	return nil
}

var driverTemplate = template.Must(template.New("driver").Parse(`// Code generated by gosim composition; DO NOT EDIT.
package gosimcomposition

import (
{{range .Imports}} {{.Alias}} {{printf "%q" .Path}}
{{end}})

const IdentityDigest = {{printf "%q" .IdentityDigest}}
const Toolchain = {{printf "%q" .Toolchain}}
const timeoutNS int64 = 600 * 1e9

type Driver struct{}
var _ simrt.Driver = Driver{}

func (Driver) TestEntrypoint(match, skip string, tests []simrt.Test) bool {
 return simtesting.Entrypoint(match, skip, tests)
}

func (Driver) Run(config simrt.RunConfig, main func()) simrt.RunResult {
 expected := abi.HostIdentity{Toolchain:Toolchain, Profile:{{printf "%q" .Profile}}, HostOS:{{printf "%q" .HostOS}}, HostArch:{{printf "%q" .HostArch}}, SourceDigest:{{printf "%q" .AdapterRevision}}}
 if simrt.HostIdentity()!=expected {
  return simrt.RunResult{Seed:config.Seed,Failed:true,Err:"native runtime identity differs from generated composition",BuildDigest:IdentityDigest,ReplayRevision:simrt.ReplayRevision}
 }
 engine, err := simrt.NewEngine(simrt.EngineConfig{Seed:config.Seed,TimeoutNS:timeoutNS,ReplayRevision:simrt.ReplayRevision})
 if err!=nil {return simrt.RunResult{Seed:config.Seed,Failed:true,Err:err.Error(),BuildDigest:IdentityDigest,ReplayRevision:simrt.ReplayRevision}}
 var failure string
 record:=func(err error){if err!=nil {if failure!="" {failure+="; "};failure+=err.Error()}}
 if err:=engine.ConfigureRun(config);err!=nil {record(err);engine.Close();return engine.RunResult(IdentityDigest,failure)}
 if err:=engine.SetMachineInitializer(func(){
  if err:=processhooks.ConfigureProcess([]string{"gosimapp"},simrt.Envs());err!=nil {panic(err)}
  simrt.InitGlobals(false,false)
 });err!=nil {record(err);engine.Close();return engine.RunResult(IdentityDigest,failure)}
 runErr:=engine.Run(func(){
  providers:=[]contracts.Provider{ {{range .Constructors}}{{.}},{{end}} }
  k,err:=kernel.New(kernel.Config{Identity:expected,RuntimeRevision:{{printf "%q" .RuntimeRevision}},BuildDigest:IdentityDigest,Certified:{{.Certified}},Features:contracts.FeatureSet({{.Features}}),TimeoutNS:timeoutNS},engine,providers)
  if err!=nil {record(err);return}
  control.Bind(k)
  defer func(){record(k.Close());control.Bind(nil)}()
  record(k.Run(main))
 })
 record(runErr)
 if closeErr:=engine.Close();runErr==nil {record(closeErr)}
 return engine.RunResult(IdentityDigest,failure)
}
`))
