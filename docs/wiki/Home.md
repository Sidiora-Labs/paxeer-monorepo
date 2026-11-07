# Welcome to Paxeer X Network

Paxeer X Network is one BFT network, Paxeer mainnet-beta, running `paxd`
with EVM chain ID **125** (`0x7d`). The LayerX kernel and deterministic
Programs runtime execute inside the node. The kernel is part of the network
that proposes blocks; a transaction receipt is final when its block commits.

The public JSON-RPC is served at the root of
`https://api-mainnet-beta.paxeer.network` and
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. Use the node origin itself as the
RPC URL. `node.hyperpaxeer.com` is only the build-time hpx library mirror.
See [Public JSON-RPC](PublicRpc.md) for node and hosted API ownership.
LayerX source is licensed under the Apache License, Version 2.0.

## Repository and monorepo structure

The chain, kernel, programs, services, and SDKs share this repository.
`node/`, `consensus/`, `modules/`, and `precompiles/` implement the node and
native chain constructs. `src/` and `include/layerx/` hold the C kernel;
`modules/lxphost/` hosts it through cgo in the node's transaction context.
Kernel writes enter the chain's transaction store and become durable with
block commit. `programs/` holds the deterministic WASM runtime, registry,
guest SDKs, and reference programs. See the [monorepo layout](../MONOREPO.md).

Programs is kernel module ID `9` (`LXP_MODULE_PROGRAMS`). Its native EVM
surface is `layerxprograms` at `0x000000000000000000000000000000000000101D`.
Guest code has no direct balance-writing authority: its monetary effects
are authenticated `402LXP` transfer sets. See [Programs](Programs.md) for
activity kinds, host registration, guest ABI compatibility, and receipts.

## What is Paxeer?

Paxeer executes EVM transactions and native network operations in one chain.
The proposer executes and orders transactions; validators verify execution
and commit the block. Submission or an early signed receipt does not establish
block finality. The committed block is the finality boundary for program
storage, transfers, events, and receipts; there is no later checkpoint
settlement stage. See [Commitment levels](CommitmentLevels.md).

Sub-chains are tenant domains: namespaces in the database that share the
network and can be launched without provisioning another chain. See
[Virtual chains](VirtualChains.md).

## Key properties

1. **One canonical history.** Native kernel execution and EVM execution share
   the node's transaction and block history.
2. **Authenticated money movement.** Programs express monetary effects through
   `402LXP`; guests cannot set balances or widen the invoking authority.
3. **Reproducible execution.** The Programs runtime validates and meters WASM
   and excludes floating point and nondeterministic host imports.

## What LayerX supports

| Area | Responsibilities |
| --- | --- |
| Identity and authority | Agent DIDs, key bindings, session keys, and scoped capabilities |
| Money movement | Asset accounts and authenticated transfers through the kernel |
| Spending controls | Escrow, budgets, and streams |
| Agent commerce | Service offers, delivery, acceptance, and disputes |
| Markets | Native perps and spot venues and oracle-backed market inputs |
| Programs | Deterministic WASM, program-owned accounts, bounded calls, storage, and events |
| Network operation | Transaction ordering, block proposals, BFT commit, history, and proofs |
| Tenancy | Database-backed tenant domains on the shared network |

## Fees

EVM gas and program execution fees are distinct charges. Programs exposes
`quoteCallFee`, `getCurrentFeeSchedule`, and `getCurrentMeteringSchedule` at
`0x101D`. A fee estimate describes the schedule and declared budget; it
neither reserves funds nor proves successful execution. Persistent program
storage also carries deterministic occupancy charges. See
[Programs](Programs.md#occupancy-and-committed-state), [Fees](Fees.md), and
[Assets and tokens](Assets.md).

Hosted `px_*` methods belong to `services/platform/hosted/gateway/`, whose
`openrpc.json` defines their contract. They are not node namespaces. The
public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and
`txpool`; obtain hosted service availability from its operator.

## Resources

| Guide | Covers |
| --- | --- |
| [Virtual chains](VirtualChains.md) | Build within a tenant domain backed by a database namespace on the shared network. |

- [Cluster quickstart](Quickstart.md)
- [Getting started](Getting-Started-Beta.md)
- [Payments developer path](PaymentsQuickstart.md)
- [Public JSON-RPC](PublicRpc.md)
- [Public payment API transcript](PublicAPI.md)
- [SDK quickstarts](SdkQuickstarts.md): [Rust](SdkQuickstartRust.md) · [TypeScript](SdkQuickstartTypeScript.md) · [Python](SdkQuickstartPython.md) · [Go](SdkQuickstartGo.md) · [JVM](SdkQuickstartJvm.md) · [.NET](SdkQuickstartDotNet.md) · [Swift](SdkQuickstartSwift.md)
- [Assets and tokens](Assets.md)
- [Commitment levels](CommitmentLevels.md)
- [Running an agent](RunningAnAgent.md)
- [Protocol](Protocol.md)
- [Unified network architecture](UnifiedNetwork.md)
- [Modules](Modules.md)
- [Asset](Asset.md)
- [Escrow](Escrow.md)
- [Budget](Budget.md)
- [Stream](Stream.md)
- [Service](Service.md)
- [Perps](Perps.md)
- [Spot](Spot.md)
- [Governance](Governance.md)
- [Bridge](Bridge.md)
- [Fees](Fees.md)
- [Sequencing](Sequencing.md)
- [Finality](Finality.md)
- [Data availability](DataAvailability.md)
- [Agent API](AgentApi.md)
- [MCP](Mcp.md)
- [Roadmap: beta surface expansion](Roadmap.md)
- [Custody](Custody.md)
- [Human journeys](HumanJourneys.md)
- [Human custody](HumanCustody.md)
- [Human owner registration](HumanOwnerRegistration.md)
- [Contributing guide](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/CONTRIBUTING.md)
- [Security policy](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md)
- [Qualification documentation](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/docs/QUALIFICATION.md)
- [Monorepo layout](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/docs/MONOREPO.md)
- [Programs](Programs.md): native WASM execution, guest ABIs, LXT-20, simulation, and committed receipts
- [Sandbox](Sandbox.md)
- [Storage scan](StorageScan.md)
- [SDK terminal verification](SdkTerminalVerification.md)
- [Portable receipt verifier](PortableVerifier.md)
- [Interop adapters](Interop.md)
- [x402 transport](X402Transport.md)
- [AP2 mandates](Ap2Mandates.md)
- [UCP](Ucp.md)
- [Visa TAP](VisaTap.md)
- [Fiat ramps](FiatRamps.md)
- [Market-maker ramps](MarketMakerRamps.md)
- [Ethereum and Solana mirrors](Mirrors.md)
- [Ethereum and Solana migration](Migration.md)
- [Agentd](Agentd.md)
- [CLI](Cli.md)
- [Hosted core](HostedCore.md)
- [Hosted authority](HostedAuthority.md)
- [Programs workspace gates](ProgramsWorkspace.md)
- [Porting](Porting.md)
- [Beta cluster](BetaCluster.md)
- [Hosted gateway](HostedGateway.md)
- [Hosted webhooks](HostedWebhooks.md)
- [Hosted dashboard](HostedDashboard.md)
- [Hosted registry](HostedRegistry.md)
- [Registry deployment journal](RegistryDeploymentJournal.md)
- [Hosted Human](HostedHuman.md)
- [Hosted internal](HostedInternal.md)
- [Hosted identity](HostedIdentity.md)
- [Hosted faucet](HostedFaucet.md)
- [Hosted control](HostedBetaControl.md)
- [Hosted node](HostedNode.md)
- [Hosted agent boundary](HostedAgentBoundary.md)
- [Paxeer boundary](PaxeerBoundary.md)

---

Paxeer X Network is developed by [Sidiora Labs](https://github.com/Sidiora-Labs).
