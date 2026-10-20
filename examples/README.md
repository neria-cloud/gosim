# Examples

From the repository root, build the command binaries from the current checkout, then run
the supported examples:

```sh
make build
./.gosim/gosimtool test -features rt,fs,net ./examples ./examples/bolt
```

Go tooling selects the compiler from the caller's module/workspace and
`GOTOOLCHAIN`. The launcher automatically builds or reuses a matching worker for
supported Go 1.25.* and 1.26.* releases. No compiler pin or certification is
required. See [Go versions and configuration](../README.md#go-versions-and-configuration).

`make build` creates `.gosim/gosimtool`, `.gosim/gosim-worker`, and
`.gosim/gosim-gen`. Use `gosimtool` for normal commands.

If an existing `.gosim/gosimtool` reports `unsupported config schema 2`, run
`make build` to replace the old CLI. Keep the current `gosim.json`.

The root examples exercise deterministic tests and explicit machine startup. The Bolt example exercises the existing file/persistence model. The root examples are included in the full-feature positive suite; the command above also runs the separate Bolt example. Machine creation returns an unstarted handle: configure services, call `Start`, and observe termination through `Wait`.

## Network latency

[latency_test.go](latency_test.go) uses one persistent TCP connection to an echo
server, with connection drops disabled (`FailProbability: 0`).

| Test | One-way latency | Simulated observation time |
| --- | --- | --- |
| `TestLatency0To5ms` | 0–5 ms | 15 seconds |
| `TestLatency50To100ms` | 50–100 ms | 15 seconds |
| `TestLatency1To2s` | 1–2 seconds | 15 seconds |

Run only these examples:

```sh
make build
./.gosim/gosimtool test -features rt,net -v -run '^TestLatency' ./examples
```

Each test connects with zero latency, then applies the latency range in both
directions using Gosim's bounded exponential sampling. This avoids the current
one-second connection-opening timeout in the slowest scenario. The observation
window excludes connection setup and lasts exactly 15 seconds of virtual time,
not wall-clock time.

The client sends sequential echoes, pausing 100 ms between requests. It starts a
request only when the maximum round trip fits in the remaining window, then
keeps the connection open until the window ends. It checks echoed data and
round-trip bounds and logs completed echoes and minimum/mean/maximum latency.
Any connection error fails the test; there are no retries or reconnections.

The [etcd examples](etcd/README.md) retain an unsupported HTTP/gRPC application as rejection coverage. They are excluded from successful simulation suites; running all examples recursively is not a supported positive test command.
