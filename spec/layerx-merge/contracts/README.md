# Contracts index

The contracts describe the network, hosted services, precompiles and clients.
Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
A receipt is final when its block is committed.

The [address table](address-table.md) defines the shared precompile address,
selector and error-code registry. The [address inventory](address-inventory.md)
is generated. The [client contract](clients.md) describes transaction submission
and receipt verification.

## Contract files

| File | Description |
| --- | --- |
| [address-inventory.md](address-inventory.md) | Precompile and agent-function address inventory, per-address methods, events and errors, error-registry rows and activation gates at the source revision |
| [address-table.md](address-table.md) | Precompile address and error registry of the merged Paxeer X chain |
| [agent-api.md](agent-api.md) | Agent API contract |
| [agent-cli.md](agent-cli.md) | Agent CLI contract |
| [agent-config.md](agent-config.md) | Agent config contract |
| [agent-errors.md](agent-errors.md) | Agent errors contract |
| [agent-formats.md](agent-formats.md) | Agent formats contract |
| [agent-functions.md](agent-functions.md) | Agent functions contract: precompiles 0x0901 to 0x0908 and the layerxagent module |
| [agent-http.md](agent-http.md) | Agent http contract |
| [agent-human-peer.md](agent-human-peer.md) | Agent human peer contract |
| [agent-lni.md](agent-lni.md) | Agent lni contract |
| [agent-mcp.md](agent-mcp.md) | Agent mcp contract |
| [agent-metrics.md](agent-metrics.md) | Agent metrics contract |
| [agent-observability.md](agent-observability.md) | Agent observability contract |
| [agent-python-sdk.md](agent-python-sdk.md) | Agent python sdk contract |
| [agent-rust-api.md](agent-rust-api.md) | Agent rust API contract |
| [agent-ts-sdk.md](agent-ts-sdk.md) | Agent TypeScript SDK contract |
| [bridge-client.md](bridge-client.md) | Bridge client contract |
| [bridge-deploy.md](bridge-deploy.md) | Bridge deploy contract |
| [bridge-solana.md](bridge-solana.md) | Bridge solana contract |
| [bridge-vectors.md](bridge-vectors.md) | Bridge vectors contract |
| [chain-launchpad.md](chain-launchpad.md) | Chain launchpad contract |
| [clients.md](clients.md) | client contract |
| [consensus-fork.md](consensus-fork.md) | Consensus fork contract |
| [cosmos-fork.md](cosmos-fork.md) | Cosmos fork contract |
| [cosmwasm.md](cosmwasm.md) | Cosmwasm contract |
| [evm-rpc.md](evm-rpc.md) | Evm rpc contract |
| [evm.md](evm.md) | Evm contract |
| [explorer-config.md](explorer-config.md) | Explorer config contract |
| [explorer-eth-rpc.md](explorer-eth-rpc.md) | Explorer eth rpc contract |
| [explorer-frontend-config.md](explorer-frontend-config.md) | Explorer frontend config contract |
| [explorer-frontend.md](explorer-frontend.md) | Explorer frontend contract |
| [explorer-http-api.md](explorer-http-api.md) | Explorer http api contract |
| [explorer-indexer.md](explorer-indexer.md) | Explorer indexer contract |
| [explorer-metrics.md](explorer-metrics.md) | Explorer metrics contract |
| [explorer-services-config.md](explorer-services-config.md) | Explorer services config contract |
| [explorer-services.md](explorer-services.md) | Explorer services contract |
| [explorer-tests.md](explorer-tests.md) | Explorer tests contract |
| [explorer-ws.md](explorer-ws.md) | Explorer ws contract |
| [ibc.md](ibc.md) | Ibc contract |
| [inference-market.md](inference-market.md) | Inference market contract: rooms, deals, laps, canaries, bonds, streams and the orchestration profile |
| [interop-ap2.md](interop-ap2.md) | Interop ap2 contract |
| [interop-bridge-relayer.md](interop-bridge-relayer.md) | Interop bridge relayer contract |
| [interop-committed-proof.md](interop-committed-proof.md) | Interop committed proof contract |
| [interop-deploy.md](interop-deploy.md) | Interop deploy contract |
| [interop-fiat.md](interop-fiat.md) | Interop fiat contract |
| [interop-gas-station.md](interop-gas-station.md) | Interop gas station contract |
| [interop-gateway.md](interop-gateway.md) | Interop gateway contract |
| [interop-intent-ingester.md](interop-intent-ingester.md) | Interop intent ingester contract |
| [interop-migrate.md](interop-migrate.md) | Interop migrate contract |
| [interop-mirror-contracts.md](interop-mirror-contracts.md) | Interop mirror contracts contract |
| [interop-mirror-signer.md](interop-mirror-signer.md) | Interop mirror signer contract |
| [interop-mirror.md](interop-mirror.md) | Interop mirror contract |
| [interop-portable.md](interop-portable.md) | Interop portable contract |
| [interop-service.md](interop-service.md) | Interop service contract |
| [interop-specs.md](interop-specs.md) | Interop specs contract |
| [interop-ucp.md](interop-ucp.md) | Interop ucp contract |
| [interop-visa-tap.md](interop-visa-tap.md) | Interop visa tap contract |
| [interop-websearch.md](interop-websearch.md) | Interop websearch contract |
| [interop-x402.md](interop-x402.md) | Interop x402 contract |
| [interop.md](interop.md) | Interop contract |
| [kernel.md](kernel.md) | Kernel contract |
| [launchpad.md](launchpad.md) | Launchpad contract: module, precompile 0x1017, Solidity contracts and tooling |
| [layerxanchor.md](layerxanchor.md) | Layerxanchor contract |
| [layerxgov.md](layerxgov.md) | Layerxgov contract |
| [ledger-auth.md](ledger-auth.md) | Ledger and auth contract: layerxledger 0x101A and layerxauth 0x101C |
| [markets.md](markets.md) | Markets contract: perps, spot, pricefeed, oracle, epoch, mint and layerxexchange |
| [mcp-tools.md](mcp-tools.md) | Mcp tools contract |
| [operations.md](operations.md) | Operations contract |
| [pay.md](pay.md) | Pay contract: layerxpay 0x101B and the layerxpay module |
| [perps-client.md](perps-client.md) | Perps client contract |
| [perps-data.md](perps-data.md) | Perps data contract |
| [platform-cli.md](platform-cli.md) | Platform cli contract |
| [platform-examples.md](platform-examples.md) | Platform examples contract |
| [platform-gateway.md](platform-gateway.md) | Platform gateway contract |
| [platform-hosted.md](platform-hosted.md) | Platform hosted contract |
| [platform-middleware.md](platform-middleware.md) | Platform middleware contract |
| [platform-perf-db.md](platform-perf-db.md) | Platform perf db contract |
| [platform-ramps.md](platform-ramps.md) | Platform ramps contract |
| [platform-relay-archive.md](platform-relay-archive.md) | Platform relay archive contract |
| [platform-sdk.md](platform-sdk.md) | Platform sdk contract |
| [platform-tools.md](platform-tools.md) | Platform tools contract |
| [platform.md](platform.md) | Platform contract |
| [programme.md](programme.md) | Programme contract |
| [programs.md](programs.md) | Programs contract: layerxprograms 0x101D, the programs runtime and the guest SDKs |
| [proto-api.md](proto-api.md) | Proto api contract |
| [retirements.md](retirements.md) | Retirements contract |
| [rpc.md](rpc.md) | RPC contract: JSON-RPC, WebSocket, gRPC and REST catalogue |
| [sdk-js.md](sdk-js.md) | Sdk js contract |
| [sequencer.md](sequencer.md) | Sequencer contract |
| [services.md](services.md) | off-chain service contracts |
| [sig-provider-http.md](sig-provider-http.md) | Sig provider http contract |
| [telemetry.md](telemetry.md) | Telemetry contract |
| [test-harness.md](test-harness.md) | Test harness contract |
| [tokenfactory.md](tokenfactory.md) | Tokenfactory contract |
| [tools-ops.md](tools-ops.md) | Tools ops contract |
| [verifier-http.md](verifier-http.md) | Verifier http contract |
| [virtual-chains.md](virtual-chains.md) | Virtual chains contract |
| [wasm-runtime.md](wasm-runtime.md) | Wasm runtime contract |
| [web-client.md](web-client.md) | Web client contract |
| [web.md](web.md) | Web contract: xweb precompile 0x1019 and modules/xweb |
