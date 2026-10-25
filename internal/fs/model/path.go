package model

import (
	"strings"
	"syscall"
)

// resolveOptions describes the final entry only. Intermediate symlinks always
// follow normal Linux pathname semantics; os.Root imposes its own confinement.
type resolveOptions struct {
	followFinal  bool
	allowMissing bool
	mutation     bool
}

type resolvedPath struct {
	parent   int
	name     string
	inode    int
	trailing bool
}

func (fs *Filesystem) resolve(base int, path string, options resolveOptions, who Identity) (resolvedPath, error) {
	fail := func(err error) (resolvedPath, error) { return resolvedPath{}, err }
	if path == "" {
		return fail(syscall.ENOENT)
	}
	if strings.IndexByte(path, 0) >= 0 {
		return fail(syscall.EINVAL)
	}
	if len(path) >= 4096 {
		return fail(syscall.ENAMETOOLONG)
	}
	if path[0] == '/' {
		base = RootInode
	}
	links := 0
	for {
		path = strings.TrimLeft(path, "/")
		if path == "" {
			path = "."
		}
		name, rest, slash := strings.Cut(path, "/")
		terminal := !slash || strings.Trim(rest, "/") == ""
		trailing := terminal && slash
		if len(name) > 255 {
			return fail(syscall.ENAMETOOLONG)
		}
		dir, ok := fs.mem.getDir(base)
		if !ok {
			return fail(syscall.ENOTDIR)
		}
		if err := fs.CheckAccess(base, who, AccessExecute); err != nil {
			return fail(err)
		}
		var inode int
		switch name {
		case ".":
			inode = base
		case "..":
			inode = dir.parent
		default:
			inode = dir.entries[name]
		}
		entry := resolvedPath{parent: base, name: name, inode: inode, trailing: trailing}
		if inode == 0 {
			if terminal && options.allowMissing {
				return entry, nil
			}
			return fail(syscall.ENOENT)
		}
		link, isLink := fs.mem.objects[inode].(*backingSymlink)
		if isLink && (!terminal || options.followFinal || trailing && !options.mutation) {
			links++
			if links > 40 {
				return fail(syscall.ELOOP)
			}
			path = link.target
			if slash {
				path += "/" + rest
			}
			if len(path) >= 4096 {
				return fail(syscall.ENAMETOOLONG)
			}
			if len(path) == 0 {
				return fail(syscall.ENOENT)
			}
			if path[0] == '/' {
				base = RootInode
			}
			continue
		}
		if terminal {
			if trailing {
				if _, ok := fs.mem.getDir(inode); !ok {
					return fail(syscall.ENOTDIR)
				}
			}
			return entry, nil
		}
		base, path = inode, rest
	}
}

func (fs *Filesystem) mutableParent(entry resolvedPath, who Identity) error {
	dir, ok := fs.mem.getDir(entry.parent)
	if !ok {
		return syscall.ENOTDIR
	}
	if dir.unlinked {
		return syscall.ENOENT
	}
	return fs.CheckAccess(entry.parent, who, AccessWrite|AccessExecute)
}

func (fs *Filesystem) Lstat(dir int, path string, who Identity) (StatResp, error) {
	entry, err := fs.resolve(dir, path, resolveOptions{}, who)
	if err != nil {
		return StatResp{}, err
	}
	return fs.statLocked(entry.inode)
}

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
	entry, err := fs.resolve(dir, path, resolveOptions{allowMissing: true, mutation: true}, who)
	if err != nil {
		return err
	}
	if entry.inode != 0 {
		return syscall.EEXIST
	}
	if entry.trailing {
		return syscall.ENOENT
	}
	if err := fs.mutableParent(entry, who); err != nil {
		return err
	}
	inode := fs.mem.next
	fs.apply(&allocOp{inode: inode, symlink: true, target: target, meta: inodeMetadata{Mode: 0o777, UID: who.UID, GID: who.GID}})
	fs.apply(&dirOp{parts: []dirOpPart{{dir: entry.parent, name: entry.name, file: inode}}})
	return nil
}

func (fs *Filesystem) Readlink(dir int, path string, who Identity) (string, error) {
	entry, err := fs.resolve(dir, path, resolveOptions{}, who)
	if err != nil {
		return "", err
	}
	link, ok := fs.mem.objects[entry.inode].(*backingSymlink)
	if !ok {
		return "", syscall.EINVAL
	}
	return link.target, nil
}
