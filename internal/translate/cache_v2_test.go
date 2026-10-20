package translate

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/neria-cloud/gosim/internal/buildspec"
	"github.com/neria-cloud/gosim/internal/translate/cache"
)

func cacheIdentity() buildspec.Identity {
	d := strings.Repeat("a", 64)
	return buildspec.Identity{Schema: 2, Toolchain: "go1.26.9", Profile: "go126", ToolchainDigest: d, StdlibDigest: d, ContractDigest: d, AdapterRevision: d, RuntimeRevision: d, HostOS: "darwin", HostArch: "arm64", SimulationOS: "linux", SimulationArch: "arm64", Features: []string{"rt"}, Tags: []string{"linkname", "sim"}, ModuleGraphDigest: d, TranslatorDigest: d, DebugPolicyDigest: d}
}

func cacheEntry(t *testing.T, key Hash) CacheEntryV2 {
	t.Helper()
	digest, err := buildspec.Digest(cacheIdentity())
	if err != nil {
		t.Fatal(err)
	}
	return CacheEntryV2{Format: 2, IdentityDigest: digest, PackageDigest: hex.EncodeToString(key[:]), Result: TranslatePackageResult{PackageName: "p", PackagePath: "example/p", CollectedTests: []PackageSelector{{Pkg: "example/p", Selector: "TestOne"}}, GlobalInfo: &PackageGlobalInfo{Container: map[string]string{"x": "G"}, ShouldShare: map[string]bool{"y": true}, Rewritten: map[string]string{"TestOne": "ImplTestOne"}}, TranslatedFiles: map[string][]byte{"example/p/p.go": []byte("package p\n")}}}
}

func rawEnvelope(t *testing.T, value any) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if err := gob.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestV2CacheRoundTripCorruptionAndReplacement(t *testing.T) {
	db, err := cache.NewDB(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := cache.NewCache(db)
	key := Hash(sha256.Sum256([]byte("package input")))
	entry := cacheEntry(t, key)
	if result, hit, err := cacheGet(c, key, cacheIdentity()); err != nil || hit || result != nil {
		t.Fatal("initial miss", hit, err)
	}
	if err := cachePut(c, key, entry); err != nil {
		t.Fatal(err)
	}
	result, hit, err := cacheGet(c, key, cacheIdentity())
	if err != nil || !hit || !reflect.DeepEqual(*result, entry.Result) {
		t.Fatal("round trip", result, hit, err)
	}
	modified := entry
	modified.Result.PackageName = "changed"
	if err := cachePut(c, key, modified); err != nil {
		t.Fatal(err)
	}
	result, hit, err = cacheGet(c, key, cacheIdentity())
	if err != nil || !hit || result.PackageName != "p" {
		t.Fatal("immutable insert replaced entry", hit, err)
	}
	wrong := entry
	wrong.PackageDigest = strings.Repeat("b", 64)
	if err := cachePut(c, key, wrong); err == nil {
		t.Fatal("envelope key mismatch accepted")
	}
	keyString := hex.EncodeToString(key[:])
	valid, err := encode(entry)
	if err != nil {
		t.Fatal(err)
	}
	crc := bytes.Clone(valid)
	crc[len(crc)-8] ^= 1
	old := entry
	old.Format = 1
	wrongID := entry
	wrongID.IdentityDigest = strings.Repeat("b", 64)
	unsafe := entry
	unsafe.Result.TranslatedFiles = map[string][]byte{"../escaped": []byte("bad")}
	for name, data := range map[string][]byte{"gzip": []byte("bad gzip"), "truncated": valid[:len(valid)-2], "checksum": crc, "old-envelope": rawEnvelope(t, old), "old-result": rawEnvelope(t, entry.Result), "wrong-identity": rawEnvelope(t, wrongID), "wrong-package": rawEnvelope(t, wrong), "unsafe-path": rawEnvelope(t, unsafe), "trailing": append(bytes.Clone(valid), rawEnvelope(t, entry)...)} {
		t.Run(name, func(t *testing.T) {
			if err := c.Delete(keyString); err != nil {
				t.Fatal(err)
			}
			if err := c.Put(keyString, data); err != nil {
				t.Fatal(err)
			}
			var diagnostic bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&diagnostic)
			defer log.SetOutput(previous)
			result, hit, err := cacheGet(c, key, cacheIdentity())
			if err != nil || hit || result != nil {
				t.Fatal("corruption was not a miss", hit, err)
			}
			if !strings.Contains(diagnostic.String(), keyString) || !strings.Contains(diagnostic.String(), "corrupt translation cache") {
				t.Fatal("corruption not observable", diagnostic.String())
			}
			if _, err := db.Get(keyString); !errors.Is(err, cache.ErrNoSuchKey) {
				t.Fatal("corrupt entry retained", err)
			}
			if err := cachePut(c, key, entry); err != nil {
				t.Fatal(err)
			}
			if _, hit, err := cacheGet(c, key, cacheIdentity()); err != nil || !hit {
				t.Fatal("replacement failed", hit, err)
			}
		})
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cacheGet(c, key, cacheIdentity()); err == nil {
		t.Fatal("SQLite read failure became miss")
	}
	if err := cachePut(c, key, entry); err == nil {
		t.Fatal("SQLite write failure ignored")
	}
}

func TestV2NeverLooksUpOldFormatKey(t *testing.T) {
	t.Parallel()
	db, err := cache.NewDB(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := cache.NewCache(db)
	oldKey := Hash(sha256.Sum256([]byte("old-format-key")))
	oldString := hex.EncodeToString(oldKey[:])
	payload := []byte("not even a valid old gob")
	if err := c.Put(oldString, payload); err != nil {
		t.Fatal(err)
	}
	tool, err := computeTranslateToolHash(cacheIdentity())
	if err != nil {
		t.Fatal(err)
	}
	key, err := computePackageHash(tool, &packages.Package{ID: "example/p", PkgPath: "example/p", Name: "p"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if key == oldKey {
		t.Fatal("v1/v2 namespace collision")
	}
	if _, hit, err := cacheGet(c, key, cacheIdentity()); err != nil || hit {
		t.Fatal("old key reused", hit, err)
	}
	stored, err := c.Get(oldString)
	if err != nil || !bytes.Equal(stored, payload) {
		t.Fatal("old entry decoded or removed", err)
	}
}

func TestV2IdentityDimensions(t *testing.T) {
	t.Parallel()
	base := cacheIdentity()
	original, err := computeTranslateToolHash(base)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < reflect.TypeOf(base).NumField(); i++ {
		t.Run(reflect.TypeOf(base).Field(i).Name, func(t *testing.T) {
			id := cacheIdentity()
			v := reflect.ValueOf(&id).Elem().Field(i)
			switch v.Kind() {
			case reflect.String:
				v.SetString(strings.Repeat("b", 64))
			case reflect.Int:
				v.SetInt(1)
			case reflect.Bool:
				v.SetBool(true)
			case reflect.Slice:
				n := 0
				if v.Len() == 0 {
					n = 1
				}
				v.Set(reflect.MakeSlice(v.Type(), n, n))
			default:
				t.Fatal("uncovered field")
			}
			if got, err := computeTranslateToolHash(id); err == nil && got == original {
				t.Fatal("identity mutation reused cache namespace")
			}
		})
	}
	id := cacheIdentity()
	id.Toolchain = "go1.26.1"
	if got, err := computeTranslateToolHash(id); err != nil || got == original {
		t.Fatal("patch change reused translation cache", err)
	}
	id = cacheIdentity()
	id.Tags = []string{"sim", "linkname"}
	if got, err := computeTranslateToolHash(id); err != nil || got != original {
		t.Fatal("ordering changed canonical cache identity", err)
	}
	a, b := newCacheHasher(), newCacheHasher()
	a.text("ab")
	a.text("c")
	b.text("a")
	b.text("bc")
	if a.finish() == b.finish() {
		t.Fatal("ambiguous field concatenation")
	}
}

func TestV2SourceContentAndSelectedInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b := write("a.go", "package p\nvar A=1\n"), write("b.go", "package p\nvar B=2\n")
	unselected := write("ignored_windows.go", "package p\n")
	asm, embedded, mod := write("a.s", "data"), write("asset.txt", "embedded"), write("go.mod", "module example\n")
	pkg := &packages.Package{ID: "example/p", PkgPath: "example/p", Name: "p", GoFiles: []string{a, b, unselected}, CompiledGoFiles: []string{a, b}, OtherFiles: []string{asm}, EmbedFiles: []string{embedded}, Module: &packages.Module{Path: "example", Version: "v1.0.0", GoMod: mod}}
	tool, err := computeTranslateToolHash(cacheIdentity())
	if err != nil {
		t.Fatal(err)
	}
	deps := map[string]Hash{"x": {1}, "y": {2}}
	hash := func() Hash {
		t.Helper()
		h, err := computePackageHash(tool, pkg, deps)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	original := hash()
	pkg.CompiledGoFiles = []string{b, a}
	if hash() != original {
		t.Fatal("selected-file ordering changed hash")
	}
	if err := os.WriteFile(unselected, []byte("different ignored file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hash() != original {
		t.Fatal("unselected source was hashed")
	}
	for _, file := range []string{a, asm, embedded, mod} {
		before, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, append(bytes.Clone(data), 'x'), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
		if hash() == original {
			t.Fatal("content mutation with unchanged mtime missed", file)
		}
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg.CompiledGoFiles = []string{a}
	if hash() == original {
		t.Fatal("compiled source selection ignored")
	}
	pkg.CompiledGoFiles = []string{a, b}
	deps["x"] = Hash{3}
	if hash() == original {
		t.Fatal("dependency ignored")
	}
	deps["x"] = Hash{1}
	pkg.Module.Version = "v1.0.1"
	if hash() == original {
		t.Fatal("module metadata ignored")
	}
	pkg.Module.Version = "v1.0.0"
	pkg.ID += " [example.test]"
	if hash() == original {
		t.Fatal("test variant ignored")
	}
	pkg.CompiledGoFiles = []string{filepath.Join(dir, "missing.go")}
	if _, err := computePackageHash(tool, pkg, deps); err == nil {
		t.Fatal("missing selected source ignored")
	}
}
