# Merged network components

The merged network combines the native kernel, Go application modules, EVM
precompiles, hosted services and clients. The table identifies their source roots
and responsibilities.

| Component | Source roots | Responsibility |
| --- | --- | --- |
| Kernel and host | `src`, `include/layerx`, `modules/lxphost` | Native execution and the Go host bridge |
| Ledger, authorization and payments | `modules/layerxledger`, `modules/layerxauth`, `modules/layerxpay` | Accounts, assets, authorization and payment constructs |
| Markets | `modules/perps`, `modules/spot`, `modules/pricefeed`, `modules/layerxexchange` | Native trading, prices and exchange surfaces |
| Launchpad | `modules/launchpad`, `precompiles/launchpad`, `services/launchpad`, `sdk/launchpad` | Launchpad state, EVM methods, services and clients |
| Node runtime | `node`, `node/sequencer`, `engine`, `consensus` | Application execution, ordered receipts, proposal validation and BFT consensus |
| Chain modules | `modules`, `precompiles` | State keepers and EVM executors |
| Node APIs | `rpc`, `api`, `proto` | JSON-RPC and generated protocol APIs |
| Hosted platform | `services/platform`, `sdk/platform` | Hosted APIs and platform SDKs |
| Agent | `services/agent` | Agent daemon, protocol libraries and SDKs |
| Human services | `services/human` | Human-service APIs, intents and KMS |
| Interoperability | `services/interop` | Bridge, mirror, web search and payment integrations |
| Wallet | `apps/wallet`, `services/wallet`, `sdk/wallet` | Wallet UI, gateway, attestation and SDK |
| Programs | `programs`, `modules/lxphost`, `precompiles/layerxprograms` | Guest runtime, chain integration and program execution |
| Web, explorer and bridge | `apps/web`, `apps/explorer`, `apps/bridge`, `services/explorer`, `services/bridge`, `sdk/js` | User applications, indexing, bridge deployment and JavaScript clients |
| SDK framework | `sdk`, `interchain` | Chain framework and integration libraries |
| Tools | `tools`, `scripts` | Build, deployment and development utilities |

## State and callable surfaces

[Application construction](../../../../node/app.go) connects the shared kernel
host to the ledger, auth and pay keepers. It also connects chain-domain state to
the EVM and launchpad keepers. Sub-chains use tenant domains within the network.

[Precompile setup](../../../../precompiles/setup.go) declares thirteen merge
entries: five construct addresses (`0x101A` through `0x101E`) and eight agent
function addresses (`0x0901` through `0x0908`). Module requirements and upgrade
gates determine when those entries are available. The
[activation table](activation-and-migration.md#merge-precompiles) lists each entry.

The [sequencer keeper](../../../../modules/sequencer/keeper/keeper.go) persists
sequence continuity, while [batch verification](../../../../modules/sequencer/types/batch_verify.go)
binds the ordered transaction list to the batch commitment. The node validates
that commitment before executing a sequenced proposal.

## Services and clients

The node JSON-RPC and hosted gateway each register their own method sets.
[Node registration](../../../../rpc/server.go) includes `eth`, `pax` and `px`
handlers; the node's `px_` methods include chain reads and explicit refusals for
hosted service operations. The hosted
[gateway contract](../../../../services/platform/hosted/gateway/openrpc.json)
declares its methods separately.
The explorer frontend lives in `apps/explorer`, and its backend and indexer live
in `services/explorer/backend`. Wallet, agent and platform packages have separate
manifests and dependency boundaries.

## Activation and coverage

The `v7.0` handler adds the merged stores and initializes sequencer state.
`v7.0-anchor-store-delete` applies at a later height. The
[activation page](activation-and-migration.md) describes persisted activation,
refunds, custody release and anchor lifetime. The [test plan](test-plan.md)
identifies focused tests and their package manifests. The
[orchestration page](orchestration.md) describes transaction and proposal flow.
