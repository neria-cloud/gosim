# Unsupported etcd integration

These fixtures retain an application that depends on HTTP/gRPC translation behavior outside the current simulator's supported boundary. They are not a working etcd simulation example, and native host networking must not be used to make them pass.

The candidate boundary suite, run through `make test-large`, verifies that translation rejects this application with an explicit unsupported-package diagnostic. Its expected rejection is not a successful application execution. See the [scope table](../../README.md#current-scope) and [syscall audit](../../docs/reports/syscall-support-for-gquorum.md) for additional model limitations.
