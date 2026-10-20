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

The [etcd examples](etcd/README.md) retain an unsupported HTTP/gRPC application as rejection coverage. They are excluded from successful simulation suites; running all examples recursively is not a supported positive test command.
