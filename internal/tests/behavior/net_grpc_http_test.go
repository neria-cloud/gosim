//go:build ignore
// +build ignore

// TestNetTcpHttp and TestNetTcpGrpc are excluded from the gosim simulation
// build because crypto/tls (and therefore net/http and gRPC) cannot be
// translated in Go 1.25+. The root cause is that crypto/tls now imports
// crypto/internal/fips140/* packages, which are internal to the Go standard
// library module and use runtime //go:linkname directives that become
// undefined when translated into gosim's translated/ module.
//
// As a result, net/http and google.golang.org/grpc are left as real (untranslated)
// packages. Their types (net.Conn, context.Context, time.Time) are the real stdlib
// types, which don't match the translated equivalents used by the rest of
// translated gosim code. This causes compile errors in any translated code that
// bridges the two type universes.
//
// When a path to translating crypto/tls is found (e.g. gosim stubs for the
// fips140 packages), these tests can be moved back into net_test.go.

package behavior_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testpb"
)

func TestNetTcpHttp(t *testing.T) {
	a := gosim.NewMachine(gosim.MachineConfig{
		Label: "a",
		Addr:  netip.MustParseAddr(aAddr),
		MainFunc: func() {
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
	})

	b := gosim.NewMachine(gosim.MachineConfig{
		Label: "b",
		Addr:  netip.MustParseAddr(bAddr),
		MainFunc: func() {
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
	})

	a.Wait()
	b.Wait()
}

type testServer struct{}

func (s *testServer) Echo(ctx context.Context, req *testpb.EchoRequest) (*testpb.EchoResponse, error) {
	return &testpb.EchoResponse{
		Message: req.Message,
	}, nil
}

func TestNetTcpGrpc(t *testing.T) {
	a := gosim.NewMachine(gosim.MachineConfig{
		Label: "a",
		Addr:  netip.MustParseAddr(aAddr),
		MainFunc: func() {
			listener, err := net.Listen("tcp", aAddr+":1234")
			if err != nil {
				t.Fatal(err)
			}

			server := grpc.NewServer()
			testpb.RegisterEchoServerServer(server, &testServer{})

			go func() {
				time.Sleep(10 * time.Second)
				server.GracefulStop()
			}()

			if err := server.Serve(listener); err != nil {
				t.Fatal(err)
			}
		},
	})

	b := gosim.NewMachine(gosim.MachineConfig{
		Label: "b",
		Addr:  netip.MustParseAddr(bAddr),
		MainFunc: func() {
			// give the other a second to start listening
			time.Sleep(time.Second)

			client, err := grpc.Dial(fmt.Sprintf("%s:1234", aAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatal(err)
			}

			testClient := testpb.NewEchoServerClient(client)

			resp, err := testClient.Echo(context.Background(), &testpb.EchoRequest{Message: "hello world"})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Message != "hello world" {
				t.Fatal(resp.Message)
			}

			client.Close()
		},
	})

	a.Wait()
	b.Wait()
}
