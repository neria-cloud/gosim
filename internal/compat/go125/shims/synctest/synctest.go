//go:build go1.25 && !go1.26

// Package synctest models internal standard-library association probes outside
// bubbles. Creating or waiting for a bubble is explicitly unsupported.
package synctest

// TODO(not planned): native synctest bubbles cannot share the simulation scheduler.
func Run(_ func()) { panic("gosim go125 synctest.Run: bubbles are unsupported") }

func Wait() { panic("gosim go125 synctest.Wait: bubbles are unsupported") }

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

func Disassociate[T any](_ *T) {}

func IsAssociated[T any](_ *T) bool { return false }

// A Bubble is a synctest bubble — always nil in gosim.
type Bubble struct{}

func Acquire() *Bubble { return nil }

func (b *Bubble) Release() {
	if b != nil {
		panic("gosim go125 synctest: non-nil bubble is unsupported")
	}
}

func (b *Bubble) Run(f func()) {
	if b != nil {
		panic("gosim go125 synctest: non-nil bubble is unsupported")
	}
	f()
}
