package gosimruntime

import (
	"math"
	"slices"
	"testing"
)

func TestDomainRandomKnownVector(t *testing.T) {
	t.Parallel()
	// Independent SHA-256/64-bit multiplication vector, including negative seed
	// encoding and uint64 little-endian length prefixes for each input field.
	r := NewDomainRandom(-123, "fs", 7)
	if r.state != 0xecf28b82b1610278 {
		t.Fatalf("derivation drift: %x", r.state)
	}
	for i, want := range []uint64{0xb8ac3c8c370a17f1, 0x5e6983e779c24d20, 0x39dad28d63ee9485, 0xf243ce92d458f617} {
		if got := r.Uint64(); got != want {
			t.Fatalf("draw %d: got %x want %x", i, got, want)
		}
	}
}

func TestDomainRandomBoundedAndFloatDraws(t *testing.T) {
	t.Parallel()
	for _, bound := range []uint64{1, 3, 255, 1<<63 + 1, math.MaxUint64} {
		r := NewDomainRandom(8, "net", 9)
		for i := 0; i < 1000; i++ {
			if got := r.Uint64N(bound); got >= bound {
				t.Fatalf("draw %d exceeds bound %d", got, bound)
			}
		}
	}
	// A high rejection threshold exercises discarded values, not just modulo.
	a, b := NewDomainRandom(1, "fs", 3), NewDomainRandom(1, "fs", 3)
	bound := uint64(1<<63 + 1)
	for i := 0; i < 100; i++ {
		var accepted uint64
		for accepted < -bound%bound {
			accepted = b.Uint64()
		}
		if a.Uint64N(bound) != accepted%bound || a.state != b.state {
			t.Fatal("rejection sampling consumed wrong draws")
		}
	}
	a, b = NewDomainRandom(1, "application", 2), NewDomainRandom(1, "application", 2)
	for i := 0; i < 100; i++ {
		if got, want := a.Float64(), float64(b.Uint64()>>11)/(1<<53); got != want || got < 0 || got >= 1 {
			t.Fatalf("Float64: %v != %v", got, want)
		}
		if got, want := a.ExpFloat64(), -math.Log1p(-b.Float64()); got != want || math.IsInf(got, 0) {
			t.Fatalf("ExpFloat64: %v != %v", got, want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("zero bound accepted")
		}
	}()
	a.Uint64N(0)
}

func TestEngineRandomOwnershipAndReplay(t *testing.T) {
	var outputs [][]uint64
	for run := 0; run < 2; run++ {
		e := testEngine(t)
		var sequence []uint64
		err := e.Run(func() {
			fs, net := e.RandomFor("fs", 7), e.RandomFor("net", 7)
			control := NewDomainRandom(42, "fs", 7)
			for i := 0; i < 20; i++ {
				net.Uint64() // another provider cannot consume this fs stream
				got := fs.Uint64()
				if got != control.Uint64() {
					t.Error("provider streams coupled")
				}
				// Store results on the native supervisor to keep the native test's
				// slice outside application race instrumentation.
				_ = e.onSupervisor(func() { sequence = append(sequence, got) })
			}
			token, err := e.PrepareMachine(7, 1, "old epoch")
			if err != nil {
				t.Fatal(err)
			}
			e.AbortMachine(token)
			token, err = e.PrepareMachine(7, 2, "new epoch")
			if err != nil {
				t.Fatal(err)
			}
			if again := e.RandomFor("fs", 7); again != fs || again.Uint64() != control.Uint64() {
				t.Error("restart rewound stream")
			}
			if e.RandomFor("fs", 8) == fs || e.RandomFor("scheduler", 0) == fs {
				t.Error("machine/domain stream identity aliased")
			}
			e.AbortMachine(token)
		})
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, sequence)
	}
	if !slices.Equal(outputs[0], outputs[1]) {
		t.Fatal("repeated seed changed with engine/process run identity")
	}
}

func TestApplicationRandomUsesLogicalMachineStream(t *testing.T) {
	e := testEngine(t)
	if err := e.Run(func() {
		out := NewChan[uint64](0)
		expected := NewDomainRandom(42, "application", 10)
		for epoch := uint64(1); epoch <= 2; epoch++ {
			token, err := e.PrepareMachine(10, epoch, "application")
			if err != nil {
				t.Fatal(err)
			}
			e.PublishMachine(token, func() { out.Send(Fastrand64()); out.Send(uint64(Fastrandn(17))) })
			if got := out.Recv(); got != expected.Uint64() {
				t.Errorf("application stream not derived from logical machine: %x", got)
			}
			if got := out.Recv(); got != expected.Uint64N(17) {
				t.Errorf("bounded application stream changed across epoch: %x", got)
			}
			e.CancelMachine(token)
			e.DisposeMachine(token)
		}
	}); err != nil {
		t.Fatal(err)
	}
}
