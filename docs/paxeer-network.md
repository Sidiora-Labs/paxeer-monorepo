# Paxeer Network

**Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).**

`paxd` embeds the LayerX kernel as part of the network's block proposal and
execution path. EVM execution, native balances, asset-ledger operations and
programs run within the merged chain. A receipt is final once its block is
committed.

This repository contains the node, chain modules, kernel, contracts, SDKs and
hosted applications. [Monorepo layout](MONOREPO.md) maps their source paths;
[Qualification](QUALIFICATION.md) explains the evidence required across the
integrated runtime. Directory and build boundaries do not create separate
networks or independent finality authorities.

## Merged runtime and services

| Path | Owns |
| --- | --- |
| `daemon/paxd/`, `node/` | Node entrypoint, application wiring, block execution, upgrades and commit |
| `src/`, `include/`, `modules/lxphost/` | C kernel and its Go/cgo integration inside the node |
| `modules/`, `precompiles/`, `programs/` | Chain state, EVM precompiles and programs runtime |
| `services/agent/`, `services/human/`, `services/platform/` | Agent, human and hosted platform services consuming the network |
| `services/explorer/`, `apps/explorer/` | Explorer backend, supporting services and frontend |
| `services/interop/`, `services/bridge/`, `apps/bridge/`, `contracts/bridge/` | Interop services, bridge deployment, frontend and cross-chain contracts |
| `spec/layerx-merge/` | Merged-network specification and interface contracts |

The node build links `build/liblayerx.a` and the programs sandbox archive
through `modules/lxphost/host.go`. The integrated module set includes
`layerxledger`, `layerxauth`, `layerxpay`, `layerxprograms`, `layerxagent`
and `layerxchains`, wired in `node/app.go`. Native gas balances and
per-asset ledger balances are read from their respective chain interfaces.
Hosted services and explorer databases provide application views; their
responses do not replace committed chain evidence.

The consensus chain identifier used by node distribution is
`hyperpax_125-1`, configured by `tools/hpx/`; its EVM chain ID is `125`.

## Public JSON-RPC and funding

Set `PAXEER_RPC_URL` to the selected node's root HTTPS URL. This request
should return `"0x7d"` as its result:

```sh
curl --max-time 15 -sS "$PAXEER_RPC_URL" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Check chain identity before signing, use bounded timeouts, and compare
`eth_blockNumber` across configured nodes when freshness matters. Consult
[Public RPC endpoints](site/paxeer-docs/src/app/public-rpc/page.tsx) for the full list.
The [HTTP server](../rpc/server.go) registers `debug`, `echo`, `eth`, `net`, `pax`, `pax2`, `px`,
`txpool` and `web3`; method availability follows node configuration. The `test`
namespace requires `EnableTestAPI` and a non-live chain ID. The `px` namespace is served at the node root from node
state; `px_register` and `px_requestFunds` use the hosted services. For valid
arguments, those two methods at the node root return JSON-RPC error `-32004`
with data code `hosted_gateway_service_required`.
The gateway's method contract is specified in
[`services/platform/hosted/gateway/openrpc.json`](../services/platform/hosted/gateway/openrpc.json).

Mainnet-beta funding follows the operator's approved allocation process.
Do not assume a public faucet. The hosted test-network faucet implementation
in `services/platform/hosted/faucet/src/main.rs` requires an explicitly
configured funding backend and credentials; its presence in the tree does
not make it a public mainnet funding service. Confirm funding in committed
chain balances before spending.

## Virtual chains

A virtual chain, or sub-chain, is a tenant domain: a namespace in the
database registered by `layerxchains`. It needs no new process or
infrastructure and shares the parent network's consensus and EVM state.
Creating one does not change the main network's chain ID `125`.

The registry precompile is
`0x000000000000000000000000000000000000101E`
(`precompiles/layerxchains/address.go`). For a nonzero domain ID `id`,
the tenant EVM chain ID is `125000000 + id`
([`modules/layerxchains/types/domain.go`](../modules/layerxchains/types/domain.go)). The node serves its HTTP and
WebSocket view at `/chain/<id>` on the same node host
([`rpc/rpcstack.go`](../rpc/rpcstack.go)); these are scoped views, not additional networks.
Each domain has gas and byte allowances, a base fee and an operator fee share.

The launchpad's **Launch a chain** flow collects the name and size tier,
submits the wallet transaction and supplies the domain RPC URL, explorer
page and wallet configuration. Feature availability depends on deployment
activation. A tenant namespace scopes identity, fees and block space under
the existing network security.

See [Virtual chains](wiki/VirtualChains.md) for the ABI, lifecycle and
consumer entrypoints, and the
[feature-20 contract](../spec/layerx-merge/contracts/virtual-chains.md)
for integration requirements. The registry implementation and node request
scoping determine the behavior available in a deployed revision.

## Layout

| Path | Purpose |
| --- | --- |
| `daemon/paxd/` | `paxd` node binary |
| `node/` | Application wiring, genesis, upgrades and commit |
| `modules/`, `precompiles/` | Native chain modules and EVM precompile implementations |
| `modules/lxphost/` | Embedded kernel host and chain-store integration |
| `rpc/` | Node JSON-RPC, receipts and tenant-domain views |
| `contracts/`, `contracts/src/` | Solidity sources selected by the root and Paxeer Foundry configurations respectively |
| `consensus/`, `sdk/`, `storage/` | Consensus, framework fork and storage engines |
| `docker/` | Local node and cluster definitions |
| `tools/hpx/` | Native node distribution and peer registry tooling |
| `docs/` | Network, API and operator documentation |

The root Go module is `github.com/Sidiora-Labs/Paxeer-X-Network`.
See [Monorepo layout](MONOREPO.md) for SDK, app and service locations.

## Build and test

Run these node entrypoints from the **repository root**:

```sh
make paxeer-build
make paxeer-lint
make paxeer-test
make paxeer-ci
```

These delegate to `chain.mk`. The build produces `build/paxd` and builds
the native kernel and programs sandbox prerequisites. The release build
entrypoint `make paxd-build` additionally stamps the version from
`tools/release/version.json`. Go dependencies must be available locally:
the `paxeer-build`, `paxeer-test` and `paxeer-ci` wrappers use `GOPROXY=off`.

Direct entrypoints, also from the repository root, include:

```sh
make -f chain.mk build    # build/paxd, including native prerequisites
make -f chain.mk install  # install daemon/paxd with the linked runtime
make -f chain.mk test
make -f chain.mk ci       # lint and test
```

`chain.mk` recognizes namespaced `paxeer-network/vX.Y.Z` tags.
`make monorepo-ci` composes the workspace build, test, lint and inventory
checks through `workspace-ci`; it is an aggregate release gate. See
[Qualification](QUALIFICATION.md) for kernel matrices and integrated
commit, proof and restart evidence.

Local cluster targets such as `docker-cluster-start` and `run-local-node`
are described in [`docker/README.md`](../docker/README.md) and `chain.mk`.
Their paths are relative to the repository root.

### Foundry

Both root Foundry configurations pin Solidity `0.8.27`. Select the project
explicitly from the repository root:

```sh
forge build --config-path foundry.toml
forge build --config-path foundry.paxeer.toml
```

The first configuration selects the root `contracts` tree and excludes
`contracts/src`, `contracts/test`, bridge and launchpad projects. The
second selects `contracts/src`, `contracts/test` and `contracts/lib`.
See [`contracts/README.md`](../contracts/README.md) for dependency setup.
Contract compilation provides development evidence; deployment follows
the selected network's release runbook.

## Documentation

- [Monorepo layout](MONOREPO.md): current source and build boundaries
- [Qualification](QUALIFICATION.md): evidence levels and release gates
- [Public RPC endpoints](site/paxeer-docs/src/app/public-rpc/page.tsx): root node URLs and method guidance
- [EVM JSON-RPC differences](evm_jsonrpc_unsupported.md): documented unsupported methods
- [OpenAPI/Swagger generation](paxeer-chain-docs.md#openapiswagger-docs-generation): generating the node's API documentation
- [Merged-network specification](../spec/layerx-merge/spec.kvx): acceptance criteria and current task status

## License

First-party network code is published under the Apache License, Version 2.0;
see [`LICENSE`](../LICENSE). Imported trees retain their own licenses,
including the explorer backend, frontend and supporting services. Consult
the relevant tree's license and provenance files before redistribution.
