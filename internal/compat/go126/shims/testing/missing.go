//go:build go1.26 && !go1.27

package testing

func AllocsPerRun(runs int, f func()) (avg float64) {
	panic("gosim go126 testing: unsupported operation")
}

func CoverMode() string { return "" }

func Coverage() float64 {
	panic("gosim go126 testing: unsupported operation")
}

// Init is a modeled init probe: translated tests are registered by composition.
func Init() {}

func Main(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, examples []InternalExample) {
	panic("gosim go126 testing: unsupported operation")
}

func RegisterCover(c Cover) {
	panic("gosim go126 testing: unsupported operation")
}

func RunBenchmarks(matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) {
	panic("gosim go126 testing: unsupported operation")
}

func RunExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ok bool) {
	panic("gosim go126 testing: unsupported operation")
}

func RunTests(matchString func(pat, str string) (bool, error), tests []InternalTest) (ok bool) {
	panic("gosim go126 testing: unsupported operation")
}

func Testing() bool { return true }

// The simulation test runner always emits verbose events.
func Verbose() bool { return true }

type Cover struct {
	Mode            string
	Counters        map[string][]uint32
	Blocks          map[string][]CoverBlock
	CoveredPackages string
}

type CoverBlock struct {
	Line0 uint32 // Line number for block start.
	Col0  uint16 // Column number for block start.
	Line1 uint32 // Line number for block end.
	Col1  uint16 // Column number for block end.
	Stmts uint16 // Number of statements included in this block.
}

type InternalBenchmark struct {
	Name string
	F    func(b *B)
}

type InternalExample struct {
	Name      string
	F         func()
	Output    string
	Unordered bool
}

type InternalFuzzTarget struct {
	Name string
	Fn   func(f *F)
}

type InternalTest struct {
	Name string
	F    func(*T)
}

type F struct{}

type PB struct{}

// TODO: missing methods
