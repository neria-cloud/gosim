//go:build sim

package behavior_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestNetTcpHttp(t *testing.T) {
	shutdown := make(chan struct{})
	a := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",
		Main: func() {
			listener, err := net.Listen("tcp", aAddr+":1234")
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Remote-Addr", r.RemoteAddr)
				if _, err := io.Copy(w, r.Body); err != nil {
					t.Error(err)
				}
			})
			mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("first"))
				w.(http.Flusher).Flush()
				time.Sleep(50 * time.Millisecond)
				_, _ = w.Write([]byte("second"))
			})
			mux.HandleFunc("/wait", func(w http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
			})
			server := &http.Server{Handler: mux}
			stopped := make(chan error, 1)
			go func() {
				select {
				case <-shutdown:
				case <-time.After(12 * time.Second):
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				stopped <- server.Shutdown(ctx)
			}()
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Fatal(err)
			}
			if err := <-stopped; err != nil {
				t.Fatal(err)
			}
		},
	}, netip.MustParseAddr(aAddr))

	b := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "b",
		Main: func() {
			defer close(shutdown)
			time.Sleep(time.Second)
			transport := &http.Transport{MaxIdleConnsPerHost: 2, MaxConnsPerHost: 1}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			url := fmt.Sprintf("http://%s:1234", aAddr)
			var firstRemote string
			for i, body := range []string{"hello one", "hello two"} {
				response, err := client.Post(url+"/echo", "text/plain", bytes.NewBufferString(body))
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.StatusCode != http.StatusOK || string(got) != body {
					t.Fatalf("echo %d: status=%d body=%q err=%v", i, response.StatusCode, got, err)
				}
				if i == 0 {
					firstRemote = response.Header.Get("X-Remote-Addr")
				} else if remote := response.Header.Get("X-Remote-Addr"); firstRemote == "" || remote != firstRemote {
					t.Fatalf("persistent connection changed from %q to %q", firstRemote, remote)
				}
			}
			response, err := client.Get(url + "/stream")
			if err != nil {
				t.Fatal(err)
			}
			stream, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || string(stream) != "firstsecond" {
				t.Fatalf("stream body=%q err=%v", stream, err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/wait", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err = client.Do(request)
			if response != nil {
				response.Body.Close()
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("request deadline: %v", err)
			}
			cancelCtx, cancelRequest := context.WithCancel(context.Background())
			request, err = http.NewRequestWithContext(cancelCtx, http.MethodGet, url+"/wait", nil)
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				time.Sleep(100 * time.Millisecond)
				cancelRequest()
			}()
			response, err = client.Do(request)
			if response != nil {
				response.Body.Close()
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("request cancellation: %v", err)
			}
		},
	}, netip.MustParseAddr(bAddr))

	testsupport.Wait(t, b)
	testsupport.Wait(t, a)
}
