//go:build go1.25 && !go1.26

package rt

// These scalar helpers stay translated. In particular, importing the complete
// native bytes/strings packages would bypass translated error and iterator types.
func byteCompare[T ~string | ~[]byte](a, b T) int {
	for i := 0; i < min(len(a), len(b)); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func byteIndex[T ~string | ~[]byte](s T, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func byteCount[T ~string | ~[]byte](s T, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

func byteSearch[T ~string | ~[]byte](a, b T) int {
	for i := 0; i <= len(a)-len(b); i++ {
		j := 0
		for j < len(b) && a[i+j] == b[j] {
			j++
		}
		if j == len(b) {
			return i
		}
	}
	return -1
}
func InternalBytealg_IndexByteString(s string, c byte) int     { return byteIndex(s, c) }
func InternalBytealg_IndexByte(s []byte, c byte) int           { return byteIndex(s, c) }
func InternalBytealg_Equal(a, b []byte) bool                   { return byteCompare(a, b) == 0 }
func InternalBytealg_Compare(a, b []byte) int                  { return byteCompare(a, b) }
func InternalBytealg_abigen_runtime_cmpstring(a, b string) int { return byteCompare(a, b) }
func InternalBytealg_Count(s []byte, c byte) int               { return byteCount(s, c) }
func InternalBytealg_CountString(s string, c byte) int         { return byteCount(s, c) }
func InternalBytealg_Index(a, b []byte) int                    { return byteSearch(a, b) }
func InternalBytealg_IndexString(a, b string) int              { return byteSearch(a, b) }

// Allocation optimization only: callers cannot depend on uninitialized contents.
func InternalBytealg_MakeNoZero(n int) []byte { return make([]byte, n) }
