# Contracts index

| Field | Value |
|---|---|
| Version | 2.0.0 |
| Source revision | 2a1033aa2 |
| Change control | changes only by amendment; each amendment raises the version of the amended contract and of this index |
| Conformance | every interface and acceptance block of the specification names one contract file and anchor in its contract key; the anchor is a heading of that file |

## Contract files
One line per normative contract file of this version; the label is how the contracts cite each other.

| Label | File | What it freezes |
|---|---|---|
| address-table.md | [address-table.md](address-table.md) | precompile and agent-function address table, per-address methods, selectors, events and errors with status, activation gates, LXP_ error registry, the superseded table's promises not served at the source revision, and the domain-6 surfaces (Solidity, proto, Msg, CLI, config, formats) whose interfaces name it |
| ledger-auth.md | [ledger-auth.md](ledger-auth.md) | layerxledger 0x101A and layerxauth 0x101C ABI with selectors, authority, effects, errors, events, gas, activation, invariants and pause |
| pay.md | [pay.md](pay.md) | layerxpay 0x101B escrow, budget, stream and service ABI |
| programs.md | [programs.md](programs.md) | layerxprograms 0x101D ABI, views, events and the programs runtime surface |
| agent-functions.md | [agent-functions.md](agent-functions.md) | agent functions 0x0901-0x0908 ABI, authority, effects and gas |
| markets.md | [markets.md](markets.md) | domain-3 precompiles (perps at 0x1015, oracle 0x1008, pricefeed 0x1020, spot 0x1021-0x1025) with selectors, authority, effects, errors, events, gas, activation, invariants and pause; takes the still-true markets content of the retired exchange-launchpad.md |
| launchpad.md | [launchpad.md](launchpad.md) | launchpad 0x1017 ABI with selectors, authority, effects, errors, events, gas, activation, invariants and pause; takes the still-true launchpad content of the retired exchange-launchpad.md |
| web.md | [web.md](web.md) | xweb 0x1019 methods, views, params and attestor rules |
| sequencer.md | [sequencer.md](sequencer.md) | ProtocolReceipt v2, receipt key pin, MsgSequenceBatch, WAL, nonce and fee rules |
| rpc.md | [rpc.md](rpc.md) | the JSON-RPC, WebSocket, gRPC and REST catalogue of domain 7 with parameters, returns, errors, status and covering tasks |
| services.md | [services.md](services.md) | per-service reads, writes, signing keys, liveness and evidence |
| clients.md | [clients.md](clients.md) | client operations, identity and keys, receipt verification, lx_* decisions and conformance |

## Retired contract files

| File | Replaced by |
|---|---|
| exchange-launchpad.md | markets.md (0x1015 and the markets modules) and launchpad.md (0x1017) |

## Contract files cited without a normative file at this version
Files named by contract keys of the specification that this version does not freeze; a block citing one of them is frozen by its own interface and acceptance text until an amendment adds the file.

| File | Citations |
|---|---|
| operations.md | 6592 |
| cosmos-fork.md | 4032 |
| platform-sdk.md | 3901 |
| agent-rust-api.md | 3502 |
| kernel.md | 2895 |
| sdk-js.md | 2727 |
| web-client.md | 1702 |
| consensus-fork.md | 1673 |
| perps-client.md | 1633 |
| platform-hosted.md | 1562 |
| explorer-config.md | 1315 |
| ibc.md | 1306 |
| agent-ts-sdk.md | 753 |
| interop-websearch.md | 714 |
| perps-data.md | 678 |
| proto-api.md | 634 |
| platform-cli.md | 614 |
| platform-gateway.md | 562 |
| evm.md | 522 |
| explorer-http-api.md | 506 |
| bridge-client.md | 476 |
| platform-middleware.md | 429 |
| interop-bridge-relayer.md | 412 |
| platform-ramps.md | 383 |
| cosmwasm.md | 380 |
| evm-rpc.md | 370 |
| agent-api.md | 321 |
| interop-mirror.md | 314 |
| explorer-frontend-config.md | 304 |
| programme.md | 297 |
| interop-migrate.md | 253 |
| interop-gas-station.md | 252 |
| interop.md | 249 |
| tokenfactory.md | 248 |
| interop-service.md | 236 |
| tools-ops.md | 223 |
| agent-config.md | 221 |
| interop-gateway.md | 218 |
| test-harness.md | 216 |
| explorer-services-config.md | 183 |
| layerxanchor.md | 180 |
| wasm-runtime.md | 176 |
| platform-perf-db.md | 152 |
| bridge-deploy.md | 141 |
| explorer-frontend.md | 137 |
| agent-errors.md | 136 |
| interop-x402.md | 130 |
| platform-relay-archive.md | 129 |
| explorer-ws.md | 108 |
| interop-ap2.md | 105 |
| agent-cli.md | 86 |
| interop-deploy.md | 77 |
| interop-portable.md | 71 |
| interop-specs.md | 62 |
| agent-human-peer.md | 61 |
| interop-ucp.md | 61 |
| platform-examples.md | 61 |
| agent-lni.md | 58 |
| explorer-metrics.md | 58 |
| telemetry.md | 56 |
| bridge-solana.md | 55 |
| interop-intent-ingester.md | 54 |
| agent-formats.md | 53 |
| bridge-vectors.md | 50 |
| interop-mirror-signer.md | 50 |
| interop-visa-tap.md | 50 |
| agent-mcp.md | 44 |
| explorer-indexer.md | 39 |
| interop-fiat.md | 35 |
| interop-mirror-contracts.md | 35 |
| layerxgov.md | 30 |
| platform-tools.md | 28 |
| verifier-http.md | 28 |
| mcp-tools.md | 24 |
| interop-committed-proof.md | 19 |
| explorer-eth-rpc.md | 16 |
| agent-observability.md | 12 |
| explorer-tests.md | 12 |
| explorer-services.md | 11 |
| sig-provider-http.md | 10 |
| agent-http.md | 8 |
| retirements.md | 4 |
| agent-metrics.md | 3 |
| agent-python-sdk.md | 3 |
| platform.md | 3 |
| perps.md | 2 |
| release.md | 2 |
| spot.md | 2 |


## Contract files generated from the merged document

The files below now resolve the contract references listed as missing above. Each file freezes its cited interface blocks, linked contexts, decisions and acceptance criteria at the pinned revision.

| File | Anchors | Interfaces |
|---|---|---|
| [agent-api.md](agent-api.md) | 188 | 201 |
| [agent-cli.md](agent-cli.md) | 21 | 57 |
| [agent-config.md](agent-config.md) | 9 | 203 |
| [agent-errors.md](agent-errors.md) | 120 | 129 |
| [agent-formats.md](agent-formats.md) | 32 | 32 |
| [agent-http.md](agent-http.md) | 3 | 0 |
| [agent-human-peer.md](agent-human-peer.md) | 61 | 61 |
| [agent-lni.md](agent-lni.md) | 46 | 55 |
| [agent-mcp.md](agent-mcp.md) | 41 | 44 |
| [agent-metrics.md](agent-metrics.md) | 2 | 0 |
| [agent-observability.md](agent-observability.md) | 12 | 12 |
| [agent-python-sdk.md](agent-python-sdk.md) | 1 | 0 |
| [agent-rust-api.md](agent-rust-api.md) | 12 | 3366 |
| [agent-ts-sdk.md](agent-ts-sdk.md) | 37 | 706 |
| [bridge-client.md](bridge-client.md) | 17 | 366 |
| [bridge-deploy.md](bridge-deploy.md) | 7 | 123 |
| [bridge-solana.md](bridge-solana.md) | 4 | 55 |
| [bridge-vectors.md](bridge-vectors.md) | 2 | 50 |
| [consensus-fork.md](consensus-fork.md) | 12 | 1590 |
| [cosmos-fork.md](cosmos-fork.md) | 14 | 3942 |
| [cosmwasm.md](cosmwasm.md) | 19 | 377 |
| [evm-rpc.md](evm-rpc.md) | 178 | 256 |
| [evm.md](evm.md) | 7 | 496 |
| [explorer-config.md](explorer-config.md) | 123 | 1311 |
| [explorer-eth-rpc.md](explorer-eth-rpc.md) | 1 | 16 |
| [explorer-frontend-config.md](explorer-frontend-config.md) | 83 | 304 |
| [explorer-frontend.md](explorer-frontend.md) | 3 | 97 |
| [explorer-http-api.md](explorer-http-api.md) | 168 | 464 |
| [explorer-indexer.md](explorer-indexer.md) | 2 | 8 |
| [explorer-metrics.md](explorer-metrics.md) | 5 | 58 |
| [explorer-services-config.md](explorer-services-config.md) | 5 | 183 |
| [explorer-services.md](explorer-services.md) | 1 | 2 |
| [explorer-tests.md](explorer-tests.md) | 1 | 0 |
| [explorer-ws.md](explorer-ws.md) | 6 | 108 |
| [ibc.md](ibc.md) | 18 | 1295 |
| [interop-ap2.md](interop-ap2.md) | 99 | 98 |
| [interop-bridge-relayer.md](interop-bridge-relayer.md) | 412 | 412 |
| [interop-committed-proof.md](interop-committed-proof.md) | 1 | 6 |
| [interop-deploy.md](interop-deploy.md) | 56 | 55 |
| [interop-fiat.md](interop-fiat.md) | 29 | 28 |
| [interop-gas-station.md](interop-gas-station.md) | 252 | 252 |
| [interop-gateway.md](interop-gateway.md) | 201 | 204 |
| [interop-intent-ingester.md](interop-intent-ingester.md) | 54 | 54 |
| [interop-migrate.md](interop-migrate.md) | 253 | 253 |
| [interop-mirror-contracts.md](interop-mirror-contracts.md) | 35 | 35 |
| [interop-mirror-signer.md](interop-mirror-signer.md) | 50 | 50 |
| [interop-mirror.md](interop-mirror.md) | 314 | 314 |
| [interop-portable.md](interop-portable.md) | 67 | 67 |
| [interop-service.md](interop-service.md) | 193 | 195 |
| [interop-specs.md](interop-specs.md) | 48 | 51 |
| [interop-ucp.md](interop-ucp.md) | 53 | 52 |
| [interop-visa-tap.md](interop-visa-tap.md) | 44 | 43 |
| [interop-websearch.md](interop-websearch.md) | 714 | 714 |
| [interop-x402.md](interop-x402.md) | 102 | 101 |
| [interop.md](interop.md) | 10 | 51 |
| [kernel.md](kernel.md) | 147 | 2639 |
| [layerxanchor.md](layerxanchor.md) | 6 | 149 |
| [layerxgov.md](layerxgov.md) | 3 | 20 |
| [mcp-tools.md](mcp-tools.md) | 11 | 0 |
| [operations.md](operations.md) | 88 | 6559 |
| [perps-client.md](perps-client.md) | 13 | 1571 |
| [perps-data.md](perps-data.md) | 19 | 618 |
| [platform-cli.md](platform-cli.md) | 18 | 574 |
| [platform-examples.md](platform-examples.md) | 3 | 59 |
| [platform-gateway.md](platform-gateway.md) | 8 | 500 |
| [platform-hosted.md](platform-hosted.md) | 92 | 1454 |
| [platform-middleware.md](platform-middleware.md) | 29 | 426 |
| [platform-perf-db.md](platform-perf-db.md) | 4 | 152 |
| [platform-ramps.md](platform-ramps.md) | 17 | 374 |
| [platform-relay-archive.md](platform-relay-archive.md) | 7 | 129 |
| [platform-sdk.md](platform-sdk.md) | 15 | 3735 |
| [platform-tools.md](platform-tools.md) | 7 | 28 |
| [platform.md](platform.md) | 1 | 2 |
| [programme.md](programme.md) | 8 | 41 |
| [proto-api.md](proto-api.md) | 582 | 584 |
| [retirements.md](retirements.md) | 1 | 0 |
| [sdk-js.md](sdk-js.md) | 35 | 2655 |
| [sig-provider-http.md](sig-provider-http.md) | 8 | 10 |
| [telemetry.md](telemetry.md) | 4 | 53 |
| [test-harness.md](test-harness.md) | 7 | 112 |
| [tokenfactory.md](tokenfactory.md) | 13 | 181 |
| [tools-ops.md](tools-ops.md) | 11 | 11 |
| [verifier-http.md](verifier-http.md) | 9 | 28 |
| [wasm-runtime.md](wasm-runtime.md) | 3 | 176 |
| [web-client.md](web-client.md) | 17 | 1603 |
