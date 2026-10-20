//go:build sim

package grpc_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
	simrt "github.com/neria-cloud/gosim/rt"
)

type echoService interface{}

func authenticated(ctx context.Context) error {
	md, _ := metadata.FromIncomingContext(ctx)
	if values := md.Get("authorization"); len(values) != 1 || values[0] != "test-token" {
		return status.Error(codes.Unauthenticated, "missing token")
	}
	return nil
}

var service = grpc.ServiceDesc{
	ServiceName: "simulation.Echo", HandlerType: (*echoService)(nil),
	Methods: []grpc.MethodDesc{{MethodName: "Unary", Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
		if err := authenticated(ctx); err != nil {
			return nil, err
		}
		msg := new(wrapperspb.BytesValue)
		if err := decode(msg); err != nil {
			return nil, err
		}
		return msg, nil
	}}},
	Streams: []grpc.StreamDesc{{StreamName: "Bidi", ServerStreams: true, ClientStreams: true, Handler: func(_ any, stream grpc.ServerStream) error {
		if err := authenticated(stream.Context()); err != nil {
			return err
		}
		for {
			msg := new(wrapperspb.BytesValue)
			if err := stream.RecvMsg(msg); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if err := stream.SendMsg(msg); err != nil {
				return err
			}
		}
	}}},
}

func TestPlaintextRPC(t *testing.T)        { testPlaintextRPC(t, 0) }
func TestPlaintextRPCLatency(t *testing.T) { testPlaintextRPC(t, 200*time.Millisecond) }

func testPlaintextRPC(t *testing.T, latency time.Duration) {
	if err := simrt.SetTimeout(30 * time.Second); err != nil {
		t.Fatal(err)
	}

	config := simnet.DefaultConfig()
	config.Latency.Min, config.Latency.Max = latency, latency
	config.ConnectionTimeout = 10 * time.Second
	config.TCPBufferSize = 256 * 1024
	if err := simnet.Configure(config); err != nil {
		t.Fatal(err)
	}

	ready, stop := make(chan struct{}), make(chan struct{})
	server, err := gosim.NewMachine(gosim.MachineConfig{Label: "grpc-server", Main: func() {
		listener, err := net.Listen("tcp", "10.0.0.1:9000")
		if err != nil {
			t.Fatal(err)
		}
		s := grpc.NewServer()
		s.RegisterService(&service, struct{}{})
		go func() {
			if err := s.Serve(listener); err != nil {
				t.Error(err)
			}
		}()
		close(ready)
		<-stop
		s.Stop()
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := simnet.ConfigureMachine(server, simnet.MachineConfig{Addr: netip.MustParseAddr("10.0.0.1")}); err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	<-ready
	conn, err := grpc.NewClient("passthrough:///10.0.0.1:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payload := &wrapperspb.BytesValue{Value: bytes.Repeat([]byte{0, 255, 128, 1}, 32768)}
	if err := conn.Invoke(ctx, "/simulation.Echo/Unary", payload, new(wrapperspb.BytesValue)); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated call: %v", err)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "test-token")
	out := new(wrapperspb.BytesValue)
	if err := conn.Invoke(ctx, "/simulation.Echo/Unary", payload, out); err != nil {
		t.Fatalf("unary: %v", err)
	}
	if !bytes.Equal(out.Value, payload.Value) {
		t.Fatal("unary payload changed")
	}
	stream, err := conn.NewStream(ctx, &service.Streams[0], "/simulation.Echo/Bidi")
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 3; n++ {
		if err := stream.SendMsg(payload); err != nil {
			t.Fatal(err)
		}
		out := new(wrapperspb.BytesValue)
		if err := stream.RecvMsg(out); err != nil {
			t.Fatalf("stream receive %d: %v", n, err)
		}
		if !bytes.Equal(out.Value, payload.Value) {
			t.Fatal("stream payload changed")
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	if err := stream.RecvMsg(new(wrapperspb.BytesValue)); err != io.EOF {
		t.Fatalf("stream end: %v", err)
	}
	close(stop)
	if exit, err := server.Wait(); err != nil || exit.Kind != gosim.Graceful {
		t.Fatalf("server: %+v %v", exit, err)
	}
}
