//go:build never

package gosimtool

// go mod tidy considers this dependency even when a caller imports metatesting
// without importing the root gosim package. Worker startup can then use the
// caller's selected tool dependencies without editing its go.mod.
import _ "github.com/neria-cloud/gosim/cmd/gosim-worker"
