# LayerX Agent Compatibility

The executable compatibility row lives in
[`compat.rs`](../../services/agent/crates/layerx-agentd/src/compat.rs); the Agent API version is
declared in [`schema/agent-api/v1.kvx`](../../services/agent/schema/agent-api/v1.kvx).
The schema filename is retained, but the current contract is **2.0**.

| Daemon | Node interface | Contract | SDK |
| --- | --- | --- | --- |
| 0.1.0 | 1.0+ | 2 | 0.1.0 |

The node column describes the client-version major accepted by `compat::verify`
from `web3_clientVersion`. The implementation parses a client release string
such as `Geth/v1.0.0` and checks major 1; it does not negotiate an independent
node protocol. A matching version does not establish chain identity or prove a
receipt. Node connections check `eth_chainId`; Paxeer mainnet-beta uses 125
(`0x7d`). See the [workspace network boundary](../../services/agent/README.md#network-boundary).

Approval operations and lifecycle events are included in the current major-2
contract. The schema's approval `1.1` and Programs `1.2` entries describe
historical additive changes to major 1, not alternative supported versions.
SDKs reject an unsupported contract major. Approval holds are daemon-enforced
restrictions and confer no protocol authority.

Major 2 signs native EVM transactions. The [write schema](../../services/agent/schema/agent-api/write.kvx)
requires the caller's exact 65-byte secp256k1 signature (`r || s || v`, 130
lowercase hex characters) bound to the preparation; clients preserve the
unsigned canonical bytes, signing preimage and disclosure before signing.
The [error schema](../../services/agent/schema/agent-api/errors.kvx) defines `UnavailableCapability`
when a required capability is absent from the negotiated intersection. Clients
must report that error explicitly, including for unavailable approval features;
they must not emulate the capability or fall back to contract 1.0 or 1.1.

Compatibility preserves canonical bytes, result codes, verification rungs and
freshness; it never authorizes reconstructing protocol facts locally. A
transaction progresses from `submitted` to `receipted` to `committed` only as
the corresponding evidence is verified. Finality comes from the committed
block of the single network. The [SDK compatibility page](../../services/agent/sdk/COMPATIBILITY.md)
describes enforcement and portable offline evidence.

## Release checks

The [inference market guides](inference-market/README.md) cover rooms, F11,
operator and client flows, and the development attestation limits.
The daemon profile and peer binding are implemented; F11 availability must be checked on the target network.
The compatibility matrix above does not establish network deployment or
end-to-end readiness.

From the repository root, `make agent-test-agentd-migration` runs the daemon's
[`migration`](../../services/agent/crates/layerx-agentd/tests/migration.rs) integration target,
including `verify_published` and unsupported-tuple refusals, and then
`agent-test-boundary`. The Makefile passes `AGENT_NODE_RPC_URL` (default
`http://127.0.0.1:8545`) to the boundary harness as
`LAYERX_QUALIFY_NODE_RPC_URL`. Use a disposable node with the required boundary
capabilities; the matrix alone does not attest to a deployment's readiness.
