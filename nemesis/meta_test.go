//go:build !sim

package nemesis_test

import (
	"testing"

	"github.com/neria-cloud/gosim/metatesting"
)

func TestMetaDeterministic(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	metatesting.CheckDeterministic(t, mt)
}

func TestMetaSeeds(t *testing.T) {
	t.Parallel()
	mt := metatesting.ForPackage(t, metatesting.Options{})
	metatesting.CheckSeeds(t, mt, 5)
}

func TestGosim(t *testing.T) {
	t.Parallel()
	runner := metatesting.ForPackage(t, metatesting.Options{})
	runner.RunAllTests(t)
}
