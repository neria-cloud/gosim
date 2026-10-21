package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args string
		code int
		text string
	}{
		{"profiles -root=. -check", 1, "profiles: command not yet implemented"},
		{"compare -parent=go125 -output=new-dir", 1, "compare: command not yet implemented"},
		{"observe -profile=go125 -output=new-file", 1, "observe: command not yet implemented"},
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
