//go:build go1.26

package rand

import (
	"io"
	"os"
	"strings"
)

func primeReader(rand io.Reader) io.Reader {
	// Go 1.26 selects its internal default reader unless this compatibility
	// setting opts back into the supplied reader. Read translated machine state;
	// never use the host entropy reader or the mutable public Reader as default.
	custom := ""
	for _, setting := range strings.Split(os.Getenv("GODEBUG"), ",") {
		key, value, _ := strings.Cut(setting, "=")
		if key == "cryptocustomrand" {
			custom = value
		}
	}
	if custom != "1" {
		rand = &deterministicReader{}
	}
	return rand
}
