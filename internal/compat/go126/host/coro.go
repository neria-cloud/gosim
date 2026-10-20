//go:build go1.26 && !go1.27

package host

import (
	"unsafe"
)

type coro struct{}

//go:linkname newcoro runtime.newcoro
func newcoro(func(*coro)) *coro

//go:linkname coroswitch runtime.coroswitch
func coroswitch(*coro)

//go:linkname coroexit runtime.coroexit
func coroexit(*coro)

// A Coro is a coroutine: A goroutine with explicit and cheap context switching.
// Coro uses go's runtime coroutine implementation (which otherwise powers
// iter.Pull) using linkname to get low overhead and direct access to coroexit.
type Coro struct {
	coro *coro
}

// Start starts the coroutine, running f in a new coroutine. It should be called only
// once on a Coro. It lets the coroutine run until the first call to Yield or the final
// call to Finish.
//
//go:norace
func (c *Coro) Start(f func()) {
	c.coro = newcoro(func(*coro) {
		RaceAcquire(unsafe.Pointer(c))
		f()
		panic("hostabi: coroutine returned without Finish")
	})
	RaceRelease(unsafe.Pointer(c))
	coroswitch(c.coro)
	RaceAcquire(unsafe.Pointer(c))
}

// Next must be called from outside the coroutine. It lets the coroutine run
// until the next call to Yield or the final call to Finish.
//
//go:norace
func (c *Coro) Next() {
	RaceRelease(unsafe.Pointer(c))
	coroswitch(c.coro)
	RaceAcquire(unsafe.Pointer(c))
}

// Yield be called form inside the coroutine. It pauses the coroutine, yielding to
// the caller of Next (or Start).
//
//go:norace
func (c *Coro) Yield() {
	RaceRelease(unsafe.Pointer(c))
	coroswitch(c.coro)
	RaceAcquire(unsafe.Pointer(c))
}

// Finish must be called from inside the coroutine. It stops the coroutine,
// yielding to the caller of Next (or Start). Finish terminates the coroutine,
// any defers in the callstack will _not_ be run.
//
//go:norace
func (c *Coro) Finish() {
	RaceRelease(unsafe.Pointer(c))
	coroexit(c.coro)
	panic("hostabi: coroutine returned without Finish")
}
