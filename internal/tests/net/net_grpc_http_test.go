//go:build ignore
// +build ignore

// Historical HTTP scenario retained separately from the supported plaintext
// gRPC tests. Full HTTP server compatibility is not claimed.

package behavior_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestNetTcpHttp(t *testing.T) {
	a := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",
		Main: func() {
			listener, err := net.Listen("tcp", aAddr+":1234")
			if err != nil {
				t.Fatal(err)
			}

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("hello world"))
			})

			srv := &http.Server{Handler: nil}

			go func() {
				time.Sleep(10 * time.Second)
				srv.Shutdown(context.Background())
			}()

			if err := srv.Serve(listener); err != nil {
				if err != http.ErrServerClosed {
					t.Fatal(err)
				}
			}
		},
	}, netip.MustParseAddr(aAddr))

	b := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "b",
		Main: func() {
			// give the other a second to start listening
			time.Sleep(time.Second)

			resp, err := http.Get(fmt.Sprintf("http://%s:1234/", aAddr))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			log.Println(resp.StatusCode)

			bytes, err := io.ReadAll(resp.Body)

			log.Println(string(bytes), err)

			if string(bytes) != "hello world" {
				t.Fatal(string(bytes))
			}
		},
	}, netip.MustParseAddr(bAddr))

	testsupport.Wait(t, a)
	testsupport.Wait(t, b)
}
