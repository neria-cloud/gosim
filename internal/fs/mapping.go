package fs

import "github.com/neria-cloud/gosim/internal/fs/model"

type fileMapping struct {
	model *model.Mmap
	owner *File
}

// Index maintenance belongs to Unmap. Bulk detach walks the index once rather
// than rescanning mappingOrder for every resource it releases.
func (i *Instance) releaseMappingResources(m *fileMapping) error {
	if m.owner.instance != i || m.owner.mappingRefs <= 0 {
		panic("invalid filesystem mapping ownership")
	}
	var cleanup cleanupErrors
	cleanup.run(func() error { i.model.Munmap(m.model); return nil })
	m.owner.mappingRefs--
	cleanup.run(m.owner.releaseIfUnreferenced)
	return cleanup.err()
}
