package model

import (
	"strings"
	"syscall"
)

type finalPolicy uint8

const (
	resolveFollowing finalPolicy = iota
	resolveNoFollow
	resolveCreateEntry
	resolveMutationEntry
)

type resolveOptions struct {
	Final             finalPolicy
	AllowMissingFinal bool
}

type resolvedPath struct {
	Parent        int
	Name          string // empty only for slash-only root; empty input is rejected
	Inode         int
	TrailingSlash bool
}

// resolve implements Linux component lookup. Raw *at calls are not confined:
// os.Root's translated traversal imposes that stronger contract itself.
func (fs *Filesystem) resolve(base int, path string, options resolveOptions, who Identity) (resolvedPath, error) {
	links := 0
	return fs.resolveWithLinks(base, path, options, who, &links)
}

// Open's exclusive/existence checks may precede following its final link.
// Keep the same counter when continuing that lookup: prefix and final links
// share Linux's limit, rather than each getting an independent allowance.
func (fs *Filesystem) resolveWithLinks(base int, path string, options resolveOptions, who Identity, links *int) (resolvedPath, error) {
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
	for {
		path = strings.TrimLeft(path, "/")
		if path == "" {
			return resolvedPath{Parent: base, Inode: base, TrailingSlash: true}, nil
		}
		name, rest, slash := strings.Cut(path, "/")
		terminal := !slash || strings.Trim(rest, "/") == ""
		trailing := terminal && slash
		dir, ok := fs.mem.getDir(base)
		if !ok {
			return fail(syscall.ENOTDIR)
		}
		if err := fs.CheckAccess(base, who, AccessExecute); err != nil {
			return fail(err)
		}
		if len(name) > 255 {
			return fail(syscall.ENAMETOOLONG)
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
		entry := resolvedPath{Parent: base, Name: name, Inode: inode, TrailingSlash: trailing}
		if inode == 0 {
			if terminal && options.AllowMissingFinal {
				return entry, nil
			}
			return fail(syscall.ENOENT)
		}
		follow := !terminal || options.Final == resolveFollowing || trailing && options.Final == resolveNoFollow
		if link, ok := fs.mem.objects[inode].(*backingSymlink); ok && follow {
			*links++
			if *links > 40 {
				return fail(syscall.ELOOP)
			}
			path = link.target
			if slash {
				path += "/" + rest
			}
			if len(path) >= 4096 {
				return fail(syscall.ENAMETOOLONG)
			}
			if path == "" {
				return fail(syscall.ENOENT)
			}
			if path[0] == '/' {
				base = RootInode
			}
			continue
		}
		if terminal {
			// Namespace callers must apply their own existence/type/permission order.
			// In particular, mkdir(existing-file/) is EEXIST, not ENOTDIR.
			if trailing && (options.Final == resolveFollowing || options.Final == resolveNoFollow) {
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
	dir, ok := fs.mem.getDir(entry.Parent)
	if !ok {
		return syscall.ENOTDIR
	}
	if dir.unlinked {
		return syscall.ENOENT
	}
	return fs.CheckAccess(entry.Parent, who, AccessWrite|AccessExecute)
}

func (fs *Filesystem) Lstat(dir int, path string, who Identity) (StatResp, error) {
	entry, err := fs.resolve(dir, path, resolveOptions{Final: resolveNoFollow}, who)
	if err != nil {
		return StatResp{}, err
	}
	return fs.statLocked(entry.Inode)
}
