# xweb

This directory holds the Cosmos SDK module for attested web data in Paxeer X. Its keeper records requests, verifies attestor signatures, stores results, and distributes fees for EVM contracts and programs within the same `paxd` network. The [xweb precompile](../../precompiles/xweb) exposes contract requests, fulfilment, refunds, and result reads; the module also registers authority messages and gRPC queries. Contract requests support fetch, search, API, and storage receipts; program requests support fetch and search. Results become final when their block is committed.

## Layout

- [client/](client/) — query and authority transaction commands under `client/cli`.
- [keeper/](keeper/) — request admission, signature verification, fee transfers, refunds, program results, queries, and genesis import and export.
- [testutil/](testutil/) — application-backed keeper stores and attestor signing helpers for tests.
- [types/](types/) — state and genesis types, message and query bindings, payload and observation codecs, attestation digests, and test vectors.
- [module.go](module.go) — module registration, services, invariants, CLI commands, and genesis entry points.
- [ATTESTATION.md](ATTESTATION.md) — signing preimage, signature rules, supported request kinds, and shared vectors.
- [module_test.go](module_test.go) — genesis validation and program-state round-trip tests.
- [module_services_test.go](module_services_test.go) — query service registration test.
- [merge_plan_test.go](merge_plan_test.go) — program-origin activation and persisted result tests through the node upgrade plan.

## Build and test

The module belongs to the root [go.mod](../../go.mod). Build it as part of the node using the root [Makefile](../../Makefile) and [chain.mk](../../chain.mk). Run commands from the repository root:

```sh
make paxd-build
```

This target builds the native libraries and the `build/paxd` binary. For the module and precompile tests, build the native libraries and run the command held in [scripts/ci/xweb-workflow-check.sh](../../scripts/ci/xweb-workflow-check.sh) and the [xweb workflow](../../.github/workflows/xweb-test.yml):

```sh
make paxd-native-libs
go test -count=1 ./modules/xweb/... ./precompiles/xweb/...
```

The tests cover keeper state, authority messages, queries, payload encodings, attestations, genesis round trips, and precompile integration.

## Configuration

The module receives configuration through its genesis state and authority messages; its runtime sources read no module-specific environment variables or configuration files. [types/state.go](types/state.go) defines the parameter names `authority`, `fee`, `max_payload_bytes`, `max_callback_gas`, `timeout_blocks`, `kernel_network_id`, and `program_fee_denoms`. [types/genesis.go](types/genesis.go) also carries `attestors` and `paused` state. The default genesis is paused with an empty attestor set.

Authority commands in [client/cli/tx.go](client/cli/tx.go) register and remove attestors, set the threshold and parameters, and pause or unpause the module. The `set-params` command requires `--kernel-network-id` and `--program-fee-denoms`. Program requests and fulfilments also require activation of the `v7.0` upgrade, checked in [keeper/activation.go](keeper/activation.go).

## Documentation

- [Chain modules and parameters](../../docs/system/chain-modules.md) — module parameters and messages.
- [Precompile reference](../../docs/system/precompiles.md) — xweb methods and Solidity interface.
- [Web Search and xweb](../../docs/site/paxeer-docs/src/app/xweb/page.tsx) — documentation site page for content retrieval, attestation, and integration.
- [Web Search and xweb Markdown](../../docs/site/paxeer-docs/public/markdown/xweb.md) — the site's Markdown page.
