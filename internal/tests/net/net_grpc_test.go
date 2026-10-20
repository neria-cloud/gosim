//go:build sim

package behavior_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testpb"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

type testServer struct{}

func (s *testServer) Echo(ctx context.Context, req *testpb.EchoRequest) (*testpb.EchoResponse, error) {
	return &testpb.EchoResponse{
		Message: req.Message,
	}, nil
}

func TestNetTcpGrpc(t *testing.T) {
	a := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",
		Main: func() {
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
	}, netip.MustParseAddr(aAddr))

	b := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "b",
		Main: func() {
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
	}, netip.MustParseAddr(bAddr))

	testsupport.Wait(t, a)
	testsupport.Wait(t, b)
}
