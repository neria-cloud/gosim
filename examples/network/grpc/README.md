# Plaintext gRPC on simulated TCP

This independent example defines its own protobuf byte-echo service. It has no
application dependency. It exercises metadata authentication, unary RPCs,
bidirectional streaming, HTTP/2 flow control with 128 KiB binary payloads, and
clean stream completion between a simulated client and server.

From the Gosim repository root:

```sh
go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim
./.gosim/gosimtool -C examples/network/grpc test -features rt,net -seeds 1-3 -v .
```

Go tooling selects the compiler; both supported Go families can run the example.
Its separate module pins gRPC 1.79.3 and protobuf 1.36.10, with a local replacement
for this Gosim checkout. No dependency on another application is required.

The two scenarios use zero latency and fixed 200 ms one-way latency. The example configures
`TCPBufferSize: 256 * 1024` before opening connections, so the large-payload
scenario is not throttled by the historical 1 KiB default window. Its RPC deadline
is 20 simulated seconds. This is a correctness example, not a throughput benchmark.

The connection uses an explicit passthrough IPv4 target and insecure transport
credentials. Authentication is application metadata checked by the example
server. TLS, certificates, DNS, and proxy behavior are outside this example.
Optional TLS and host runtime-metric operations still abort if executed.
