//go:build go1.25

// Package synctest models internal standard-library association probes outside
// bubbles. Creating or waiting for a bubble is explicitly unsupported.
package synctest

// TODO(not planned): native synctest bubbles cannot share the simulation scheduler.
func Run(f func()) { panic("gosim synctest.Run: bubbles are unsupported") }

func Wait() { panic("gosim synctest.Wait: bubbles are unsupported") }

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

func (b *Bubble) Release() {
	if b != nil {
		panic("gosim synctest: non-nil bubble is unsupported")
	}
}

func (b *Bubble) Run(f func()) {
	if b != nil {
		panic("gosim synctest: non-nil bubble is unsupported")
	}
	f()
}
