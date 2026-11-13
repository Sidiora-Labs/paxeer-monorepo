# Source reference

Hyperfide's implementation lives under `tools/hyperfide`. The paths below link
to the source for the local graph, distributed control service and platform surfaces.
The [architecture](ARCHITECTURE.md) describes their behavior; the
[test reference](TEST-REFERENCE.md) lists test and fixture declarations.

| Area | Source |
| --- | --- |
| CLI dispatch and help | [main.c](../../tools/hyperfide/core/native/src/main.c), [help.c](../../tools/hyperfide/core/native/src/help.c) |
| Database and indexing | [db.c](../../tools/hyperfide/core/native/src/db.c), [scan.c](../../tools/hyperfide/core/native/src/scan.c), [walk.c](../../tools/hyperfide/core/native/src/walk.c) |
| Parsing and semantics | [native source directory](../../tools/hyperfide/core/native/src/), including `ast*.c`, `semantic.c` and `sem_*.c` |
| Graph queries | [graph.c](../../tools/hyperfide/core/native/src/graph.c), [grep.c](../../tools/hyperfide/core/native/src/grep.c), [files.c](../../tools/hyperfide/core/native/src/files.c) |
| Version control | [vcs.c](../../tools/hyperfide/core/native/src/vcs.c), [gitint.c](../../tools/hyperfide/core/native/src/gitint.c), [projection.c](../../tools/hyperfide/core/native/src/projection.c) |
| Spec and documentation | [spec.c](../../tools/hyperfide/core/native/src/spec.c), [docs.c](../../tools/hyperfide/core/native/src/docs.c), [kvx.c](../../tools/hyperfide/core/native/src/kvx.c) |
| Fleet and agents | [fleet.c](../../tools/hyperfide/core/native/src/fleet.c), [orchestrate.c](../../tools/hyperfide/core/native/src/orchestrate.c), [native.c](../../tools/hyperfide/core/native/src/native.c) |
| Native components | [component sources](../../tools/hyperfide/core/native/src/hyperfide/), [component fragments](../../tools/hyperfide/core/native/src/hyperfide/components/) |
| Distributed control service | [TypeScript sources](../../tools/hyperfide/core/control-plane/src/), [component fragments](../../tools/hyperfide/core/control-plane/components/) |
| KVX reference implementation | [Go implementation](../../tools/hyperfide/core/kvx/impl/go/) |
| Platform surfaces | [apps](../../tools/hyperfide/apps/) |
| VS Code extension | [extensions/vscode](../../tools/hyperfide/extensions/vscode/) |
| Repository tooling | [tools/repository](../../tools/hyperfide/tools/repository/), [deployment helpers](../../tools/hyperfide/deploy/hyperfide/) |

The [Makefile](../../tools/hyperfide/Makefile) selects the C sources and native
component inputs for `hyperfide` and its `cg` compatibility executable.
`make hyperfide-services` builds the separate `@hyperfide/control-services`
package. Its [package manifest](../../tools/hyperfide/core/control-plane/package.json)
declares its build and check scripts.
