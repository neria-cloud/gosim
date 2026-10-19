//go:build go1.26 && !go1.27

package testing

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
)

func (c *common) Context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		c.ctx, c.cancelCtx = context.WithCancel(context.Background())
	}
	return c.ctx
}

func (c *common) checkParallel() {
	for parent := c; parent != nil; parent = parent.parent {
		if parent.isParallel {
			panic("testing: Setenv or Chdir called in parallel test")
		}
	}
	c.denyParallel = true
}

func (c *common) setenv(key, value string) {
	c.Helper()
	c.checkParallel()
	old, exists := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		c.Fatalf("Setenv: %v", err)
	}
	c.Cleanup(func() {
		var err error
		if exists {
			err = os.Setenv(key, old)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			c.Errorf("Setenv cleanup: %v", err)
		}
	})
}

func (c *common) Chdir(dir string) {
	c.Helper()
	c.checkParallel()
	old, err := os.Getwd()
	if err != nil {
		c.Fatalf("Chdir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		c.Fatalf("Chdir: %v", err)
	}
	// All os calls are translated: cwd and environment stay machine-local and
	// filesystem calls reach the guarded fs routes. There is no native os import.
	cwd, err := os.Getwd()
	if err != nil {
		c.Fatalf("Chdir: %v", err)
	}
	c.setenv("PWD", cwd)
	c.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			c.Errorf("Chdir cleanup: %v", err)
		}
	})
}

func (c *common) Attr(key, value string) {
	if strings.ContainsFunc(key, unicode.IsSpace) {
		c.Errorf("disallowed whitespace in attribute key %q", key)
		return
	}
	if strings.ContainsAny(value, "\r\n") {
		c.Errorf("disallowed newline in attribute value %q", value)
		return
	}
	if c.chatty != nil {
		c.chatty.Updatef(c.name, "=== ATTR  %s %v %v\n", c.name, key, value)
	}
}

type outputWriter struct {
	mu      sync.Mutex
	owner   *common
	pending string
}

func (c *common) Output() io.Writer {
	w := &outputWriter{owner: c}
	c.Cleanup(w.flush)
	return w
}

func (w *outputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending += string(p)
	for {
		line, rest, ok := strings.Cut(w.pending, "\n")
		if !ok {
			break
		}
		w.pending = rest
		w.owner.Log(line)
	}
	return len(p), nil
}

func (w *outputWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != "" {
		w.owner.Log(w.pending)
		w.pending = ""
	}
}

// ArtifactDir implements Go 1.26's default temporary artifact directory mode.
// The simulation runner does not accept -test.artifacts persistence; files live
// in the simulated filesystem and follow normal test cleanup.
func (c *common) ArtifactDir() string {
	c.artifactDirOnce.Do(func() { c.artifactDir = c.TempDir() })
	return c.artifactDir
}
