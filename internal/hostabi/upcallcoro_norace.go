//go:build go1.25 && !go1.27 && !race

package hostabi

// Upcallcoro is a non-race instrumented version of Upcallcoro in upcallcoro_race.go.
type Upcallcoro struct {
	coro Coro
}

func (u *Upcallcoro) Start(f func()) {
	u.coro.Start(f)
}

func (u *Upcallcoro) Next() {
	u.coro.Next()
}

func (u *Upcallcoro) Upcall(f func()) {
	f()
}

func (u *Upcallcoro) Yield() {
	u.coro.Yield()
}

func (u *Upcallcoro) Finish() {
	u.coro.Finish()
}
