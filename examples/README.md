# Examples

From the repository root, build the command binaries from the current checkout, then run
the supported examples:

```sh
make build
./.gosim/gosimtool test -features rt,fs,net ./examples/simple ./examples/bolt ./examples/network/...
```

Go tooling selects the compiler from the caller's module/workspace and
`GOTOOLCHAIN`. The launcher automatically builds or reuses a matching worker for
supported Go 1.25.* and 1.26.* releases. No compiler pin or certification is
required. See [Go versions and configuration](../README.md#go-versions-and-configuration).

`make build` creates `.gosim/gosimtool`, `.gosim/gosim-worker`, and
`.gosim/gosim-gen`. Use `gosimtool` for normal commands.

If an existing `.gosim/gosimtool` reports `unsupported config schema 2`, run
`make build` to replace the old CLI. Keep the current `gosim.json`.

## Cases and packages

Each case has its own package, so it can be translated and run independently.
The three latency ranges share the latency case's ping/pong helper.

| Package | Case | Required features |
| --- | --- | --- |
| [simple](simple/simple_test.go) | Deterministic execution and randomness | `rt` |
| [bolt](bolt/bolt_test.go) | Write, read, and reopen a Bolt database | `rt,fs` |
| [network/machines](network/machines/machines_test.go) | TCP requests, machine crash/restart, and link latency | `rt,net` |
| [network/latency](network/latency/latency_test.go) | Three latency ranges without connection drops | `rt,net` |
| [etcd](etcd/README.md) | Retained etcd cluster, snapshot, and WAL fixtures | Unsupported HTTP/gRPC integration |

Run the supported cases individually:

```sh
./.gosim/gosimtool test -features rt ./examples/simple
./.gosim/gosimtool test -features rt,fs ./examples/bolt
./.gosim/gosimtool test -features rt,net ./examples/network/machines
./.gosim/gosimtool test -features rt,net ./examples/network/latency
```

Supported cases are included in the feature suites according to the table above.
The `examples/` directory itself contains documentation only.
Machine creation returns an unstarted handle: configure services, call `Start`,
and observe termination through `Wait`.

## Network latency

[network/latency](network/latency/latency_test.go) uses one persistent TCP connection.
The client sends a JSON `ping` with a sequence number and send timestamp. The
server returns a `pong` with that sequence number, the ping's receive timestamp,
and the pong's send timestamp. Connection drops are disabled (`FailProbability: 0`).

| Test | One-way latency | Simulated observation time |
| --- | --- | --- |
| `TestLatency0To5ms` | 0–5 ms | 15 seconds |
| `TestLatency50To100ms` | 50–100 ms | 15 seconds |
| `TestLatency1To2s` | 1–2 seconds | 15 seconds |

For visualization, run all three cases at normal speed (15 real seconds per
case, 45 seconds total, plus initial build/recording time):

```sh
make build
python3 examples/network/latency/visualize.py
```

The visualization first runs the deterministic tests, then displays their
recorded events at one second per simulated second. Each case prints send and
receive timestamps, measured one-way latency (`receive - send`), and round-trip
latency (`pong received - ping sent`). The two simulated machines share a clock,
so their timestamps can be compared directly. A single case can be selected:

```sh
python3 examples/network/latency/visualize.py --test TestLatency1To2s
```

Run the tests directly for fast automated checks and the same timestamped output:

```sh
./.gosim/gosimtool test -features rt,net -v ./examples/network/latency
```

Each test applies the latency range in both directions before connecting, using
Gosim's bounded exponential sampling. It sets `net.Config.ConnectionTimeout` and
the caller's dial timeout to five seconds, allowing the slowest scenario's
handshake to complete with latency enabled. The observation
window excludes connection setup and lasts exactly 15 seconds of virtual time.
Direct test execution advances virtual time immediately; the visualization
script supplies wall-clock pacing without slowing ordinary test automation.

The client sends sequential pings, pausing one second between requests. It starts a
request only when the maximum round trip fits in the remaining window, then
keeps the connection open until the window ends. It checks pong identity,
one-way and round-trip bounds, and logs each exchange plus minimum/mean/maximum RTT.
Any connection error fails the test; there are no retries or reconnections.

The [etcd examples](etcd/README.md) retain an unsupported HTTP/gRPC application as rejection coverage. They are excluded from successful simulation suites; running all examples recursively is not a supported positive test command.
