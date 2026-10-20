//go:build !sim

package fs

import "github.com/neria-cloud/gosim"

func CrashStates(gosim.Machine, EnumerateOptions) (*CrashIterator, error) {
	return nil, gosim.ErrNotSim
}
func (*CrashIterator) Next() (Snapshot, bool, error) { return Snapshot{}, false, gosim.ErrNotSim }
func (*CrashIterator) Close() error                  { return gosim.ErrNotSim }
func Restore(gosim.Machine, Snapshot) error          { return gosim.ErrNotSim }
func (Snapshot) Close() error                        { return gosim.ErrNotSim }
