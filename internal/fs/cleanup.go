package fs

import (
	"errors"
	"fmt"
)

// Cleanup must continue even after a provider invariant panic. Report the
// accumulated failure only after releasing the remaining owned resources.
type cleanupErrors struct{ failures []error }

func (c *cleanupErrors) run(fn func() error) {
	defer func() {
		if value := recover(); value != nil {
			c.failures = append(c.failures, fmt.Errorf("filesystem cleanup invariant: %v", value))
		}
	}()
	if err := fn(); err != nil {
		c.failures = append(c.failures, err)
	}
}

func (c *cleanupErrors) err() error { return errors.Join(c.failures...) }

func (c *cleanupErrors) finish() {
	if err := c.err(); err != nil {
		panic(err)
	}
}
