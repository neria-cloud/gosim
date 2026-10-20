# Feature verification

`suites.go` is the package applicability registry used by the candidate probes and internal certification command. Packages are selected before translation and initialization:

| Tuple | Positive packages |
| --- | --- |
| rt | internal/tests/rt, examples/simple |
| rt,fs | internal/tests/rt, internal/tests/fs, examples/simple, examples/bolt |
| rt,net | internal/tests/rt, internal/tests/net, examples/simple, examples/network/machines, examples/network/latency |
| rt,fs,net | all four feature packages, nemesis, examples/simple, examples/bolt, examples/network/machines, examples/network/latency |

Example cases are separate packages, selected by their required features.
The unsupported `examples/etcd` integration remains separate rejection coverage.

Each positive package retains native metatests for deterministic replay, seeds and execution. Expected application failures have separate native assertions. The race oracle runs only for full-feature race tuples, with its native harness checking intentional race reports. HTTP/gRPC are explicit translation rejections; the retained ignored source examples do not count as simulated coverage. The old commented nemesis behavior files remain preserved in integration; the active nemesis package runs on full-feature tuples.

`inventory.json` maps all 207 declared baseline behavior tests to their new files and build constraints. An AST comparison against the Task 24 commit confirmed identical function bodies immediately after moving. The fs syscall trace expectation was then adjusted to omit the hostname init call that now belongs to rt; filesystem syscall assertions remain intact. `inventory_test.go` prevents declarations or their applicability constraints from silently disappearing. `suite-moves.json` records the default file destinations; mixed crash/logging declarations are individually mapped in the test inventory.

Candidate workers additionally build real isolated disabled-fs/net packages and execute init and test cases for every disabled feature. They verify non-recoverable failure, diagnostic ownership/callers, and absence of host filesystem/network effects. Every built artifact undergoes compiler dependency-graph checks, including inactive version families and provider boundaries. Cache checks use real artifacts and changed normalized identities. The same native worker patch executes all four feature sets; the common commit gate invokes both exact patches independently.

Full-feature workers also build an isolated copy of the framework with test-only lifecycle injection. The copy has its own content identity. Private fakes and one route exist only in that copy, never in production or its published artifacts. Translated scenarios run on the generated driver's existing supervisor and engine, reserve disjoint fixture machine IDs, and close their owned kernel sessions. They test startup/publication/rollback/cleanup failures, exhaustion, invocation cancellation, poll generations and independent random streams. Host assertions require both the intended result and logged disposal of descriptors, transactions, polls and leases. Real public-API suites cover network restart/endpoint reuse, blocked-read cancellation, snapshot ownership/limits, root/self transitions and init failures.

Reduced certification requires build, execution, applicable native metatests and boundary-check evidence. Full certification requires build, execution and the complete native suite (which includes the candidate worker). Reports remain pending; this test registry does not promote compatibility tuples.
