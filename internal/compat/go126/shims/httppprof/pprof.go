//go:build go1.26 && !go1.27

// Package httppprof keeps optional profiling handlers fail-closed.
package httppprof

import (
	"net/http"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(w http.ResponseWriter, r *http.Request) {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "rt",
			Operation: "net/http/pprof",
			Message:   "host profiling is unsupported",
		},
	)
}
func Handler(name string) http.Handler               { return http.HandlerFunc(unsupported) }
func Index(w http.ResponseWriter, r *http.Request)   { unsupported(w, r) }
func Cmdline(w http.ResponseWriter, r *http.Request) { unsupported(w, r) }
func Profile(w http.ResponseWriter, r *http.Request) { unsupported(w, r) }
func Symbol(w http.ResponseWriter, r *http.Request)  { unsupported(w, r) }
func Trace(w http.ResponseWriter, r *http.Request)   { unsupported(w, r) }
