# layerx-merge specification v2: documentation index

Index of every artifact delivered under `spec/layerx-merge/` for the layerx-merge specification v2, built from the merged fragments at repository revision `2a1033aa2`. Links are relative to `spec/layerx-merge/docs/`. Nothing here is evidence that a task ran: every task is pending and every acceptance criterion is UNRUN.

## Specification files

| File | Lines | Blocks | Content |
|---|---|---|---|
| [spec.kvx](../spec.kvx) | 170,345 | 15,350 | every block except interface blocks; the file the Hyperfide CLI (`cg`) reads |
| [interfaces.kvx](../interfaces.kvx) | 774,914 | 77,723 | the interface blocks, same grammar as spec.kvx; tasks cite them in their interfaces key |
| [qualification.kvx](../qualification.kvx) | 31 | 0 | qualification ledger: header with the finding format of spec/workflow.kvx [qualification_log], no findings, and the superseded-finding list of the previous ledger |
| [design.body.md](../design.body.md) | 449 | - | Merged design body included by the [design] block of spec.kvx: domains, surfaces and the planned changes, every citation verified at the pinned revision. |

Blocks of spec.kvx by kind:

| Kind | Blocks |
|---|---|
| acceptance | 4,025 |
| task | 2,545 |
| capability | 2,450 |
| req | 2,388 |
| override | 1,333 |
| decision | 615 |
| capability_group | 599 |
| resolution | 599 |
| wiring | 536 |
| contract_context | 200 |
| service | 40 |
| feature | 17 |
| meta | 1 |
| design | 1 |
| execution | 1 |

### Why interface blocks are in interfaces.kvx

Interface blocks moved out of spec.kvx because the Hyperfide CLI parser is quadratic in block count: `cg spec status` takes 40 s on 8.5k blocks, 605 s on 48.5k blocks, more than 30 min on the 93k-block merged document, and 216 s on the 15.2k-block spec.kvx. The CLI never consumed interface blocks. Interface blocks remain the authority for interfaces, the specification build validates both files exactly as one document, and `contracts/*.md` tabulate the interfaces. Do not merge the two files.

## Documentation (docs/)

| File | Lines | Description |
|---|---|---|
| [README.md](README.md) | - | This index. |
| [BUILD-REPORT.md](BUILD-REPORT.md) | - | Final build report: build result, block and line counts of both files, the Hyperfide CLI acceptance result and the recorded CLI defects. |
| [architecture/clients-and-sdks.md](architecture/clients-and-sdks.md) | 3,466 | Schematic of the six applications and the JavaScript, wallet, launchpad, platform, agent and programs SDKs with their pending migrations. |
| [architecture/kernel-and-chain.md](architecture/kernel-and-chain.md) | 292 | Schematic of the C kernel, the cgo host and the chain modules at HEAD, separated from the changes the tasks specify. |
| [architecture/markets-and-launchpad.md](architecture/markets-and-launchpad.md) | 21,710 | Schematic of the perps, spot, oracle, pricefeed and launchpad surfaces, baseline edges and planned edges. |
| [architecture/sequencer-and-consensus.md](architecture/sequencer-and-consensus.md) | 304 | Schematic of the sequencer, receipts and consensus at HEAD and the target behaviour of the pending tasks. |
| [architecture/services.md](architecture/services.md) | 1,225 | Projection of every [service.*] block: purpose, route, inputs, outputs, state, dependencies, failure policy and owning tasks. |
| [architecture/system-overview.md](architecture/system-overview.md) | 591 | Schematic of the 17 specification domains, the source trees each owns and the merge work declared across them. |
| [plans/activation-and-migration.md](plans/activation-and-migration.md) | 312 | Activation boundary, the evidence required to cross it, migration rehearsal, retirement order and recovery per wave. |
| [plans/interface-matrix.md](plans/interface-matrix.md) | 77,890 | Interface matrix: every effective interface declaration joined to the tasks that name its id, by domain, kind and status. |
| [plans/orchestration.md](plans/orchestration.md) | 200 | Execution plan for the orchestrator, seventeen feature managers and up to 500 task sandboxes on one task board. |
| [plans/test-plan.md](plans/test-plan.md) | 705 | Test plan: one acceptance criterion, one test selector and one owning task, with counts per domain and gate kind. |
| [plans/wave-plan.md](plans/wave-plan.md) | 954 | Wave plan: the pending tasks per wave 0-7 and domain, same-wave admission by requires and the wave gates. |
| [decisions/INDEX.md](decisions/INDEX.md) | 621 | Index of all 615 decision blocks with title, affects and resolution count. |
| `decisions/D<N>.md` (615 files, D1 to D1791) | - | One page per decision block: title, value, basis, affects and the resolution blocks that name it. |

## Contracts (contracts/)

Normative files freeze interfaces by hand; generated files freeze the interface blocks, contexts, decisions and acceptance criteria that cite them. Counts are the blocks whose contract key names the file.

| File | Interfaces | Acceptance | Description |
|---|---|---|---|
| [README.md](../contracts/README.md) | - | - | Contracts index: version, change control, normative and generated files, retired files. |
| [address-table.md](../contracts/address-table.md) | 131 | 6 | Normative: Precompile address table, method selectors, LXP_ error-code registry (D6) and activation gates of the merged chain in the layout the code builds and tests against (services/agent/crates/layerx-types/build.rs, scripts/ci/result-codes-gen.py, precompiles/common/lxp_errors_test.go, cmd/layerxctl/tests/files.rs, the wallet-gateway and SDK registry tests); sections (a), (a2), (b) and (c) change only by amendment. |
| [address-inventory.md](../contracts/address-inventory.md) | 2,607 | 218 | Generated: Precompile and agent-function address table, per-address methods, selectors, events and errors with status, activation gates, LXP_ error registry, the superseded table's promises not served at the source revision, and the domain-6 surfaces (Solidity, proto, Msg, CLI, config, formats) whose interfaces name it. |
| [agent-api.md](../contracts/agent-api.md) | 201 | 88 | Generated: freezes proto_message, http_route, error_code interfaces under `services/agent/schema`, `services/agent/crates`. |
| [agent-cli.md](../contracts/agent-cli.md) | 57 | 18 | Generated: freezes cli_command interfaces under `services/agent/crates`, `services/agent/tools`. |
| [agent-config.md](../contracts/agent-config.md) | 203 | 12 | Generated: freezes env_var, config_key interfaces under `services/agent/crates`, `services/agent/tests`. |
| [agent-errors.md](../contracts/agent-errors.md) | 129 | 5 | Generated: freezes error_code interfaces under `services/agent/crates`, `services/agent/schema`. |
| [agent-formats.md](../contracts/agent-formats.md) | 32 | 15 | Generated: freezes file_format interfaces under `services/agent/crates`, `services/agent/schema`. |
| [agent-functions.md](../contracts/agent-functions.md) | 237 | 58 | Normative: Agent functions 0x0901-0x0908 ABI, authority, effects and gas. |
| [agent-http.md](../contracts/agent-http.md) | 0 | 5 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [agent-human-peer.md](../contracts/agent-human-peer.md) | 61 | 0 | Generated: freezes proto_message interfaces under `services/agent/crates`. |
| [agent-lni.md](../contracts/agent-lni.md) | 55 | 2 | Generated: freezes proto_message, c_symbol interfaces under `services/agent/schema`, `services/agent/include`. |
| [agent-mcp.md](../contracts/agent-mcp.md) | 44 | 0 | Generated: freezes http_route, error_code, jsonrpc_method interfaces under `services/agent/crates`. |
| [agent-metrics.md](../contracts/agent-metrics.md) | 0 | 2 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [agent-observability.md](../contracts/agent-observability.md) | 12 | 0 | Generated: freezes metric interfaces under `services/agent/crates`. |
| [agent-python-sdk.md](../contracts/agent-python-sdk.md) | 0 | 2 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [agent-rust-api.md](../contracts/agent-rust-api.md) | 3,366 | 113 | Generated: freezes rust_pub_item interfaces under `services/agent/crates`. |
| [agent-ts-sdk.md](../contracts/agent-ts-sdk.md) | 706 | 32 | Generated: freezes ts_export interfaces under `services/agent/sdk`. |
| [bridge-client.md](../contracts/bridge-client.md) | 366 | 77 | Generated: freezes ts_export, env_var, cli_command interfaces under `apps/bridge/src`, `apps/bridge/instrumentation.node.ts`. |
| [bridge-deploy.md](../contracts/bridge-deploy.md) | 123 | 11 | Generated: freezes go_exported, env_var, cli_command interfaces under `services/bridge/deploy`. |
| [bridge-solana.md](../contracts/bridge-solana.md) | 55 | 0 | Generated: freezes error_code, rust_pub_item, file_format interfaces under `contracts/bridge/solana`. |
| [bridge-vectors.md](../contracts/bridge-vectors.md) | 50 | 0 | Generated: freezes go_exported, file_format interfaces under `contracts/bridge/vectors`, `contracts/bridge/evm`. |
| [clients.md](../contracts/clients.md) | 8,742 | 247 | Normative: Client operations, identity and keys, receipt verification, lx_* decisions and conformance. |
| [consensus-fork.md](../contracts/consensus-fork.md) | 1,590 | 46 | Generated: freezes go_exported, proto_message, metric interfaces under `consensus/proto/tendermint`, `consensus/internal/p2p`. |
| [cosmos-fork.md](../contracts/cosmos-fork.md) | 3,942 | 89 | Generated: freezes go_exported, ts_export, rust_pub_item interfaces under `sdk/x/staking`, `sdk/chainflow/typescript`. |
| [cosmwasm.md](../contracts/cosmwasm.md) | 377 | 2 | Generated: freezes wasm_binding, go_exported, proto_message interfaces under `wasm/x/wasm`, `wasm/proto/cosmwasm`. |
| [evm-rpc.md](../contracts/evm-rpc.md) | 256 | 97 | Generated: freezes jsonrpc_method, config_key, go_exported interfaces under `rpc/config/config.go`, `rpc/worker_pool_metrics.go`. |
| [evm.md](../contracts/evm.md) | 496 | 14 | Generated: freezes go_exported, storage_key, genesis_param interfaces under `modules/evm/types`, `modules/evm/keeper`. |
| [explorer-config.md](../contracts/explorer-config.md) | 1,311 | 2 | Generated: freezes env_var, config_key interfaces under `services/explorer/backend`, `tests/explorer/paxeer-x-data-boundary.py`. |
| [explorer-eth-rpc.md](../contracts/explorer-eth-rpc.md) | 16 | 0 | Generated: freezes jsonrpc_method interfaces under `services/explorer/backend`. |
| [explorer-frontend-config.md](../contracts/explorer-frontend-config.md) | 304 | 0 | Generated: freezes env_var, config_key interfaces under `apps/explorer/configs`. |
| [explorer-frontend.md](../contracts/explorer-frontend.md) | 97 | 22 | Generated: freezes http_route interfaces under `apps/explorer/pages`. |
| [explorer-http-api.md](../contracts/explorer-http-api.md) | 464 | 25 | Generated: freezes http_route interfaces under `services/explorer/backend`. |
| [explorer-indexer.md](../contracts/explorer-indexer.md) | 8 | 17 | Generated: freezes storage_key interfaces under `services/explorer/backend`. |
| [explorer-metrics.md](../contracts/explorer-metrics.md) | 58 | 0 | Generated: freezes metric, http_route interfaces under `services/explorer/backend`, `services/explorer/services`. |
| [explorer-services-config.md](../contracts/explorer-services-config.md) | 183 | 0 | Generated: freezes env_var, config_key interfaces under `services/explorer/services`. |
| [explorer-services.md](../contracts/explorer-services.md) | 2 | 5 | Generated: freezes config_key, rust_pub_item interfaces under `services/explorer/services`. |
| [explorer-tests.md](../contracts/explorer-tests.md) | 0 | 7 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [explorer-ws.md](../contracts/explorer-ws.md) | 108 | 0 | Generated: freezes ws_route interfaces under `services/explorer/backend`, `services/explorer/services`. |
| [ibc.md](../contracts/ibc.md) | 1,295 | 7 | Generated: freezes go_exported, proto_message, error_code interfaces under `interchain/modules/core`, `interchain/modules/apps`. |
| [interop-ap2.md](../contracts/interop-ap2.md) | 98 | 4 | Generated: freezes rust_pub_item, file_format interfaces under `services/interop/crates`. |
| [interop-bridge-relayer.md](../contracts/interop-bridge-relayer.md) | 412 | 0 | Generated: freezes rust_pub_item, config_key, file_format interfaces under `services/interop/crates`. |
| [interop-committed-proof.md](../contracts/interop-committed-proof.md) | 6 | 8 | Generated: freezes rust_pub_item interfaces under `services/interop/crates`. |
| [interop-deploy.md](../contracts/interop-deploy.md) | 55 | 11 | Generated: freezes file_format, http_route, cli_command interfaces under `services/interop/deploy`. |
| [interop-fiat.md](../contracts/interop-fiat.md) | 28 | 4 | Generated: freezes rust_pub_item interfaces under `services/interop/crates`. |
| [interop-gas-station.md](../contracts/interop-gas-station.md) | 252 | 0 | Generated: freezes rust_pub_item, config_key, error_code interfaces under `services/interop/crates`. |
| [interop-gateway.md](../contracts/interop-gateway.md) | 204 | 9 | Generated: freezes rust_pub_item, http_route, error_code interfaces under `services/interop/crates`. |
| [interop-intent-ingester.md](../contracts/interop-intent-ingester.md) | 54 | 0 | Generated: freezes rust_pub_item, env_var, cli_command interfaces under `services/interop/crates`. |
| [interop-migrate.md](../contracts/interop-migrate.md) | 253 | 0 | Generated: freezes rust_pub_item, config_key, error_code interfaces under `services/interop/crates`. |
| [interop-mirror-contracts.md](../contracts/interop-mirror-contracts.md) | 35 | 0 | Generated: freezes error_code, sol_function, file_format interfaces under `services/interop/contracts`. |
| [interop-mirror-signer.md](../contracts/interop-mirror-signer.md) | 50 | 0 | Generated: freezes rust_pub_item, env_var, cli_command interfaces under `services/interop/crates`. |
| [interop-mirror.md](../contracts/interop-mirror.md) | 314 | 0 | Generated: freezes rust_pub_item, config_key, error_code interfaces under `services/interop/crates`. |
| [interop-portable.md](../contracts/interop-portable.md) | 67 | 2 | Generated: freezes rust_pub_item, file_format interfaces under `services/interop/crates`, `services/interop/PORTABILITY.md`. |
| [interop-service.md](../contracts/interop-service.md) | 195 | 27 | Generated: freezes error_code, config_key, file_format interfaces under `services/interop/crates`. |
| [interop-specs.md](../contracts/interop-specs.md) | 51 | 6 | Generated: freezes file_format interfaces under `services/interop/specs`. |
| [interop-ucp.md](../contracts/interop-ucp.md) | 52 | 5 | Generated: freezes rust_pub_item interfaces under `services/interop/crates`. |
| [interop-visa-tap.md](../contracts/interop-visa-tap.md) | 43 | 4 | Generated: freezes rust_pub_item interfaces under `services/interop/crates`. |
| [interop-websearch.md](../contracts/interop-websearch.md) | 714 | 0 | Generated: freezes rust_pub_item, error_code, config_key interfaces under `services/interop/crates`. |
| [interop-x402.md](../contracts/interop-x402.md) | 101 | 18 | Generated: freezes rust_pub_item, file_format interfaces under `services/interop/crates`. |
| [interop.md](../contracts/interop.md) | 51 | 138 | Generated: freezes rust_pub_item, config_key, http_route interfaces under `services/interop/crates`. |
| [kernel.md](../contracts/kernel.md) | 2,639 | 249 | Generated: freezes c_symbol, go_exported, file_format interfaces under `include/layerx/programs.h`, `include/layerx/lxp_kernel.h`. |
| [launchpad.md](../contracts/launchpad.md) | 5,405 | 98 | Normative: Launchpad 0x1017 ABI with selectors, authority, effects, errors, events, gas, activation, invariants and pause; takes the still-true launchpad content of the retired exchange-launchpad.md. |
| [layerxanchor.md](../contracts/layerxanchor.md) | 149 | 18 | Generated: freezes go_exported, storage_key, error_code interfaces under `modules/layerxanchor/types`, `modules/layerxanchor/keeper`. |
| [layerxgov.md](../contracts/layerxgov.md) | 20 | 5 | Generated: freezes go_exported interfaces under `modules/layerxgov/types`, `modules/layerxgov/client`. |
| [ledger-auth.md](../contracts/ledger-auth.md) | 186 | 133 | Normative: Layerxledger 0x101A and layerxauth 0x101C ABI with selectors, authority, effects, errors, events, gas, activation, invariants and pause. |
| [markets.md](../contracts/markets.md) | 1,975 | 158 | Normative: Domain-3 precompiles (perps at 0x1015, oracle 0x1008, pricefeed 0x1020, spot 0x1021-0x1025) with selectors, authority, effects, errors, events, gas, activation, invariants and pause; takes the still-true markets content of the retired exchange-launchpad.md. |
| [mcp-tools.md](../contracts/mcp-tools.md) | 0 | 16 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [operations.md](../contracts/operations.md) | 6,559 | 21 | Generated: freezes env_var, cli_command, config_key interfaces under `Makefile`, `tools/qualification/paxeer-x`. |
| [pay.md](../contracts/pay.md) | 242 | 95 | Normative: Layerxpay 0x101B escrow, budget, stream and service ABI. |
| [perps-client.md](../contracts/perps-client.md) | 1,571 | 35 | Generated: freezes ts_export, env_var, http_route interfaces under `apps/perps/src`. |
| [perps-data.md](../contracts/perps-data.md) | 618 | 32 | Generated: freezes go_exported, http_route, storage_key interfaces under `services/platform/perps-data`. |
| [platform-cli.md](../contracts/platform-cli.md) | 574 | 28 | Generated: freezes cli_command, rust_pub_item, env_var interfaces under `tools/platform/cli`, `tools/platform/emulator`. |
| [platform-examples.md](../contracts/platform-examples.md) | 59 | 1 | Generated: freezes ts_export, config_key, env_var interfaces under `services/platform/examples`. |
| [platform-gateway.md](../contracts/platform-gateway.md) | 500 | 36 | Generated: freezes http_route, error_code, env_var interfaces under `services/platform/hosted`. |
| [platform-hosted.md](../contracts/platform-hosted.md) | 1,454 | 55 | Generated: freezes env_var, error_code, cli_command interfaces under `services/platform/hosted`, `services/platform/docs`. |
| [platform-middleware.md](../contracts/platform-middleware.md) | 426 | 2 | Generated: freezes ts_export, config_key, env_var interfaces under `services/platform/middleware`, `services/platform/examples`. |
| [platform-perf-db.md](../contracts/platform-perf-db.md) | 152 | 0 | Generated: freezes rust_pub_item, cli_command, file_format interfaces under `services/platform/perf-db`. |
| [platform-ramps.md](../contracts/platform-ramps.md) | 374 | 6 | Generated: freezes config_key, rust_pub_item, env_var interfaces under `services/platform/ramps`. |
| [platform-relay-archive.md](../contracts/platform-relay-archive.md) | 129 | 0 | Generated: freezes error_code, config_key, cli_command interfaces under `services/platform/relay_archive`. |
| [platform-sdk.md](../contracts/platform-sdk.md) | 3,735 | 113 | Generated: freezes go_exported, file_format, config_key interfaces under `sdk/platform/jvm`, `sdk/platform/swift`. |
| [platform-tools.md](../contracts/platform-tools.md) | 28 | 0 | Generated: freezes cli_command, env_var, rust_pub_item interfaces under `services/platform/tools`, `services/platform/tests`. |
| [platform.md](../contracts/platform.md) | 2 | 1 | Generated: freezes http_route interfaces under `services/platform/hosted`. |
| [programme.md](../contracts/programme.md) | 41 | 89 | Generated: freezes file_format, cli_command, config_key interfaces under `spec/layerx-merge/programme`, `spec/layerx-merge/docs`. |
| [programs.md](../contracts/programs.md) | 5,149 | 206 | Normative: Layerxprograms 0x101D ABI, views, events and the programs runtime surface. |
| [proto-api.md](../contracts/proto-api.md) | 584 | 50 | Generated: freezes proto_message, rest_route, grpc_query interfaces under `sdk/proto/cosmos`, `api/tokenfactory/query.proto`. |
| [retirements.md](../contracts/retirements.md) | 0 | 3 | Generated: freezes acceptance criteria and contexts only; no interface block cites it. |
| [rpc.md](../contracts/rpc.md) | 77 | 0 | Normative: The JSON-RPC, WebSocket, gRPC and REST catalogue of domain 7 with parameters, returns, errors, status and covering tasks. |
| [sdk-js.md](../contracts/sdk-js.md) | 2,655 | 43 | Generated: freezes ts_export, config_key interfaces under `sdk/js/dist`, `sdk/js/package.json`. |
| [sequencer.md](../contracts/sequencer.md) | 2,723 | 343 | Normative: ProtocolReceipt v2, receipt key pin, MsgSequenceBatch, WAL, nonce and fee rules. |
| [services.md](../contracts/services.md) | 6,451 | 382 | Normative: Per-service reads, writes, signing keys, liveness and evidence. |
| [sig-provider-http.md](../contracts/sig-provider-http.md) | 10 | 0 | Generated: freezes grpc_service_method, http_route interfaces under `services/explorer/services`. |
| [telemetry.md](../contracts/telemetry.md) | 53 | 1 | Generated: freezes metric interfaces under `node/metrics.go`, `modules/evm/keeper`. |
| [test-harness.md](../contracts/test-harness.md) | 112 | 55 | Generated: freezes go_exported, file_format, c_symbol interfaces under `tests/testutil/processblock`, `tests/occ/utils`. |
| [tokenfactory.md](../contracts/tokenfactory.md) | 181 | 37 | Generated: freezes go_exported, storage_key, cli_command interfaces under `modules/tokenfactory/types`, `modules/tokenfactory/keeper`. |
| [tools-ops.md](../contracts/tools-ops.md) | 11 | 140 | Generated: freezes cli_command, config_key, file_format interfaces under `tools/paxeer-x/candidate.py`, `scripts/gates/run_gate.py`. |
| [verifier-http.md](../contracts/verifier-http.md) | 28 | 0 | Generated: freezes grpc_service_method, http_route interfaces under `services/explorer/services`. |
| [wasm-runtime.md](../contracts/wasm-runtime.md) | 176 | 0 | Generated: freezes go_exported, error_code interfaces under `wasm-runtime/types/queries.go`, `wasm-runtime/types/msg.go`. |
| [web-client.md](../contracts/web-client.md) | 1,603 | 65 | Generated: freezes ts_export, http_route, env_var interfaces under `apps/web/src`. |
| [web.md](../contracts/web.md) | 353 | 57 | Normative: Xweb 0x1019 methods, views, params and attestor rules. |
