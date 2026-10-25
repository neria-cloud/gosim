package model

import (
	"strings"
	"syscall"
)

func (fs *Filesystem) Symlink(dir int, path, target string, who Identity) error {
	if target == "" {
		return syscall.ENOENT
	}
	if strings.IndexByte(target, 0) >= 0 {
		return syscall.EINVAL
	}
	if len(target) >= 4096 {
		return syscall.ENAMETOOLONG
	}
	entry, err := fs.resolve(dir, path, resolveOptions{Final: resolveCreateEntry, AllowMissingFinal: true}, who)
	if err != nil {
		return err
	}
	if entry.Inode != 0 {
		return syscall.EEXIST
	}
	if entry.TrailingSlash {
		return syscall.ENOENT
	}
	if err := fs.mutableParent(entry, who); err != nil {
		return err
	}
	inode := fs.mem.next
	fs.apply(&allocOp{inode: inode, symlink: true, target: target, meta: inodeMetadata{Mode: 0o777, UID: who.UID, GID: who.GID}})
	fs.apply(&dirOp{parts: []dirOpPart{{dir: entry.Parent, name: entry.Name, file: inode}}})
	return nil
}

func (fs *Filesystem) Readlink(dir int, path string, who Identity) (string, error) {
	entry, err := fs.resolve(dir, path, resolveOptions{Final: resolveNoFollow}, who)
	if err != nil {
		return "", err
	}
	link, ok := fs.mem.objects[entry.Inode].(*backingSymlink)
	if !ok {
		return "", syscall.EINVAL
	}
	return link.target, nil
}
