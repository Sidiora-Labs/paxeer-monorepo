# Merged network documentation

Paxeer runs one BFT network with an integrated kernel and EVM chain ID `125` (`0x7d`). Receipts are final when their blocks commit. The pages below describe the node, native constructs, application services and clients.

## Architecture and build

| Page | Content |
|---|---|
| [Network design](../design.body.md) | Execution, authority, persistence, receipts and tenant domains. |
| [Native build components](BUILD-REPORT.md) | Node targets, native link inputs and program workspace. |
| [Contracts index](../contracts/README.md) | The full set of component contract pages. |

## Component contracts

| Page | Content |
|---|---|
| [ledger-auth.md](../contracts/ledger-auth.md) | Ledger and auth contract: layerxledger 0x101A and layerxauth 0x101C |
| [markets.md](../contracts/markets.md) | Markets contract: perps, spot, pricefeed, oracle, epoch, mint and layerxexchange |
| [mcp-tools.md](../contracts/mcp-tools.md) | MCP tools contract |
| [operations.md](../contracts/operations.md) | Operations contract |
| [pay.md](../contracts/pay.md) | Pay contract: layerxpay 0x101B |
| [perps-client.md](../contracts/perps-client.md) | Perps client contract |
| [perps-data.md](../contracts/perps-data.md) | Perps data contract |
| [platform-cli.md](../contracts/platform-cli.md) | Platform CLI contract |
| [platform-examples.md](../contracts/platform-examples.md) | Platform examples contract |
| [platform-gateway.md](../contracts/platform-gateway.md) | Platform gateway contract |
| [platform-hosted.md](../contracts/platform-hosted.md) | Platform hosted contract |
| [platform-middleware.md](../contracts/platform-middleware.md) | Platform middleware contract |
| [platform-perf-db.md](../contracts/platform-perf-db.md) | Platform perf DB contract |
| [platform-ramps.md](../contracts/platform-ramps.md) | Platform ramps contract |
| [platform-relay-archive.md](../contracts/platform-relay-archive.md) | Platform receipt retrieval contract |
| [platform-sdk.md](../contracts/platform-sdk.md) | Platform sdk contract |
| [platform-tools.md](../contracts/platform-tools.md) | Platform tools contract |
| [platform.md](../contracts/platform.md) | Platform contract |
| [programme.md](../contracts/programme.md) | Merged network component contract |
| [programs.md](../contracts/programs.md) | Programs contract: layerxprograms 0x101D |
| [proto-api.md](../contracts/proto-api.md) | Proto API contract |
| [retirements.md](../contracts/retirements.md) | Native execution and Solidity interfaces |
| [rpc.md](../contracts/rpc.md) | RPC contract: node transports and method handlers |
| [sdk-js.md](../contracts/sdk-js.md) | JavaScript Cosmos package contract |
| [sequencer.md](../contracts/sequencer.md) | Sequencer contract |
| [services.md](../contracts/services.md) | Off-chain service contracts |
| [sig-provider-http.md](../contracts/sig-provider-http.md) | Sig provider HTTP contract |
| [telemetry.md](../contracts/telemetry.md) | Telemetry contract |
| [test-harness.md](../contracts/test-harness.md) | Test harness contract |
| [tokenfactory.md](../contracts/tokenfactory.md) | Tokenfactory contract |
| [tools-ops.md](../contracts/tools-ops.md) | Tools ops contract |
| [verifier-http.md](../contracts/verifier-http.md) | Verifier http contract |
| [virtual-chains.md](../contracts/virtual-chains.md) | Virtual chains contract |
| [wasm-runtime.md](../contracts/wasm-runtime.md) | Wasm runtime contract |
| [web-client.md](../contracts/web-client.md) | Web client contract |
| [web.md](../contracts/web.md) | Web contract: xweb precompile 0x1019 and modules/xweb |

## Source definitions

The [precompile registration](../../../precompiles/setup.go), [node assembly](../../../node/app.go), [upgrade gates](../../../node/upgrades.go), [protobuf schemas](../../../api) and [hosted OpenRPC contract](../../../services/platform/hosted/gateway/openrpc.json) define their corresponding product surfaces. Listener configuration and upgrade activation determine what a particular node exposes.

<a id="layerx-merge-specification-v2-documentation-index"></a>
<a id="specification-files"></a>
<a id="why-interface-blocks-are-in-interfaceskvx"></a>
<a id="documentation-docs"></a>
<a id="contracts-contracts"></a>
