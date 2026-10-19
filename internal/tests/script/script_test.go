package script_test

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/gotooltest"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/hostabi"
)

func TestScript(t *testing.T) {
	if os.Getenv("GOSIM_TEST_LARGE") != "1" {
		t.Skip("run with make test-large")
	}

	p := testscript.Params{
		Dir: "testdata",
	}
	gotooltest.Setup(&p)
	originalCondition := p.Condition
	p.Condition = func(cond string) (bool, error) {
		if cond == "gosimrace" {
			return hostabi.RaceEnabled, nil
		}
		if originalCondition != nil {
			return originalCondition(cond)
		}
		return false, fmt.Errorf("unknown condition %q", cond)
	}

	curModDir, err := gosimtool.FindGoModDir()
	if err != nil {
		t.Fatal(err)
	}
	extractedModDir := filepath.Join(curModDir, gosimtool.OutputDirectory, "scripttest", "mod")

	// add test dependencies to the copied gosim module
	if err := filepath.Walk(extractedModDir, func(path string, info fs.FileInfo, err error) error {
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	origSetup := p.Setup
	p.Setup = func(e *testscript.Env) error {
		// put a fake go.mod in the work directory
		// this include a copy of the gosim module (not the entire module because we want to minimize test dependencies)
		if err := gosimtool.MakeGoModForTest(extractedModDir, e.WorkDir, []string{
			"github.com/dave/dst",
			"github.com/google/go-cmp",
			"github.com/mattn/go-isatty",
			"github.com/mattn/go-sqlite3",
			"golang.org/x/mod",
			"golang.org/x/sync",
			"golang.org/x/sys",
			"golang.org/x/tools",
			"mvdan.cc/gofumpt",
			"google.golang.org/grpc",
		}); err != nil {
			return err
		}

		config := `{"schema":2,"features":["rt","fs","net"],"simulation_os":"linux"}`
		if err := os.WriteFile(filepath.Join(e.WorkDir, "gosim.json"), []byte(config), 0o644); err != nil {
			return err
		}
		e.Setenv("GOSIM_CONFIG", filepath.Join(e.WorkDir, "gosim.json"))
		// add /bin to PATH in tests
		binDir := filepath.Join(curModDir, gosimtool.OutputDirectory, "scripttest", "bin")
		// add dependencies on /bin
		if err := filepath.Walk(binDir, func(path string, info fs.FileInfo, err error) error {
			return nil
		}); err != nil {
			log.Fatal(err)
		}
		e.Setenv("PATH", binDir+string(filepath.ListSeparator)+e.Getenv("PATH"))
		e.Setenv("GOSIMTOOL", "")

		// allow the module to access our shared cache
		e.Vars = append(e.Vars, "GOSIMCACHE="+filepath.Join(curModDir, gosimtool.OutputDirectory))

		if origSetup != nil {
			return origSetup(e)
		}
		return nil
	}
	testscript.Run(t, p)
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, nil))
}
