# layerxagent

`layerxagent` contains the on-chain scheduler for EVM calls in the Paxeer X
network. It stores scheduled jobs and their escrowed deposits, executes due
calls as their creators within per-job and per-block gas limits, and settles
fees and refunds. The scheduler precompile at `0x0905` exposes scheduling,
cancellation, rescheduling, and job queries; this module supplies its keeper,
genesis handling, governance parameter updates, and native query services.

## Layout

- [`client/`](client/) — CLI queries for scheduler parameters, individual jobs, and a creator's pending job IDs.
- [`integration/`](integration/) — application tests for end-block execution, genesis export, and governance updates.
- [`keeper/`](keeper/) — persistent job indexes, deposit transfers, bounded execution, settlement payments, genesis handling, and service implementations with tests.
- [`types/`](types/) — module keys, codecs, parameter and genesis validation, message types, generated protobuf services and events, and tests.
- [`module.go`](module.go) — application module registration, query commands, gRPC gateway routes, message and query services, and genesis lifecycle.
- [`module_test.go`](module_test.go) — module registration and genesis tests, including the application's ownership of end-block execution.

## Build and test

This directory belongs to the [root Go module](../../go.mod) and uses the
repository's chain targets. Run these commands from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The [root Makefile](../../Makefile) forwards these targets to
[`chain.mk`](../../chain.mk). The build produces `build/paxd` with its native
library dependencies. The test target splits the root Go packages into groups
and runs them with race detection and coverage, including this module's CLI,
types, keeper, and integration tests. Both targets use offline Go dependency
resolution, so the dependency cache must already be populated.

## Configuration

The module receives its store, EVM keeper, upgrade plan name, and governance
authority from the application in [`node/app.go`](../../node/app.go). It reads
scheduler parameters from chain state, initialized through module genesis and
updated by an authorized `MsgUpdateParams` message. Its sources read no
environment variables or standalone configuration files.

The parameter names are `max_jobs_per_creator`, `max_schedule_horizon`,
`min_deposit_factor`, `max_jobs_per_block`, `max_job_gas_limit`,
`max_reason_bytes`, `max_call_data_bytes`, `max_block_gas`, and `paused`.
Defaults and validation are defined in [`types/params.go`](types/params.go).
Pausing stops scheduling, rescheduling, and due-job execution; cancellation
and outstanding settlement payments remain available.

## Documentation

- [Chain module parameters and messages](../../docs/system/chain-modules.md#layerxagent).
- [Scheduler precompile methods and events](../../docs/system/precompiles.md#layerxagentscheduler-abijson).
- [Documentation site module overview](../../docs/site/paxeer-docs/src/app/modules/page.tsx) and its [Markdown page](../../docs/site/paxeer-docs/public/markdown/modules.md).
- [Documentation site query service overview](../../docs/site/paxeer-docs/public/markdown/rest-grpc.md).
