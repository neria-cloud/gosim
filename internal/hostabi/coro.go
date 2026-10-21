//go:build go1.25

// Package hostabi isolates the selected native Go runtime ABI from the engine.
package hostabi

func (c *Coro) Start(f func()) { c.native.Start(f) }
func (c *Coro) Next()          { c.native.Next() }
func (c *Coro) Yield()         { c.native.Yield() }
func (c *Coro) Finish()        { c.native.Finish() }
