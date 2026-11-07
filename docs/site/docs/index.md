# Welcome to Paxeer X Network

Paxeer X Network is one BFT network for autonomous agents, payments, and
programmable execution. LayerX is integrated into the Paxeer node, `paxd`.
The kernel is part of the network and participates in proposing blocks.
Paxeer mainnet-beta uses EVM chain ID **125** (`0x7d`).

Transactions execute on this chain, and a receipt is final when its block is
committed. Submitting a transaction or estimating its fee does not establish
successful execution. Check the committed receipt's status and logs.

Normative requirements and implementation tasks live under
[`spec/layerx-merge/`](https://github.com/Sidiora-Labs/Paxeer-X-Network/tree/main/spec/layerx-merge).
The network is mainnet-beta and uses real assets; a local example is not a
promise of a public test allocation. LayerX is licensed under Apache License 2.0.

## Repository and monorepo structure

This is the Sidiora Labs monorepo for the merged network. `node/` wires the
application, `modules/lxphost/` embeds the native kernel, `modules/` contains
chain state and business logic, and `precompiles/` exposes EVM-callable entry
points. `src/` and `include/` contain the C runtime; `programs/` contains the
Programs runtime and tooling. `rpc/` implements node JSON-RPC.

`make paxd-build` builds the node with its native libraries and release version;
`make build` builds the native runtime and supporting tools. These targets cover
different parts of the same network. See [Qualification](operators/qualification.md)
for their evidence boundaries and [Monorepo layout](overview/monorepo.md) for
workspace navigation.

## What is Paxeer?

Paxeer is the chain that executes transactions, maintains balances and application
state, and commits blocks through BFT consensus. Agent operations enter through
ordinary chain transactions, including calls to the merged precompiles. Monetary
effects are applied through the chain's ledger, bank, and EVM state paths.

Sub-chains are tenant domains stored in the database. Creating a tenant domain
does not require a new validator network or a separate node deployment. Domain
selection, account authority, fees, and state isolation still need to be checked
on the appropriate transaction path.

## Key properties

1. **Committed-block finality.** Execution and finality belong to one chain.
2. **Deterministic execution.** The native runtime uses canonical encoding and
   checked arithmetic; replay gates exercise reproducibility across compilers
   and architectures.
3. **Explicit authority.** Modules and precompiles validate callers and apply
   state changes through their wired chain keepers.

## What the merged network supports

| Area | Implementation and responsibilities |
| --- | --- |
| Identity and authority | `modules/layerxauth/`: authority, grants, rotation, recovery, and revocation |
| Money movement | `modules/layerxledger/`: asset accounts and transfers integrated with native balances |
| Spending controls and commerce | `modules/layerxpay/`: holds, escrow, budgets, streams, and service operations |
| Programs | `precompiles/layerxprograms/` and `programs/`: deployment, invocation, and sandboxed guest execution |
| Markets | `modules/layerxexchange/`: exchange state and execution |
| Tenant domains | `modules/layerxchains/`: database namespaces and domain lifecycle |
| Network operation | `node/`, `consensus/`, and `rpc/`: block processing, commit, recovery, and node access |

The merged precompile address table is declared in
[`precompiles/setup.go`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/precompiles/setup.go).
For example, Programs uses `0x000000000000000000000000000000000000101D`;
its ABI is in `precompiles/layerxprograms/abi.json`. See [Programs](programs/index.md).

## Connect to the network

Public node JSON-RPC is served over HTTPS at the root of
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`,
and at `api-mainnet-beta.paxeer.network`.

```sh
curl --fail --silent --show-error --max-time 30 \
  https://api-mainnet-beta.paxeer.network \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID result is `"0x7d"`. Node namespaces include `debug`,
`echo`, `eth`, `net`, `pax`, `test`, and `txpool`; availability also depends on
node configuration. Hosted `px_` methods are described by
`services/platform/hosted/gateway/openrpc.json`; they are gateway methods, not a
promise that every node exposes them. `node.hyperpaxeer.com` is the hpx library mirror
used by build tooling, not a transaction endpoint.

## Fees

Use `eth_estimateGas` and the node's fee methods for an EVM transaction. Estimates
are advisory: they neither reserve funds nor prove that a transaction will
succeed. Committed receipt status, gas usage, and logs describe the actual result.
See [Assets and tokens](concepts/assets.md) for asset-specific material.

## Translated overviews

[Deutsch](overview/i18n/de.md) · [Español](overview/i18n/es.md) ·
[Français](overview/i18n/fr.md) · [日本語](overview/i18n/ja.md)

## Resources

- [Quickstart](overview/quickstart.md)
- [Getting started](overview/getting-started.md)
- [Payments developer path](overview/payments.md)
- [Public node JSON-RPC](reference/public-rpc.md)
- [Hosted gateway API](platform/gateway-rpc.md)
- [Public payment API transcript](platform/public-api.md)
- [Assets and tokens](concepts/assets.md)
- [Commitment levels](protocol/commitment-levels.md)
- [Running an agent](agents/running.md)
- [Protocol](protocol/index.md)
- [Modules](protocol/modules.md)
- [Finality](protocol/finality.md)
- [Merged network specification](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/spec/layerx-merge/spec.kvx)
- [Contributing guide](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/CONTRIBUTING.md)
- [Security policy](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md)
- [Qualification documentation](operators/qualification.md)
- [Monorepo layout](overview/monorepo.md)
- [Programs](programs/index.md)
- [Sandbox](programs/sandbox.md)
- [Storage scan](programs/storage-scan.md)
- [SDK terminal verification](agents/sdk-verification.md)
- [Portable receipt verifier](interop/portable-receipts.md)
- [x402 transport](interop/x402.md)
- [Agentd](agents/agentd.md)
- [CLI](platform/cli.md)
- [Hosted core](platform/hosted-core.md)
- [Hosted authority](platform/hosted-authority.md)
- [Programs workspace gates](programs/workspace.md)
- [Porting](operators/porting.md)
- [Beta cluster](operators/beta-cluster.md)
- [Hosted gateway](platform/hosted-gateway.md)
- [Hosted webhooks](platform/webhooks.md)
- [Hosted registry](platform/registry.md)
- [Registry deployment journal](platform/registry-journal.md)
- [Hosted Human](human/hosted.md)
- [Hosted internal](operators/hosted-internal.md)
- [Hosted identity](platform/identity.md)
- [Hosted faucet](platform/faucet.md)
- [Hosted control](operators/beta-control.md)
- [Hosted node](platform/hosted-node.md)
- [Hosted agent boundary](platform/agent-boundary.md)

---

Paxeer X Network is developed by [Sidiora Labs](https://github.com/Sidiora-Labs).
