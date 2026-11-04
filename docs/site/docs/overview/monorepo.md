# Monorepo layout

This repository contains Paxeer X Network: one network with the LayerX
kernel embedded in the Paxeer node. Paxeer mainnet-beta uses EVM chain id
`125` (`0x7d`). The kernel is part of the network and proposes blocks;
execution and committed receipts belong to the same BFT consensus boundary.

## What lives where

| Path | Role | Build entry |
| --- | --- | --- |
| `daemon/paxd/`, `node/`, `consensus/`, `modules/`, `rpc/`, `precompiles/`, `storage/` | Node application, consensus integration, chain modules, JSON-RPC, EVM entry points, storage | Root `Makefile` and `chain.mk` |
| `src/`, `include/` | C17 kernel and public C headers | `make build` produces `build/liblayerx.a` |
| `modules/lxphost/`, `modules/sequencer/` | Embedded kernel host and committed sequence state used by the node | `make paxd-build` includes native dependencies |
| `programs/` | Program runtime, guest SDK, examples and fixtures | Root Programs Makefile targets and crate manifests |
| `services/agent/`, `services/human/`, `services/platform/`, `services/interop/` | Agent tooling, Human control plane, hosted platform services, external protocol adapters | Subsystem Cargo workspaces and included Makefile targets |
| `sdk/platform/`, `sdk/wallet/`, `sdk/js/`, `sdk/integrations/`, `sdk/launchpad/` | Client SDKs and application integrations | Package-local manifests |
| `sdk/baseapp/`, `sdk/types/`, `sdk/x/` | Chain application framework, types and SDK modules | Go manifests and `chain.mk` |
| `contracts/`, `precompiles/` | Contract sources and the node's native EVM contracts | Foundry/Hardhat manifests and Go packages |
| `tools/platform/cli/`, `tools/release/`, `docker/` | CLI, release tooling, container definitions | Local manifests and scripts |
| `spec/`, `tests/`, `docs/` | Specifications, qualification sources and documentation | Task-specific gates; `docs/site/mkdocs.yml` configures this site |

The former root `agent/`, `human/`, `platform/`, and `interop/` service
locations have moved under `services/`. Use the paths above when locating
an implementation or choosing a package to build.

## Build and release boundaries

Language workspaces and service images still have distinct build inputs.
That separation does not create another chain:

- `make build` builds the C kernel archive, not the complete node.
- `make paxd-build` depends on the kernel archive and Programs release build,
  then builds the integrated node. Its version comes from
  `tools/release/version.json`.
- `make paxeer-build`, `make paxeer-lint`, `make paxeer-test`, and
  `make paxeer-ci` delegate to `chain.mk`. Their dependencies and available
  local toolchains determine what can run.
- Service and SDK manifests define their own package builds and tests.
  Qualification is selected by the owning task; broad CI and release gates
  are separate from a package's focused verification.

Read the root `Makefile`, included `services/*/Makefile.inc` files, and the
owning package manifest before running a command. Historical release-tag
prefixes are not evidence of separate execution or settlement authority.

## Trust boundaries

`node/app.go` integrates the kernel host, chain keepers and sequence module.
EVM precompiles call those keepers under chain authorization. A signed
execution receipt is useful before commit, but payment finality requires
committed block inclusion; see [Activities](../concepts/activities.md) and
[Payments](payments.md).

Hosted gateways, Human sessions, KMS handles, registry publications, and
webhook delivery each have their own credentials and operational state.
Those services do not create an additional settlement layer. Their journals
prove local durable acceptance, not a chain commit. See
[Hosted internal](../operators/hosted-internal.md) for authentication,
enrollment and restart responsibilities.

Sub-chains are tenant domains represented by namespaces in the database.
Creating a tenant domain does not require another infrastructure deployment;
services must carry the tenant parameter through authorization and storage.

## Workflow naming

CI retains subsystem names such as `agent`, `human`, `platform`, and
`paxeer`. A workflow name or directory prefix identifies source ownership,
not an independently deployed network. Chain sources span root directories
and shared subpaths, so inspect the workflow's path filters rather than
assuming all chain changes live under one directory.

The public JSON-RPC uses the numbered mainnet-beta node hosts and the
unnumbered node host at their root path, as listed in
[Public JSON-RPC](../reference/public-rpc.md).
`node.hyperpaxeer.com` is only the hpx library mirror used by build scripts,
not an RPC endpoint.

## Further reading

- Root `README.md`: repository overview.
- Root `CONTRIBUTING.md`: contribution workflow.
- `docs/QUALIFICATION.md`: qualification evidence and release gates.
- [Payments developer path](payments.md): transaction construction and merchant settlement.
- [Hosted internal](../operators/hosted-internal.md): internal signing and event adapters.
