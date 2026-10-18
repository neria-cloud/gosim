// Package synctest is a gosim stub for Go 1.25's internal/synctest package.
// The real internal/synctest is a stdlib internal package that cannot be
// imported outside the standard library. Gosim redirects imports of
// "internal/synctest" to this stub, which provides no-op implementations.
//
// In the simulation there are no synctest bubbles, so IsInBubble always
// returns false, and all other bubble-related functions are no-ops.
package synctest

func Run(f func()) { f() }

func Wait() {}

func IsInBubble() bool { return false }

// Association is the state of a pointer's bubble association.
type Association int

const (
	Unbubbled     = Association(0)
	CurrentBubble = Association(1)
	OtherBubble   = Association(2)
)

func Associate[T any](p *T) Association {
	_ = p
	return Unbubbled
}

func Disassociate[T any](p *T) {}

func IsAssociated[T any](p *T) bool { return false }

// A Bubble is a synctest bubble — always nil in gosim.
type Bubble struct{}

func Acquire() *Bubble { return nil }

func (b *Bubble) Release() {}

func (b *Bubble) Run(f func()) { f() }
