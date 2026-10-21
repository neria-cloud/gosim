package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/compat/audit"
)

func TestDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args string
		code int
		text string
	}{
		{"profiles -root=/nonexistent-gosim-profile-fixture -check", 1, "no such file"},
		{"compare -parent=go125 -output=new-dir -arches=mips", 2, "invalid or duplicate architecture"},
		{"observe -profile=go125 -output=new-file -toolchain=go1.99.0", 2, "must equal running compiler"},
		{"profiles -root=", 2, "nonempty -root"},
		{"compare -parent=go125", 2, "requires -parent and -output"},
		{"observe -output=new-file", 2, "requires -profile and -output"},
		{"-profile=go125 -arches=amd64 -output=. -check -refresh-origin", 2, "cannot be combined"},
		{"-profile=go125 -arches=amd64", 2, "requires -profile"},
		{"unknown", 2, "unexpected positional"},
		{"profiles extra", 2, "unexpected positional"},
		{"profiles -bogus", 2, "flag provided but not defined"},
	}
	for _, test := range cases {
		t.Run(test.args, func(t *testing.T) {
			t.Parallel()
			runner := commandRunner{generate: func(commandOptions) error {
				t.Fatal("dispatch unexpectedly reached generation/compiler discovery")
				return nil
			}}
			var stdout, stderr bytes.Buffer
			code := runner.dispatch(context.Background(), strings.Fields(test.args), &stdout, &stderr)
			if code != test.code || !strings.Contains(stderr.String(), test.text) || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestGenerationArguments(t *testing.T) {
	t.Parallel()
	want := commandOptions{
		toolchain: "go1.25.14", profile: "go125", arches: "arm64,amd64", output: "/output", check: true,
	}
	var got commandOptions
	runner := commandRunner{generate: func(options commandOptions) error {
		got = options
		return errors.New("generation failed")
	}}
	var stdout, stderr bytes.Buffer
	args := []string{"-toolchain=go1.25.14", "-profile=go125", "-arches=arm64,amd64", "-output=/output", "-check"}
	code := runner.dispatch(context.Background(), args, &stdout, &stderr)
	if code != 1 || !reflect.DeepEqual(got, want) || stderr.String() != "generation failed\n" {
		t.Fatalf("code=%d options=%+v stderr=%q", code, got, stderr.String())
	}
	runner.generate = func(options commandOptions) error {
		if !options.refresh || options.check {
			t.Fatalf("refresh arguments lost: %+v", options)
		}
		return nil
	}
	args[len(args)-1] = "-refresh-origin"
	if code := runner.dispatch(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("successful generation returned %d", code)
	}
}

func TestCommandHelp(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"", "profiles", "observe", "compare"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := runCLI(context.Background(), strings.Fields(command+" -help"), &stdout, &stderr)
			if code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestProfilesMaintenanceBootstrap(t *testing.T) {
	const directoryMode = 0o755
	const fileMode = 0o644
	const definition = `{"schema":1,"family":"1.25","supported":true,"delta":{}}`
	root := t.TempDir()
	directory := filepath.Join(root, "internal/compat/profiles")
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go125.json"), []byte(definition), fileMode); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runCLI(t.Context(), []string{"profiles", "-root=" + root}, &stdout, &stderr); code != 0 {
		t.Fatalf("bootstrap %d: %s", code, &stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/buildspec/families_gen.go")); err != nil {
		t.Fatal(err)
	}
	code := runCLI(t.Context(), []string{"profiles", "-root=" + root, "-check"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "origin.json") {
		t.Fatalf("readiness %d: %s", code, &stderr)
	}
}

func TestComparisonExitAndCancellation(t *testing.T) {
	for _, complete := range []bool{true, false} {
		dir := filepath.Join(t.TempDir(), "report")
		report := audit.ComparisonReport{
			Schema: 1, Complete: complete, APIDiff: "skipped-no-parent",
			Findings: []audit.Finding{{Kind: "signature", Severity: "incompatible"}},
		}
		draft := audit.DraftDelta{Schema: 1, Kind: "gosim-profile-draft", Complete: complete}
		var stdout, stderr bytes.Buffer
		code := finishComparison(t.Context(), comparisonResult{dir, report, draft, nil}, &stdout, &stderr)
		want := 1
		if complete {
			want = 0
		}
		if code != want {
			t.Fatal(code, stdout.String(), stderr.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "report.json")); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, command := range []string{"observe", "compare"} {
		var stdout, stderr bytes.Buffer
		output := filepath.Join(t.TempDir(), "cancelled")
		familyFlag := "-parent=go125"
		if command == "observe" {
			familyFlag = "-profile=go125"
		}
		if code := runCLI(ctx, []string{command, familyFlag, "-output=" + output}, &stdout, &stderr); code != 1 {
			t.Fatal(code)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatal("cancelled publication", err)
		}
	}
}

func TestGeneratorMetadataOnlyClosure(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), filepath.Join(runtime.GOROOT(), "bin", "go"),
		"list", "-deps", "-tags=linkname", ".")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	const module = "github.com/neria-cloud/gosim/"
	for _, pkg := range strings.Fields(string(output)) {
		for _, forbidden := range []string{
			"internal/compat/selected", "internal/compat/host", "internal/compat/hooks",
			"internal/compat/shims", "internal/hostabi", "rt/runtime",
		} {
			if pkg == module+forbidden || strings.HasPrefix(pkg, module+forbidden+"/") {
				t.Fatal("executable compatibility import", pkg)
			}
		}
	}
}

func TestAuditDestinationPreflight(t *testing.T) {
	const fileMode = 0o600
	const observeCommand = "observe"
	output := filepath.Join(t.TempDir(), "nested", "output")
	if err := audit.PrepareOutput(output); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("owned by caller"), fileMode); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{observeCommand, "compare"} {
		var stdout, stderr bytes.Buffer
		family := "-parent=go125"
		if command == observeCommand {
			family = "-profile=go125"
		}
		code := runCLI(t.Context(), []string{command, family, "-output=" + output}, &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "already exists") {
			t.Fatal(code, stdout.String(), stderr.String())
		}
	}
}
