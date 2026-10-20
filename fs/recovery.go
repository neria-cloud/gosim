package fs

import "github.com/neria-cloud/gosim/internal/contracts"

// Snapshot retains an immutable crash outcome until Close or session teardown.
type Snapshot struct{ runID, snapshotID uint64 }

// CrashIterator enumerates retained crash outcomes up to an explicit budget.
// Closing it does not invalidate snapshots already returned by Next.
type (
	CrashIterator    struct{ runID, iteratorID uint64 }
	EnumerateOptions struct{ MaxStates uint64 }
)

var ErrEnumerationLimit = contracts.ErrEnumerationLimit
