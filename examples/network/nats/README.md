# Embedded NATS with delayed links

This is a separate application module. Its `go.mod` owns the NATS server/client
versions and replaces Gosim with the checkout at `../../..`. The Gosim module has
no NATS dependency. Root `go test ./...` and the root example package patterns do
not include this nested module.

[Architecture diagram](../../../docs/diagrams/gosim-architecture.svg) shows the
implementation layers. [Animated test workflows](../../../docs/diagrams/gosim-test-workflows.svg)
illustrate network and filesystem capabilities. [Diagram notes](../../../docs/diagrams/README.md)
map the components to source.

From the repository root:

```sh
make build
./.gosim/gosimtool -C examples/network/nats test -features rt,fs,net -v .
```

Ordinary Go tooling selects the compiler. An optional `GOTOOLCHAIN=go1.26.9`
prefix selects that compiler for the same command.
`-C` must precede `test`; it selects this module without changing your shell's
directory. To filter test names, add `-run '^TestPublisherReceiver'` before `.`.

This runs both scenarios, which share the same server, clients, and message checks:

- `TestPublisherReceiver`: 1–3 seconds of one-way latency per network leg.
- `TestPublisherReceiverWithoutLatency`: zero injected network latency.

Each test starts three simulated hosts:

- `10.62.0.1`: a real embedded NATS Core server.
- `10.62.0.2`: a real NATS Go client publishing `1000` through `1009` once per simulated second.
- `10.62.0.3`: a second NATS client receiving and checking the sequence.

Each message contains the decimal number as its payload and an RFC3339Nano
`Send-Time` header. The publisher prints its send time; the receiver prints both
timestamps and their difference in nanoseconds:

```text
SEND payload=1000 sent=2020-01-15T14:10:03.025001234Z
RECEIVE payload=1000 sent=2020-01-15T14:10:03.025001234Z received=2020-01-15T14:10:05.680001234Z latency=2655000000ns
```

The host-to-host connection timeout is **five seconds**. After both clients
connect and the subscription is registered, each network leg gets the selected
scenario's latency, with connection drops disabled. This setup phase avoids
racing publication against subscription registration. The publisher-to-server
and server-to-receiver legs contribute separately to delivery time; the
five-second connection timeout is not a message expiry. Queueing can add delay
beyond those two network legs. The receiver allows ten seconds per next message.
Time in the output is simulated time; execution is not paced to wall-clock time.

Only plain TCP NATS Core is exercised. TLS/certificate operations, monitoring,
JetStream, native signal handling and native profiling are not enabled. Gosim
translates NATS's ordinary Go protocol/concurrency code, delegates reviewed
computation to native Go implementations, and supplies deterministic randomness.
Unsupported optional operations abort explicitly instead of accessing the host.

The result should looks like this:
```
❯   ../../../.gosim/gosimtool test -features rt,fs,net -v -run .
=== RUN   TestPublisherReceiver (seed 1)
    1 publisher/10 14:10:03.025 INF SEND payload=1000 sent=2020-01-15T14:10:03.025001234Z method=stdout
    2 publisher/10 14:10:04.025 INF SEND payload=1001 sent=2020-01-15T14:10:04.025001234Z method=stdout
    3 publisher/10 14:10:05.025 INF SEND payload=1002 sent=2020-01-15T14:10:05.025001234Z method=stdout
    4 receiver/9 14:10:05.680 INF RECEIVE payload=1000 sent=2020-01-15T14:10:03.025001234Z received=2020-01-15T14:10:05.680001234Z latency=2655000000ns method=stdout
    5 publisher/10 14:10:06.025 INF SEND payload=1003 sent=2020-01-15T14:10:06.025001234Z method=stdout
    6 receiver/9 14:10:06.796 INF RECEIVE payload=1001 sent=2020-01-15T14:10:04.025001234Z received=2020-01-15T14:10:06.796001234Z latency=2771000000ns method=stdout
    7 publisher/10 14:10:07.025 INF SEND payload=1004 sent=2020-01-15T14:10:07.025001234Z method=stdout
    8 receiver/9 14:10:07.300 INF RECEIVE payload=1002 sent=2020-01-15T14:10:05.025001234Z received=2020-01-15T14:10:07.300001234Z latency=2275000000ns method=stdout
    9 publisher/10 14:10:08.025 INF SEND payload=1005 sent=2020-01-15T14:10:08.025001234Z method=stdout
   10 publisher/10 14:10:09.025 INF SEND payload=1006 sent=2020-01-15T14:10:09.025001234Z method=stdout
   11 publisher/10 14:10:10.025 INF SEND payload=1007 sent=2020-01-15T14:10:10.025001234Z method=stdout
   12 receiver/9 14:10:10.207 INF RECEIVE payload=1003 sent=2020-01-15T14:10:06.025001234Z received=2020-01-15T14:10:10.207001234Z latency=4182000000ns method=stdout
   13 receiver/9 14:10:10.207 INF RECEIVE payload=1004 sent=2020-01-15T14:10:07.025001234Z received=2020-01-15T14:10:10.207001234Z latency=3182000000ns method=stdout
   14 receiver/9 14:10:10.207 INF RECEIVE payload=1005 sent=2020-01-15T14:10:08.025001234Z received=2020-01-15T14:10:10.207001234Z latency=2182000000ns method=stdout
   15 publisher/10 14:10:11.025 INF SEND payload=1008 sent=2020-01-15T14:10:11.025001234Z method=stdout
   16 publisher/10 14:10:12.025 INF SEND payload=1009 sent=2020-01-15T14:10:12.025001234Z method=stdout
   17 receiver/9 14:10:12.109 INF RECEIVE payload=1006 sent=2020-01-15T14:10:09.025001234Z received=2020-01-15T14:10:12.109001234Z latency=3084000000ns method=stdout
   18 receiver/9 14:10:14.471 INF RECEIVE payload=1007 sent=2020-01-15T14:10:10.025001234Z received=2020-01-15T14:10:14.471001234Z latency=4446000000ns method=stdout
   19 receiver/9 14:10:14.471 INF RECEIVE payload=1008 sent=2020-01-15T14:10:11.025001234Z received=2020-01-15T14:10:14.471001234Z latency=3446000000ns method=stdout
   20 receiver/9 14:10:14.471 INF RECEIVE payload=1009 sent=2020-01-15T14:10:12.025001234Z received=2020-01-15T14:10:14.471001234Z latency=2446000000ns method=stdout
--- PASS: TestPublisherReceiver (14.08s simulated)
```
