package translate

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/gosimtool"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

const translationNamespace = "gosim-translation-v2"

type Hash [32]byte

type CacheEntryV2 struct {
	Format         uint32
	IdentityDigest string
	PackageDigest  string
	Result         TranslatePackageResult
}

// Each field, including fixed-size hashes, has an explicit length. Section
// labels and counts distinguish source lists from dependencies and metadata.
type cacheHasher struct{ hash.Hash }

func newCacheHasher() *cacheHasher { return &cacheHasher{sha256.New()} }
func (h *cacheHasher) add(data []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(data)))
	h.Write(length[:])
	h.Write(data)
}
func (h *cacheHasher) text(value string) { h.add([]byte(value)) }
func (h *cacheHasher) finish() Hash      { var result Hash; copy(result[:], h.Sum(nil)); return result }

func computeTranslateToolHash(identity buildspec.Identity) (Hash, error) {
	canonical, err := buildspec.Canonical(identity)
	if err != nil {
		return Hash{}, fmt.Errorf("translation identity: %w", err)
	}
	h := newCacheHasher()
	h.text(translationNamespace)
	h.add(canonical)
	return h.finish(), nil
}

func computePackageHash(tool Hash, pkg *packages.Package, importHashes map[string]Hash) (Hash, error) {
	if pkg == nil || pkg.ID == "" {
		return Hash{}, errors.New("missing package hash input")
	}
	h := newCacheHasher()
	h.text(translationNamespace)
	h.add(tool[:])
	h.text(pkg.ID)
	h.text(pkg.PkgPath)
	h.text(pkg.Name)
	for _, section := range []struct {
		name  string
		files []string
	}{
		{"compiled-go", pkg.CompiledGoFiles}, {"non-go", pkg.OtherFiles}, {"embed", pkg.EmbedFiles},
	} {
		files := slices.Clone(section.files)
		slices.Sort(files)
		files = slices.Compact(files)
		h.text(section.name)
		h.text(fmt.Sprint(len(files)))
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return Hash{}, fmt.Errorf("hash %s source %s: %w", pkg.ID, file, err)
			}
			h.text(filepath.ToSlash(file))
			digest := sha256.Sum256(data)
			h.add(digest[:])
		}
	}
	// The canonical identity contains the complete resolved module-graph digest;
	// package-local module/replacement metadata and go.mod bytes are inputs too.
	h.text("module")
	module, err := json.Marshal(pkg.Module)
	if err != nil {
		return Hash{}, fmt.Errorf("hash %s module: %w", pkg.ID, err)
	}
	h.add(module)
	for m := pkg.Module; m != nil; m = m.Replace {
		if m.GoMod != "" {
			data, err := os.ReadFile(m.GoMod)
			if err != nil {
				return Hash{}, fmt.Errorf("hash %s go.mod: %w", pkg.ID, err)
			}
			h.text(m.GoMod)
			digest := sha256.Sum256(data)
			h.add(digest[:])
		}
	}
	h.text("imports")
	keys := make([]string, 0, len(importHashes))
	for key := range importHashes {
		if key != pkg.ID {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	h.text(fmt.Sprint(len(keys)))
	for _, key := range keys {
		h.text(key)
		digest := importHashes[key]
		h.add(digest[:])
	}
	return h.finish(), nil
}

func validateCacheEntry(entry CacheEntryV2, identity, packageDigest string) error {
	if entry.Format != 2 || !buildspec.ValidDigest(identity) || !buildspec.ValidDigest(packageDigest) ||
		entry.IdentityDigest != identity ||
		entry.PackageDigest != packageDigest {
		return errors.New("cache format or identity/package digest mismatch")
	}
	if err := gosimtool.ValidateGeneratedFiles(entry.Result.TranslatedFiles); err != nil {
		return fmt.Errorf("unsafe cached output: %w", err)
	}
	return nil
}

func encode(entry CacheEntryV2) ([]byte, error) {
	if err := validateCacheEntry(entry, entry.IdentityDigest, entry.PackageDigest); err != nil {
		return nil, err
	}
	var data bytes.Buffer
	writer := gzip.NewWriter(&data)
	if err := gob.NewEncoder(writer).Encode(entry); err != nil {
		writer.Close()
		return nil, fmt.Errorf("encode translation cache: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("compress translation cache: %w", err)
	}
	return data.Bytes(), nil
}

func decode(data []byte, expectedIdentity, expectedPackage string) (CacheEntryV2, error) {
	var entry CacheEntryV2
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return entry, err
	}
	// Consume the gzip footer, including its checksum, before trusting the gob.
	payload, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return entry, err
	}
	decoder := gob.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&entry); err != nil {
		return entry, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return CacheEntryV2{}, fmt.Errorf("trailing cache payload: %v", err)
	}
	if err := validateCacheEntry(entry, expectedIdentity, expectedPackage); err != nil {
		return CacheEntryV2{}, err
	}
	return entry, nil
}

func cacheGet(c *cache.Cache, key Hash, identity buildspec.Identity) (*TranslatePackageResult, bool, error) {
	digest, err := buildspec.Digest(identity)
	if err != nil {
		return nil, false, fmt.Errorf("cache identity: %w", err)
	}
	keyString := hex.EncodeToString(key[:])
	payload, err := c.Get(keyString)
	if errors.Is(err, cache.ErrNoSuchKey) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read translation cache: %w", err)
	}
	entry, err := decode(payload, digest, keyString)
	if err != nil {
		log.Printf("discarding corrupt translation cache entry %s: %v", keyString, err)
		if err := c.Delete(keyString); err != nil {
			return nil, false, fmt.Errorf("delete corrupt translation cache entry: %w", err)
		}
		return nil, false, nil
	}
	return &entry.Result, true, nil
}

func cachePut(c *cache.Cache, key Hash, entry CacheEntryV2) error {
	keyString := hex.EncodeToString(key[:])
	if entry.PackageDigest != keyString {
		return errors.New("cache key differs from envelope package digest")
	}
	data, err := encode(entry)
	if err != nil {
		return err
	}
	if err := c.Put(keyString, data); err != nil {
		return fmt.Errorf("write translation cache: %w", err)
	}
	return nil
}
