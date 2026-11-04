# paxeer-monorepo — Agent Context

<!-- hyperfide-owned: graph-agent-context v1 -->

_Generated graph context owned by `hyperfide agentmd`. Regenerate with `hyperfide agentmd --write` after significant changes._

Workflow: the single source is `spec/workflow.kvx` (rules, loop and policies); read it there. Active feature: `spec/layerx-merge/spec.kvx`.

## Merged network

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel is part of the network and participates in block proposal and execution;
a receipt is final when its block is committed. The node's
[proposal handler](../node/app.go) validates sequence batches, and
[`HostExecute`](../modules/lxphost/host.go) runs kernel activities through cgo
against the chain's transaction store.

Public JSON-RPC uses the root path of `https://api-mainnet-beta.paxeer.network`
and `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`; see the
[public RPC reference](../docs/site/docs/reference/public-rpc.md).
Hosted gateway `px_` methods are declared in the
[gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
Sub-chains are tenant domains in the database. The
[audit brief](../spec/layerx-merge/programme/audit/BRIEF.kvx) records these
network facts for documentation work.

## Graph coverage

The generated inventory below reflects the local graph filtered by
[`.cgignore`](../.cgignore). This checkout indexes specification tooling;
the language totals, directory map, entry points and reference counts describe
that indexed subset, not the whole monorepo. Build metadata is discovered at
the repository root. Regenerate the inventory after changing graph coverage;
do not copy counts from an older snapshot.

## Languages

| Language | Files | Lines |
|---|---:|---:|
| python | 85 | 7815 |
| json | 35 | 2196 |

120 source files, 10011 lines total.

## Directory map

- `spec/` — 120 files, 10011 lines (mostly python)

## Build & tooling

- `Makefile` — make
- `package.json` — npm/node (scripts: `build` `build:dependencies`)
- `go.mod` — Go modules
- `foundry.toml` — Foundry (Solidity)

## Entry points

- function `main` — `spec/layerx-merge/programme/audit_check.py:70`
- function `main` — `spec/layerx-merge/programme/chains_closure.py:26`
- function `main` — `spec/layerx-merge/programme/closure_check.py:43`
- function `main` — `spec/layerx-merge/programme/dependabot/check.py:42`
- function `main` — `spec/layerx-merge/programme/docs_check.py:48`
- function `main` — `spec/layerx-merge/programme/interfaces_reconcile.py:222`
- function `main` — `spec/layerx-merge/programme/issues/check.py:187`
- function `main` — `spec/layerx-merge/programme/names_closure.py:26`
- function `main` — `spec/layerx-merge/programme/parity_stale_check.py:18`
- function `main` — `spec/layerx-merge/programme/proofs_closure.py:26`
- function `main` — `spec/layerx-merge/programme/reconcile_check.py:148`
- function `main` — `spec/layerx-merge/programme/sweeps_summary.py:193`

## HTTP routes

The graph's route below comes from a source string in an interface-reconciliation
test fixture. It is not a deployed network endpoint; use the public RPC reference
above for client configuration.

| Method | Pattern | Handler | Where |
|---|---|---|---|
| GET | `/live` | `handler` | `spec/layerx-merge/programme/tests/test_interfaces_reconcile.py:42` |

## Load-bearing symbols (most referenced)

- `load` (function, 95 refs) — `spec/layerx-merge/programme/kvx.py:63`
- `errors` (variable, 60 refs) — `spec/layerx-merge/programme/issues/check.py:26`
- `run` (function, 49 refs) — `spec/layerx-merge/programme/evidence.py:15`
- `KvxError` (class, 41 refs) — `spec/layerx-merge/programme/kvx.py:11`
- `err` (function, 41 refs) — `spec/layerx-merge/programme/issues/check.py:29`
- `CardError` (class, 28 refs) — `spec/layerx-merge/programme/runcards.py:12`
- `ROOT` (variable, 18 refs) — `spec/layerx-merge/programme/dependabot/check.py:6`
- `ParityError` (class, 17 refs) — `spec/layerx-merge/programme/parity.py:72`
- `CHECKER` (variable, 15 refs) — `spec/layerx-merge/programme/tests/test_names_closure.py:8`
- `SPEC_PATH` (variable, 13 refs) — `spec/layerx-merge/programme/services.py:12`
- `load_index` (function, 13 refs) — `spec/layerx-merge/programme/decisions.py:21`
- `text_key` (function, 13 refs) — `spec/layerx-merge/programme/issues/check.py:70`
- `check_against_spec` (function, 12 refs) — `spec/layerx-merge/programme/runcards.py:57`
- `load_card` (function, 12 refs) — `spec/layerx-merge/programme/runcards.py:16`
- `main` (function, 12 refs) — `spec/layerx-merge/programme/closure_check.py:43`

## Querying this codebase

This project is indexed by Hyperfide (SQLite + FTS5, 100% local). Prefer these over grep/file-walking — one call returns definitions, snippets, and call edges:

```bash
hyperfide context <query>      # symbols + snippets + callers/callees + routes
hyperfide search <text>        # instant name/full-text search
hyperfide grep <pattern> [-i]  # line search, each hit in its symbol
hyperfide files <glob|words>   # find files; --dirs maps directories
hyperfide symbol <name>        # definition + snippet + reference count
hyperfide impact <name> -d 3   # who breaks if this changes
hyperfide routes [filter]      # URL pattern -> handler
hyperfide changes              # impact radius of uncommitted edits
```

`cg` remains a compatibility alias for every command above (`cg context <query>` works unchanged).

All of the above accept `--json`. The graph auto-syncs via `hyperfide watch`, or connect over MCP with `hyperfide mcp-install`.

Over MCP, use `grep_code`, `find_files`, `search_code`, and `get_context` before any shell search: `hyperfide grep` and `hyperfide files` answer what grep, rg, and find would, ranked and capped to a token budget.

When another process (an editor's indexer) holds the database, `hyperfide remember`, `hyperfide handoff`, and the bookkeeping behind `hyperfide spec done` say **queued**: the write is kept in `.codegraph/journal/` and the next `hyperfide` command applies it — do not re-run it. Exit 75 (a claim) means nothing changed: re-run the same command. `hyperfide journal` lists what is queued.
