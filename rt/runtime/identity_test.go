package gosimruntime

import (
	"math"
	"sync/atomic"
	"testing"
)

func TestProcessRunIDExhaustion(t *testing.T) {
	t.Parallel()
	var counter atomic.Uint64
	counter.Store(math.MaxUint64 - 1)
	if id, err := reserveRunID(&counter); err != nil || id != math.MaxUint64 {
		t.Fatal(id, err)
	}
	if id, err := reserveRunID(&counter); err == nil || id != 0 || counter.Load() != math.MaxUint64 {
		t.Fatal("run ID wrapped", id, err)
	}
}
