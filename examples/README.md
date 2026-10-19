# Examples

Run the supported examples with an exact-patch CLI and explicit features:

```sh
./.gosim/gosimtool test -go go1.25.14 -features rt,fs,net ./examples ./examples/bolt
```

Normal commands require matching certified source/tuple evidence. On a pending development checkout, insert `internal-candidate` before `test`; see the [development workflow](../README.md#development-and-certification).

The root examples exercise deterministic tests and explicit machine startup. The Bolt example exercises the existing file/persistence model. Both are included in the full-feature positive suite. Machine creation returns an unstarted handle: configure services, call `Start`, and observe termination through `Wait`.

The [etcd examples](etcd/README.md) retain an unsupported HTTP/gRPC application as rejection coverage. They are excluded from successful simulation suites; running all examples recursively is not a supported positive test command.
