//go:build sim

package nemesis_test

import (
	"testing"
	"time"

	"github.com/neria-cloud/gosim/nemesis"
)

func TestSleepScenario(t *testing.T) {
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
