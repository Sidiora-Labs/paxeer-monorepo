# Code indexing in Hyperfide

Hyperfide `2.2.0` indexes source facts locally and records how it reached each graph
relationship. Queries report the branch, extraction coverage, and a bounded
check of the source files that contributed to the answer. A missing relation
does not prove that a call or data flow cannot occur.

Examples use `hyperfide`, the canonical command. `cg` is a deprecated
compatibility alias installed from the same executable, with identical
arguments, JSON and exit codes, kept for the compatibility window of the
approved release. Implementation paths and build commands are relative to `tools/hyperfide`.

```mermaid
flowchart LR
    A[Files and Git candidates] --> B[Change verification]
    B --> C[Bounded syntax extraction]
    C --> D[Symbols, references, bindings and type facts]
    D --> E[Scoped resolution]
    E --> F[Route and runtime relations]
    F --> G[Branch-scoped SQLite index]
    G --> H[Search, context and graph queries]
    H --> I[Coverage and source freshness]
```

## Extraction coverage

Every registered adapter is listed below with the parser identity it
records. `hyperfide info --json` and `hyperfide status --json` report that
identity per branch under `index_health.parsers` (name and file count, for
the files the index holds, at most 32 entries ordered by file count), and query answers repeat it per contributing
file in `index_health.source_validation.files[].parser`.

| Source family | Indexing mode | Parser identity | Boundary |
| --- | --- | --- | --- |
| JavaScript, JSX, TypeScript, TSX | Native syntax grammar | `tree-sitter:javascript` (`.js`, `.jsx`, `.mjs`, `.cjs`), `tree-sitter:typescript`, `tree-sitter:tsx` | Static imports, declarations, members, calls and JSX facts; computed targets remain unresolved. |
| C, C++, Objective-C, Go, Rust | Native syntax grammar | `tree-sitter:c`, `tree-sitter:cpp`, `tree-sitter:objc`, `tree-sitter:go`, `tree-sitter:rust` | Macro expansion, dynamic dispatch and unknown function-pointer targets remain boundaries. |
| Python, Ruby, PHP | Native syntax grammar | `tree-sitter:python`, `tree-sitter:ruby`, `tree-sitter:php` | Runtime attribute assignment, reflection and dynamic imports cannot be inferred from syntax alone. |
| Java, Kotlin, Scala, C#, Swift, Dart | Native syntax grammar | `tree-sitter:java`, `tree-sitter:kotlin`, `tree-sitter:scala`, `tree-sitter:csharp`, `tree-sitter:swift`, `tree-sitter:dart` | Overloads and ownership are recorded; dispatch without sufficient receiver or signature evidence stays unresolved. |
| Lua, Luau, R, Erlang, Solidity, Terraform, Nix | Native syntax grammar | `tree-sitter/lua/extended`, `tree-sitter/luau/extended`, `tree-sitter/r/extended`, `tree-sitter/erlang/extended`, `tree-sitter/solidity/extended`, `tree-sitter/terraform/extended` (`.tf`, `.tfvars`, HCL grammar), `tree-sitter/nix/extended` | Structural facts are extracted; each language's supported relation kinds depend on the syntax evidence available. |
| Pascal, COBOL, VB.NET, ArkTS | Native syntax grammar | `tree-sitter:pascal`, `tree-sitter:cobol`, `tree-sitter:vbnet`, `tree-sitter:arkts` | Error recovery can produce partial coverage; generated grammar facts do not imply runtime behavior. |
| CFScript, CFQuery | Native syntax grammar, embedded | `tree-sitter:cfscript` for `.cfs` files; inside `.cfm` and `.cfc` files the CFML adapter parses `<cfscript>` and `<cfquery>` blocks with these grammars and the file records `declarative:cfml`; a script-syntax component (a file that begins with `component`) records `embedded:cfscript` | CFQuery has no file extension of its own; SQL identifiers are recorded as references, not as resolved database objects. |
| CFML, Vue, Svelte, Astro, MyBatis XML | Embedded or declarative adapter | `declarative:cfml`, `declarative:vue`, `declarative:svelte`, `declarative:astro`, `declarative:xml` | Embedded facts retain source coordinates; only recognized script, template and mapping constructs receive relationships. |
| Liquid, Razor, Delphi forms | Text adapter | `declarative:liquid`, `declarative:razor`, `declarative:delphi_form` | Recognition is limited to supported literal constructs and references; Liquid and Razor contribute references only. |
| JSON, YAML, Twig, properties | File tracking only | `file-only:json`, `file-only:yaml`, `file-only:twig`, `file-only:properties` | These files are tracked but do not provide structural symbols or call relations. |

The grammars are vendored under `vendor/parsers/` in six families (`web`,
`systems`, `dynamic`, `managed`, `extended`, `mixed`) with one adapter
source each (`core/native/src/ast_web.c`, `core/native/src/ast_systems.c`, `core/native/src/ast_dynamic.c`,
`core/native/src/ast_managed.c`, `core/native/src/ast_extended.c`, `core/native/src/ast_mixed.c`); `core/native/src/ast.c`
holds the shared runtime. The parser identity and status are stored per
file. Native grammar support means that the file is parsed into syntax
nodes; it does not mean that every language construct is modeled. If the
syntax runtime cannot parse a file (source-size bound or parse timeout), the
`core/native/src/lang.c` fallback records the parser as `heuristic`. A file whose
extension has no language entry is tracked with parser `none`, status
`skipped` and reason `unsupported_language`. Syntax recovery, excessive depth or node count,
unreadable files, and unsupported input appear as partial, skipped, or
failed coverage with reason codes. The scanner skips files above 8 MiB; the
syntax runtime also bounds source size to 32 MiB, traversal to 500,000 nodes
and depth 512, and applies a two-second timeout to the parse stage. That
timeout is not an end-to-end indexing time limit.

## Relationships and evidence

Symbols retain their owner, qualified name, source span and available
signature, visibility, export, type and decorator facts. Import bindings
preserve local and original names, defaults, namespaces and re-exports. The
resolver uses those bindings, lexical ownership, declared or constructed
receiver types and known call arity before considering a name. Equal plausible
targets stay unresolved. An unknown receiver is an explicit boundary.

Call and other typed relations retain a source site, target when known,
strategy, provenance and evidence. `original_text` is the source-written
callee or reference name; qualifier and exact source span are separate
fields. This text is never replaced by the normalized target name.
Framework registrations become route facts. Unique handlers and literal
mounts can be connected, while competing handlers and computed paths stay
unresolved. Callback, event, queue and HTTP relations are heuristic when
they depend on registration or matching rules; their evidence records the
channel or registration site. These links are navigation aids, not proof of
all possible runtime flows.

`hyperfide flow callers <name>`, `hyperfide flow callees <name>`,
`hyperfide flow impact <name>` and `hyperfide flow paths <source> <target>`
traverse typed branch-scoped relations. The query bounds depth (`--depth`,
1 to 8), nodes (`--max-nodes`, up to 256), edges (`--max-edges`, up to 512),
elapsed time (`--max-ms`, up to 2000 ms) and output bytes (`--max-output`,
1024 to 131072); JSON includes source sites, provenance, dynamic boundaries
and truncation reasons. Over MCP the same traversal is the `flow` tool.
A missing path within those limits is not proof that no runtime path exists.
`hyperfide search` and `hyperfide context` accept `--path`, `--lang`,
`--kind` and `--name` filters; `--name` keeps symbols whose name equals the
value or contains it, ignoring case. Identifier postings make compound names
searchable by segment; when exact names and segments find nothing, a bounded
typo-tolerant fallback (at most 16 candidates) runs, and `--json` counts
the hits it contributed in `weak_matches`.

`hyperfide inspect hierarchy <name>` reports bounded ancestors, subtypes and
implementations from the typed graph. `hyperfide inspect unused` identifies
symbols with no known incoming reference and internally referenced islands
outside known entry roots. Its results carry reasons for excluding exports,
framework roots, reflection candidates, generated files and uncertain
incoming boundaries. The CLI and the MCP `inspect_hierarchy` and
`inspect_unreferenced` tools use the current branch; absence from the known
graph is never a safe-deletion verdict.

## Freshness and migrations

`hyperfide init` builds an index, `hyperfide sync` updates changed files, and
`hyperfide index --full` requests a full pass. `hyperfide info --json`,
`hyperfide status --json` and the `index_status` MCP tool show branch
coverage (`complete`, `partial`, `skipped`, `failed`, `unchecked`), parser
inventory, pending reconciliation and degraded state under `index_health`.
Code answers classify contributing files as verified, stale or unchecked in
`index_health.source_validation`. The answer check has caps of 16 files,
1 MiB per file, 4 MiB total and 25 ms, so unchecked is a real limit rather
than a freshness claim.

File changes use subsecond metadata and content verification for explicitly
dirty paths. Watcher loss creates durable reconciliation debt; the debt clears
only after successful reconciliation. Extractor output has a version
(`CG_EXTRACTION_VERSION`, currently 3) separate from the database schema
version (18). A version change re-extracts stale files, and partial or
failed passes do not stamp them current. Derived index tables may be
migrated or rebuilt; project memory, snapshots and workflow history remain
independent of derived extraction state.

Branch-specific files and graph endpoints stay isolated even when worktrees
share storage. Resolution and synthesized relations are retried when their
definitions, bindings or registrations change. A no-op sync skips those passes.

The Hyperfide control plane (`core/control-plane`, see
[the operations guide](hyperfide/operations.md)) does not change this
pipeline: each checkout or sandbox still builds and queries its own local
index with the commands above.

## Reproducing the indexing checks

`tests/integration/76_semantic_conformance.sh` runs the labelled adapter,
route and graph-convergence corpus in `tests/semantic/`. The labels
distinguish recognized syntax, resolved targets and explicitly unresolved
cases; a file being parsed is not counted as a successful relationship. Set
`CG` to the binary under test and `CG_SEMANTIC_REPORT_DIR` to keep the JSON
reports. `tests/integration/77_index_resources.sh` checks cold, no-op and
incremental operations on a small generated fixture against fixed peak-memory
and database-size limits. `tools/repository/bench-indexing.py` emits the raw timings,
peak memory, database size, diagnostics and source-to-query latency for
repeatable local inspection; its large profile is opt-in.

The conformance corpus covers adapter extraction, typed relations and framework registrations, including supported examples and decoys. It also compares graph sets across incremental and rebuild checkpoints. These are bounded fixtures rather than measurements of recall or precision on arbitrary repositories.

## Resource measurements

Performance depends on the host, fixture, worker count and SQLite version. Report those inputs with the binary digest when using `tools/repository/bench-indexing.py`. Vendored grammars are compiled into the binary and require no separate runtime parser service.
