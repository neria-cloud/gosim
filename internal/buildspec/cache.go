package buildspec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func CacheRoot() (string, error) {
	if root := os.Getenv("GOSIMCACHE"); root != "" {
		return filepath.Abs(root)
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "gosim"), nil
}

// LockCache waits cancellably. The lock file stays in place: unlinking it would
// let a new caller lock a different inode while a previous waiter still owns it.
// Kernel locks are released even if a worker dies before calling unlock.
func LockCache(ctx context.Context, path string) (unlock func() error, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, file.Close())
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EINTR) {
			return nil, err
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return func() error { return errors.Join(syscall.Flock(int(file.Fd()), syscall.LOCK_UN), file.Close()) }, nil
}
