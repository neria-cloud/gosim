//go:build go1.25 && !go1.26

package rt

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	_ "unsafe"

	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

const (
	chachaSeedWords      = 4
	chachaTestSeed       = 17
	chachaTestSamples    = 64
	byteTestSeed         = 19
	byteTestSamples      = 60
	byteTestRandomMax    = 300
	byteTestRange        = 256
	byteTestCorpusRepeat = 70
	noZeroTestSize       = 19
)

// Test-only oracle from the selected native runtime; production uses Go code.
//
//go:linkname nativeChaChaBlock internal/chacha8rand.block
func nativeChaChaBlock(*[chachaSeedWords]uint64, *[32]uint64, uint32)

func TestChaCha8MatchesSelectedRuntime(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(chachaTestSeed, 23))
	for sample := 0; sample < chachaTestSamples; sample++ {
		seed := [chachaSeedWords]uint64{r.Uint64(), r.Uint64(), r.Uint64(), r.Uint64()}
		for _, counter := range []uint32{0, 4, 8, 12, 0xfffffffe, 0xffffffff} {
			var got, want [32]uint64
			InternalChacha8rand_block(&seed, &got, counter)
			nativeChaChaBlock(&seed, &want, counter)
			if got != want {
				t.Fatalf("sample=%d counter=%d got=%x want=%x", sample, counter, got, want)
			}
		}
	}
}

func TestByteHelpersMatchStandardLibrary(t *testing.T) {
	t.Parallel()
	corpus := []string{"", "a", "aa", "ab", "ba", "\x00\xff\xc0", "é世", strings.Repeat("ab", byteTestCorpusRepeat)}
	r := rand.New(rand.NewPCG(byteTestSeed, 29))
	for i := 0; i < byteTestSamples; i++ {
		b := make([]byte, r.IntN(byteTestRandomMax))
		for j := range b {
			b[j] = byte(r.Uint64())
		}
		corpus = append(corpus, string(b))
	}
	for _, a := range corpus {
		for _, b := range corpus {
			if got := InternalBytealg_Compare([]byte(a), []byte(b)); got != bytes.Compare([]byte(a), []byte(b)) {
				t.Fatal("compare", a, b, got)
			}
			if got := InternalBytealg_abigen_runtime_cmpstring(a, b); got != strings.Compare(a, b) {
				t.Fatal("cmpstring", a, b, got)
			}
			if InternalBytealg_Equal([]byte(a), []byte(b)) != (a == b) {
				t.Fatal("equal", a, b)
			}
			if got := InternalBytealg_Index([]byte(a), []byte(b)); got != bytes.Index([]byte(a), []byte(b)) {
				t.Fatal("index", a, b, got)
			}
			if got := InternalBytealg_IndexString(a, b); got != strings.Index(a, b) {
				t.Fatal("indexstring", a, b, got)
			}
		}
		for c := 0; c < byteTestRange; c++ {
			if InternalBytealg_IndexByte([]byte(a), byte(c)) != bytes.IndexByte([]byte(a), byte(c)) || InternalBytealg_IndexByteString(a, byte(c)) != strings.IndexByte(a, byte(c)) {
				t.Fatal("indexbyte", a, c)
			}
			want := bytes.Count([]byte(a), []byte{byte(c)})
			if InternalBytealg_Count([]byte(a), byte(c)) != want || InternalBytealg_CountString(a, byte(c)) != want {
				t.Fatal("count", a, c)
			}
		}
	}
	if !InternalBytealg_Equal(nil, []byte{}) {
		t.Fatal("nil and empty differ")
	}
	b := InternalBytealg_MakeNoZero(noZeroTestSize)
	if len(b) != noZeroTestSize {
		t.Fatal("allocation length")
	}
	b[noZeroTestSize-1] = 1
}

func TestNetRandomUsesApplicationStream(t *testing.T) {
	var got, want [16]uint64
	runHooks(t, func() {
		for i := range got {
			got[i] = Net_runtime_rand()
		}
	})
	runHooks(t, func() {
		for i := range want {
			want[i] = simrt.Fastrand64()
		}
	})
	if got != want {
		t.Fatal("net randomness uses another stream")
	}
}

func TestPrivateRuntimeOperationsAbort(t *testing.T) {
	operations := map[string]func(){
		"internal/abi.FuncPCABI0":                         func() { InternalAbi_FuncPCABI0(nil) },
		"internal/abi.FuncPCABIInternal":                  func() { InternalAbi_FuncPCABIInternal(nil) },
		"internal/bytealg.abigen_runtime_memequal":        func() { InternalBytealg_abigen_runtime_memequal(nil, nil, 1) },
		"internal/bytealg.abigen_runtime_memequal_varlen": func() { InternalBytealg_abigen_runtime_memequal_varlen(nil, nil) },
		"internal/cpu.cpuid":                              func() { InternalCpu_cpuid(0, 0) },
		"internal/cpu.xgetbv":                             func() { InternalCpu_xgetbv() },
		"internal/cpu.getGOAMD64level":                    func() { InternalCpu_getGOAMD64level() },
		"internal/cpu.getisar0":                           func() { InternalCpu_getisar0() },
		"internal/cpu.getMIDR":                            func() { InternalCpu_getMIDR() },
		"internal/cpu.getpfr0":                            func() { InternalCpu_getpfr0() },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 1, TimeoutNS: 1e9, ReplayRevision: simrt.ReplayRevision})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			after, deferred := false, false
			err = e.Run(func() {
				token, err := e.PrepareMachine(1, 1, "reject")
				if err != nil {
					t.Fatal(err)
				}
				e.PublishMachine(token, func() { defer func() { _ = recover(); deferred = true }(); operation(); after = true })
				simrt.NewChan[struct{}](0).Recv()
			})
			var aborted *simrt.AbortError
			if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != name || after || deferred {
				t.Fatalf("err=%v after=%v deferred=%v", err, after, deferred)
			}
		})
	}
}
