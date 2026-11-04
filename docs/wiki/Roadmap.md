# Roadmap: beta surface expansion

This index separates implemented libraries and guest examples from native
module support and operational deployment. The labels 8.1–8.7 are page
sections, not implementation task identifiers. Source availability does not
establish a deployment address or activation on a particular network.

Paxeer is one network with EVM chain id **125** (`0x7d`). The kernel is part
of that network and proposes blocks; receipts are final when their blocks
are committed. Guest functionality runs through Programs module `9`, so a
feature can be implemented without acquiring a dedicated module ordinal.
See [Programs workspace gates](ProgramsWorkspace.md) for the current
workspace, ABI checks and guest packaging requirements.

---

## 8.1 LXT721

**Status: implemented guest SDK and collection reference.**

`programs/sdk/rust/src/lxt721.rs` provides the request codecs, and
`programs/sdk/rust/examples/nft-lxt721/src/lib.rs` implements the ABI-v2
collection guest. Its nine exports cover minting, ownership and balance
queries, transfers, approvals, operator approvals, token URI, metadata and
total supply. Ownership lives in the program's shared namespace. The
reference does not provide burn, royalties, enumeration or transfer callbacks.

`programs/crates/layerx-programs-registry/src/lxt721.rs` binds the reference
interface to the WASM exports and capabilities. The example's `README.md`
documents its issuer, supply cap and build script. A deployment must select
those configuration values and register the resulting artifact and interface;
this page does not assign a deployed collection address.

See [Programs](Programs.md) and [Programs workspace gates](ProgramsWorkspace.md#frozen-abi).

---

## 8.2 Multisig and timelock authority

**Status: primary-key rotation and threshold recovery exist; general multisig
transaction authority is a separate capability.**

`include/layerx/lxp_identity.h` defines one current primary key, pending and
superseded keys, recovery thresholds and approvals, and delayed rotation and
recovery operations. Their challenge delays use batch timestamps. Threshold
recovery authorizes replacement of a primary key; it does not itself define
a general transaction policy requiring several live signing keys.

Programs upgrade policy and authority are declared by the deployment and
exposed by the Programs precompile's `getProgram` view. A multisig or timelock
integration must specify and prove its own authorization path rather than
infer one from a recovery threshold or an upgrade-authority field.

---

## 8.3 Naming

**Status: implemented guest SDK, naming reference and registry interface.**

`programs/sdk/rust/src/naming.rs` defines the naming codecs and reference
parameters. `programs/sdk/rust/examples/naming/src/lib.rs` implements
`register`, `transfer`, `renew`, `resolve` and `reverse_resolve` in a program
namespace, with expiry checked against the execution height. Registration
and renewal fund the reference's derived occupancy account.

`programs/crates/layerx-programs-registry/src/naming.rs` binds those five
exports and their capabilities. This is guest functionality, distinct from
the kernel's canonical account-name hashing. A production namespace needs
its chosen parameters, deployed program id and verified interface.

---

## 8.4 `oracle_read`

**Status: implemented Programs host function from guest ABI 3 onward.**

`programs/crates/layerx-programs-runtime/src/host/oracle.rs` registers
`layerx_v3.oracle_read`. It reads a committed perps observation through the
execution ABI and writes a 64-byte record containing price, observation
timestamp, sequence and source-set digest. The guest SDK's
`programs/sdk/rust/src/oracle.rs` decodes that record through `oracle::read`.

Execution reads committed state; it does not fetch an external price over
the network during a call. The caller must select a compatible guest ABI
and handle host refusals, including an unknown or halted market. See the
[ABI policy](ProgramsWorkspace.md#frozen-abi) and [Perps](Perps.md).

---

## 8.5 Constant-product swap

**Status: implemented ABI-v2 guest reference.**

`programs/sdk/rust/examples/swap-cpmm/src/lib.rs` implements
`add_liquidity`, `remove_liquidity`, `swap_exact_in`, `quote` and `reserves`.
It pools two LXT-20 program tokens through nested `program_call` operations
and uses its own derived program accounts for the share token. The example
includes fee and slippage bounds; its `README.md` documents the integer
arithmetic and token configuration.

`programs/crates/layerx-programs-registry/src/swap.rs` supplies the bound
reference interface. Deploying a usable pool also requires the two token
programs, backing assets, configured interface and liquidity. This guest
does not change [Spot](Spot.md) into a constant-product exchange: Spot's
native matching remains an order book. All of these are capabilities of
the merged network.

---

## 8.6 1024-asset registry

**Status: implemented kernel capacity.**

`include/layerx/lx_asset.h` defines `LX_ASSET_REGISTRY_CAPACITY = 1024` and
sizes the registry's asset array with that constant. This is the capacity
bound, not a claim that 1024 assets have been registered. See [Assets](Assets.md).

---

## 8.7 Self-custody key export

**Status: declared Human API ceremony with custody implementation and tests.**

`services/human/schema/human-api/identity.kvx` declares
`security.key-export.begin` and `security.key-export.finish` at
`/v1/security/key-export/begin` and `/v1/security/key-export/finish`.
The ceremony requires step-up evidence bound to the service-issued export
challenge. Its contract returns the DID primary key once, marks the identity
self-custodied and ends service signing for that identity.

The custody implementation in
`services/human/crates/layerx-human-service/src/custody/export.rs` manages
encrypted export-envelope delivery and acknowledgement; related coverage is
in `services/human/crates/layerx-human-service/tests/key_export.rs`.
Treat the API contract, custody delivery and configured KMS provider as
parts of the same integration. The presence of these paths is not evidence
that an operator's deployed service has completed that integration.
See [Human custody](HumanCustody.md).

---

## What is shipped today

The tree contains the guest and host implementations above, together with
the AI-market and arbiter crates listed in
[Programs workspace gates](ProgramsWorkspace.md#workspace-and-crate-layout).
Deployment and activation must be checked against committed chain state.

Use the merged-chain Programs precompile at
`0x000000000000000000000000000000000000101D`, declared in
`precompiles/layerxprograms/events.go` and
`precompiles/layerxprograms/LayerXPrograms.sol`, for lifecycle operations,
calls and views such as `getProgram` and `getInterface`.
Proof-bearing query schemas are in `api/layerxprograms/query.proto`.
Verify the deployed ABI, interface, authority and chain evidence before
using a reference program.

Public JSON-RPC uses the root URL of nodes `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, or
`api-mainnet-beta.paxeer.network`, for example
`https://api1.mainnet-beta.paxeer.network`. See [Public RPC](PublicRpc.md)
and [SDK terminal verification](SdkTerminalVerification.md).

## Start here

- [Home](Home.md)
- [Modules](Modules.md)
- [Programs](Programs.md)
- [Programs workspace gates](ProgramsWorkspace.md)
- [Public RPC](PublicRpc.md)
