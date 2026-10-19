//go:build sim

package behavior_test

import (
	"os"
	"testing"
)

func TestLogTraceSyscall(t *testing.T) {
	// should print open, write, close
	if err := os.WriteFile("hello", []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
}
