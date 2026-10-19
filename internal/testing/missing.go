//go:build gosim_retired

package testing

func AllocsPerRun(runs int, f func()) (avg float64) {
	panic("not implemented")
}

func CoverMode() {
	panic("not implemented")
}

func Coverage() float64 {
	panic("not implemented")
}

func Init() {
	panic("not implemented")
}

func Main(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, examples []InternalExample) {
	panic("not implemented")
}

func RegisterCover(c Cover) {
	panic("not implemented")
}

func RunBenchmarks(matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) {
	panic("not implemented")
}

func RunExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ok bool) {
	panic("not implemented")
}

func RunTests(matchString func(pat, str string) (bool, error), tests []InternalTest) (ok bool) {
	panic("not implemented")
}

func Testing() bool {
	panic("not implemented")
}

func Verbose() bool {
	panic("not implemented")
}

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
