//go:build go1.26 && !go1.27

// Package maps_shim provides a minimal replacement for internal/runtime/maps
// used when translating hash/maphash. The real internal/runtime/maps is
// internal to the std module and cannot be imported from the translated module.
//
// hash/maphash only uses maps.Use64BitHash from this package.
package maps

// Use64BitHash reports whether the hash function produces 64-bit values.
// This is a constant so the translator does not globalize it.
// The selected amd64 and arm64 targets both use 64-bit hashes.
const Use64BitHash = true
