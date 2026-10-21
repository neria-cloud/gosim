# Pinned etcd restoration contract

This is the finite first admission frontier for the `examples/etcd` source graph.
The source is the root module at Go 1.25.14 and 1.26.9, with etcd 3.5.17,
bbolt 1.3.11, JWT v4.5.1, jsoniter v1.1.11, reflect2 v1.0.1, and
OpenTelemetry SDK v1.20.0. `go mod verify` succeeds under both compilers.
Existing `go.sum` checksums and module pins remain the authority; do not fork or
upgrade dependencies while restoring this scenario.

The initial profile traversal started at the sim-only example, stopped at existing bridge
and native-pure boundaries, and yielded the same five rejected application
packages for both families. Those packages now have exact, limited replacement
rules in the Go 1.25 baseline, inherited by Go 1.26. The Go 1.26 raw-list-only
`internal/runtime/syscall/linux` item is removed by the actual syscall hook
composition; it is not a newly authorized package. The first translator error
at that frontier was `crypto/ecdsa` through etcd transport. This inventory
records the admission frontier; typed loading and simulation execution establish
the current required method bodies.

| Package | Pinned source reference | Required declarations | Disposition |
| --- | --- | --- | --- |
| `crypto/elliptic` | etcd client pkg transport `listener.go` | `Curve`, `CurveParams`, `P521() Curve`; curve methods and other P-curves needed by certificate/X509 type declarations | Type metadata limited; active curve arithmetic guarded until explicitly modeled |
| `crypto/ecdsa` | transport `listener.go`, etcd auth/JWT, golang-jwt v4 `ecdsa*.go` | `PublicKey` with embedded `elliptic.Curve`, `X`, `Y`; `PrivateKey` with embedded `PublicKey`, `D`; `GenerateKey`, `Sign`, `Verify`, `Public`, signer methods and X509 parsing result types | Key shapes modeled; generation/sign/verify guarded; plaintext path must not call them |
| `crypto/rsa` | etcd auth/JWT, golang-jwt v4 `rsa*.go` | `PublicKey{N,E}`, `PrivateKey` and CRT fields, `PSSOptions`, salt constants, PKCS1v15/PSS sign and verify, `Public`, `Size`, signer methods | Key shapes modeled; algorithms guarded; plaintext auth disabled |
| `runtime/trace` | OpenTelemetry SDK `trace/span.go` | `IsEnabled() bool`, `NewTask(context.Context,string) (context.Context,*Task)`, `(*Task).End()`; rest of exported trace surface only as required by typed imports | `IsEnabled` returns false; active task, region, log and trace writing abort |
| `github.com/modern-go/reflect2` | etcd v2 discovery → client v2 → jsoniter v1.1.11 | Full `Type`, kind interfaces, `Config`, `API`; `TypeOfPtr`, `TypeOf`, `Type2`, `PtrTo`, `DefaultTypeOfKind`, `IsNil`, `IsNullable`; distinct exported concrete `UnsafePtrType`, `UnsafeIFaceType`, `UnsafeSliceType`, `UnsafeArrayType`, `UnsafeMapType`, `UnsafeStructType` | Initialization type metadata modeled; raw pointer/value mutation operations guarded |

The additional concrete reflect2 names are necessary because jsoniter uses
type assertions and pointer fields against them. jsoniter package
initialization calls `TypeOfPtr(...).Elem()` for JSON RawMessage, interfaces,
numeric types, string and marshal interfaces; these calls must return real,
canonical, comparable type wrappers. Its encoding/decoding paths call
`RTypeOf`, `PtrOf`, `Unsafe*` methods and concrete type assertions. Whether
those paths execute in the accepted plaintext cluster is an execution fact:
the final three-node and repeated-worker runs on both compilers passed without
them. They remain fatal guards, not success-shaped pointer stubs.

`crypto/ecdsa.GenerateKey(elliptic.P521(), rand.Reader)` is in transport's
self-signed certificate branch. JWT signing and verification are registered
at initialization but require auth to be enabled. OpenTelemetry's
`recordingSpan.runtimeTrace` checks `IsEnabled` before `NewTask`. The positive
plaintext runs on both compilers establish that their guarded branches stayed
inactive for the accepted scenario.

Pinned jsoniter also imports `github.com/modern-go/concurrent`, whose dormant
panic handler calls `runtime/debug.Stack`. The selected debug facade provides
that declaration with a fatal execution guard; the disabled path does not
collect a host stack. Prometheus stores `runtime.ReadMemStats` in a typed
`func(*runtime.MemStats)` field at initialization, so the runtime bridge keeps
that exact signature and aborts any actual host metric collection.

The pinned transport and JWT source also names X.509 functions which the
existing facade did not fully declare: `CreateCertificate`,
`MarshalECPrivateKey`, `ParseCRL`, `ParseECPrivateKey`,
`ParsePKCS1PrivateKey`, `ParsePKIXPublicKey`, and `DecryptPEMBlock`.
`ParsePKCS1PrivateKey` must return `*rsa.PrivateKey`, not `any`; the JWT source
assigns it to an interface variable but other typed callers may use the exact
return type. These are declaration-only optional paths with fatal execution
guards. `ParsePKCS8PrivateKey`, `ParseCertificate`, and `ParseCertificates`
already have guarded facade declarations.

The existing sim test names are `TestEtcd`, `TestSnapshotter`, and `TestWal`.
The target controller requires readiness from three independently configured
machines, a successful put/get before partition, a bounded seeded partition,
post-repair put/get progress, exact snapshot/WAL round trips, and graceful
cleanup. `RunEmbedded(ctx,cfg,ready)` and `nodeHandle` have their signatures in
plan A5. Fixture homes are: `examples/etcd` for real startup and storage,
`internal/compat/shims` for recovery-resistant guarded operations,
`internal/tests/net` for translated HTTP, and `internal/tests/integration` for
same-worker protobuf/replay coverage. Positive etcd tests are now in the
ordinary `rt,fs,net` feature registry.

The baseline profile uses exact package/bridge records and `limited`
capabilities for guarded executable symbols. Unknown crypto, private reflection
and active tracing stay rejected or abort on execution. Go 1.26 inherits the
common declarations; only verified signature or behavior differences belong in
its delta. Both Linux source architectures pass independent generator checks.
