# Plaintext etcd simulation

This example runs a real three-node embedded etcd 3.5.17 cluster on Gosim's
simulated TCP network and filesystem. `TestEtcd` requires successful put/get
round trips before a seeded network partition and after the partition is
repaired. `TestSnapshotter` checks saved snapshot metadata and bytes;
`TestWal` checks state and entries after closing and reopening the WAL.

From the repository root:

```sh
make build
./.gosim/gosimtool test -features rt,fs,net -seeds 1-3 -v ./examples/etcd
```

Stable Go 1.25.* and 1.26.* releases are supported. Go tooling selects the
compiler; `GOTOOLCHAIN` can select an exact patch. The full-feature ordinary
`make test` suite includes these tests. A native integration metatest also runs
`TestEtcd` with seeds 1, 2, 3, then 1 in one worker and compares seed 1 with a
fresh matching worker.

The fixture uses explicit machine lifecycle, static plaintext listeners, bounded
simulated deadlines and per-node data directories. It disables auto-TLS,
tracing export, pprof and metrics scraping. It does not exercise certificate
algorithms, JWT authentication, etcd v2 discovery, unsafe reflect2 value
operations or host runtime metrics. Those optional declarations remain guarded
and abort if executed. The filesystem's current lock stubs do not establish
contention or crash-durability guarantees beyond the snapshot/WAL checks here.
