package buildspec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var workerGOROOT = runtime.GOROOT()

// CheckEnvironment rejects settings that could silently change selected source
// or executable semantics. GOENV is disabled in children so persisted go env -w
// settings cannot override the explicit build contract.
func CheckEnvironment(tc Toolchain) error {
	for _, key := range []string{
		"GOFLAGS", "GOEXPERIMENT", "GODEBUG", "GOCACHEPROG",
		"GOTOOLDIR", "GO_EXTLINK_ENABLED", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS",
	} {
		if os.Getenv(key) != "" {
			return fmt.Errorf("unapproved environment setting %s", key)
		}
	}
	for _, item := range []struct{ key, allowed string }{
		{"GOAMD64", "v1"}, {"GOARM64", "v8.0"}, {"GOOS", tc.HostOS}, {"GOARCH", tc.HostArch},
	} {
		if value := os.Getenv(item.key); value != "" && value != item.allowed {
			return fmt.Errorf("unapproved environment setting %s=%q", item.key, value)
		}
	}
	if root := os.Getenv("GOROOT"); root != "" && root != tc.GOROOT && root != workerGOROOT {
		return fmt.Errorf("unapproved environment setting GOROOT=%q", root)
	}

	return nil
}

// GoCommand is shared with host profile inspection to avoid a spec/buildcfg
// dependency cycle. The public buildcfg.Command delegates to it.
func GoCommand(tc Toolchain, dir string, args ...string) *exec.Cmd {
	return OutputCommand(context.Background(), tc, dir, args...)
}

func OutputCommand(ctx context.Context, tc Toolchain, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, tc.Executable, args...)
	cmd.Dir = dir
	if !filepath.IsAbs(tc.Executable) || !filepath.IsAbs(tc.GOROOT) {
		cmd.Err = fmt.Errorf("toolchain executable and GOROOT must be absolute")
	} else if err := ValidateVersion(tc.Version); err != nil {
		cmd.Err = err
	} else if err := CheckEnvironment(tc); err != nil {
		cmd.Err = err
	} else if err := checkGoFlags(args); err != nil {
		cmd.Err = err
	}
	replaced := map[string]string{
		"PATH":        filepath.Dir(tc.Executable) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GOTOOLCHAIN": "local", "GOROOT": tc.GOROOT, "GOOS": tc.HostOS, "GOARCH": tc.HostArch,
		"GOENV": "off", "GOFLAGS": "", "GOEXPERIMENT": "", "GODEBUG": "", "GOWORK": "off",
		"GOAMD64": "v1", "GOARM64": "v8.0",
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := replaced[key]; !ok {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	// Explicit order, independent of Go map iteration.
	for _, key := range []string{
		"PATH", "GOTOOLCHAIN", "GOROOT", "GOOS", "GOARCH", "GOENV", "GOFLAGS",
		"GOEXPERIMENT", "GODEBUG", "GOWORK", "GOAMD64", "GOARM64",
	} {
		cmd.Env = append(cmd.Env, key+"="+replaced[key])
	}
	return cmd
}

// ApplicationCommand preserves the workspace selected by Go during discovery.
// OutputCommand deliberately isolates generated modules from that workspace.
func ApplicationCommand(ctx context.Context, r Resolution, args ...string) *exec.Cmd {
	cmd := OutputCommand(ctx, r.Toolchain, r.Context.Directory, args...)
	work := r.Context.GoWork
	if work == "" {
		work = "off"
	}
	for i, entry := range cmd.Env {
		if strings.HasPrefix(entry, "GOWORK=") {
			cmd.Env[i] = "GOWORK=" + work
		}
	}
	return cmd
}

func checkGoFlags(args []string) error {
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		switch name {
		case "-gcflags", "-asmflags", "-compiler", "-toolexec", "-overlay", "-modfile", "-pgo", "-asan", "-msan", "-cover", "-covermode", "-coverpkg":
			return fmt.Errorf("unapproved Go build flag %s", name)
		case "-tags", "-ldflags":
			if !hasValue {
				i++
				if i == len(args) {
					return fmt.Errorf("missing value for %s", name)
				}
				value = args[i]
			}
			if name == "-ldflags" {
				if value != "-checklinkname=0" {
					return fmt.Errorf("unapproved linker flags %q", value)
				}
			} else {
				seen := make(map[string]bool)
				for _, tag := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' }) {
					if (tag != "sim" && tag != "linkname") || seen[tag] {
						return fmt.Errorf("unapproved or duplicate build tag %q", tag)
					}
					seen[tag] = true
				}
			}
		}
	}
	return nil
}
