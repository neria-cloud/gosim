//go:build sim

package nemesis_test

import (
	"testing"
	"time"

	"github.com/neria-cloud/gosim/nemesis"
)

func TestSleepScenario(t *testing.T) {
	// Basic test exercising the nemesis Sleep scenario without HTTP.
	// TestPartition (which used net/http) is disabled in nemesis_http_test.go
	// because net/http is skipped since Go 1.25 (crypto/tls/fips140 cascade).
	scenario := nemesis.Sequence(
		nemesis.Sleep{
			Duration: 1 * time.Second,
		},
		nemesis.Sleep{
			Duration: 1 * time.Second,
		},
	)
	scenario.Run()
}
