# LayerX governance

This directory holds the `LayerXProposal` content type, governance execution
handler, and submission client used by `paxd` on the Paxeer network. Proposals
carry an allowlisted set of typed module messages whose authority must be the
governance module account. The handler dispatches them through the SDK
`MsgServiceRouter` in a cached context and commits state changes and events only
when every message succeeds. Target modules enforce their own validation and
retirement rules.

## Layout

- [`client/`](client/) — `cli/tx.go` implements `layerx-proposal` submission from a JSON file and the proposal REST handler.
- [`types/`](types/) — proposal validation, message allowlisting, codec registration, generated protobuf serialization, module keys, scoped pause checks, and their tests.
- [`handler.go`](handler.go) — `NewProposalHandler` validates and atomically executes proposal messages.
- [`handler_anchor_retired_test.go`](handler_anchor_retired_test.go) — exercises anchor parameter updates through governance and their refusal after merge activation.
- [`handler_pause_live_test.go`](handler_pause_live_test.go) — exercises ledger and authorization pause updates through the application's governance route.
- [`handler_pay_pause_live_test.go`](handler_pay_pause_live_test.go) — exercises payment pause updates, authority validation, and rollback when a later message fails.

## Build and test

This package belongs to the repository's root [Go module](../../go.mod) and uses
the root [Makefile](../../Makefile) and [chain.mk](../../chain.mk). Run from the
repository root:

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds `paxd`, including this package, and prepares its native
libraries. `paxeer-test` runs the chain's Go package test groups, including this
directory's handler and proposal-type tests.

## Configuration

The package has no dedicated configuration file or environment variables.
The `layerx-proposal` client reads the JSON proposal file supplied as its
positional argument and uses `--deposit` and `--from` for submission. Proposal
content contains `title`, `description`, and `messages`; each carried message
must use the authority returned by `GovernanceAuthority()`.

## Documentation

- [Governance proposal types](../../docs/system/governance.md)
- [Chain modules and parameters](../../docs/system/chain-modules.md)
- [Governance on the documentation site](../../docs/site/paxeer-docs/src/app/governance/page.tsx)
- [Chain modules on the documentation site](../../docs/site/paxeer-docs/src/app/modules/page.tsx)
