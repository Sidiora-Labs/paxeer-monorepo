<!-- Source: https://docs.paxeer.app/discovery/ -->

# Program Discovery

Discover registered programs and resolve current interfaces, lifecycle, accounts and signed deployment provenance.

## Resolve evidence before execution

Paxeer X discovery lets wallets, agents and applications find registered programs and inspect the version they are about to call. The hosted registry combines deployment records, a verified current protocol head and published interfaces. Each resolved program carries identity, version and freshness information that the client can bind to its preparation.

A program ID is a stable 32-byte identifier. The latest code hash, ABI, lifecycle and account bindings describe its current verified execution context. An application preserves both the stable identity and the version-specific evidence when presenting an action to its user.

## Start with the registry index

```
GET /v1/programs/registry
Authorization: Bearer <registry-request-token>

Response fields:
program_ids
state_root
observed_sequence
observed_at
valid_through
verification = "registry-receipt-and-current-head-verified"
```

Direct calls use the registry's mutual TLS connection and request credential. The index synchronizes protocol state and checks journal read authority before returning registered IDs. Its authenticated head and freshness window describe the observation used for that response.

### Resolve one program

```
GET /v1/programs/registry/<64-hex-program-id>
GET /v1/programs/registry/<64-hex-program-id>/interface
```

The first response includes versions, current lifecycle, upgrade policy, account bindings, source status and deployment receipts. The interface response binds canonical interface bytes and their digest to a specific program, version, code hash and ABI. Its verification marker is `deployment-interface-and-current-head-verified`.

## What a resolver checks

| Check | Required relationship |
| --- | --- |
| Program identity | The returned program ID equals the requested 32-byte identity. |
| Deployment | The selected version has a verified deployment receipt and matching code hash. |
| Interface | Interface version, code hash and ABI match the selected current deployment. |
| Lifecycle | The application handles active, deprecated and tombstoned status deliberately. |
| Freshness | The current time remains inside `valid_through` for the verified observation. |
| Value accounts | Account bindings and balance proofs match program state and the observed head. |
| Source | A verified source claim reproduces the deployed artifact under its named environment. |

Resolve related records against compatible current evidence. If an upgrade or lifecycle change occurs between reads, fetch the program and interface again before preparing a call. Keep the canonical interface digest and code hash attached to the user-facing disclosure or internal execution record.

## Node discovery with the CLI

The CLI reads the programs precompile through `eth_call` at `pending`. `layerx program discover` returns the program ID, upgrade policy, authority, code hash, ABI version, version and active status. Discovery refuses an inactive program.

| Node discovery field | Meaning |
| --- | --- |
| `policy` | The program's upgrade policy value. |
| `authority` | The 32-byte program authority. |
| `code_hash` | The code hash returned for the selected program. |

The hosted registry returns `deployment_receipt_digest` with current-head metadata. The CLI's node read returns `source: eth_call` and `block_tag: pending`. Keep these evidence types distinct when presenting an action.

### Evidence requirements

A pending node view describes the state available for preparation. Establish a committed result from the execution receipt and independently verified block-commit evidence. Display the verification level established by the returned evidence.

## Application journey

1. Fetch the registry index and retain its observed head and validity bound.
2. Resolve the chosen program and inspect its current version, lifecycle and upgrade authority.
3. Fetch the matching interface and validate its program, ABI, code hash and interface digest.
4. When using the CLI, resolve the active program and matching interface through the selected node.
5. Resolve program accounts and source provenance relevant to the intended action.
6. Prepare the activity and disclose the target program, action, amounts, counterparties and fees.
7. Track execution and retain the resulting receipt alongside the discovery evidence.

For example, a treasury agent discovers a payment program, confirms its current ABI and active lifecycle, checks its permitted assets and accounts, then prepares a bounded payment under its capability and budget. A changed version or stale discovery window triggers a fresh resolution before signing.

## Lifecycle and account-aware discovery

Immutable programs advertise an immutable upgrade policy. Upgradeable programs identify their authority and preserve a numbered deployment history. Lifecycle history and exit routes let applications explain changes and route users through appropriate recovery or withdrawal journeys.

ABI 1 programs without value accounts declare `account-incapable-abi1`. Account-capable records supply proof-verified bindings and balances. Account profile 2 includes the owner established by its verified state. Resolve these bindings directly instead of inferring account ownership from names or UI labels.

## Source and publisher identity

Source verification links a mirrored archive, reproducible build environment and artifact digest to a verified deployment. Publication keys resolve through hosted identity, associating publication with a principal. Source status and publisher identity complement execution evidence: application trust policy decides whether a source verdict or publisher binding is required for a particular interaction.

Use the [registry source workflow](https://docs.paxeer.app/registry) to publish archives and request verification. Preserve source digest, environment digest and deployment provenance when reviewing releases or generating a program catalog.

## Operate a discovery service

Back discovery with the canonical deployment journal, authenticated node-state connection and independent receipt authority. Keep the node receipt authority and its trust history current. Synchronization persists verified program state before advancing its cursor, and refuses regressions or incompatible state. Restart replay reconstructs the projection from committed evidence and refreshes its current head.

The default registry staleness bound is 300 seconds. Cache records with their `valid_through`, state root and version; revalidate before an economic action. A healthy HTTP listener does not itself establish fresh program evidence.

| Result | Resolver behavior |
| --- | --- |
| `404 not_found` | Treat the program as absent from this verified registry view. |
| `404 interface_absent` | Do not invent an interface; require published current-version metadata. |
| `503 stale_read`, `protocol_head_unavailable` | Refresh evidence before preparing a new action. |
| `interface_registry_mismatch`, `balance_registry_mismatch` | Reject inconsistent records and resolve matching evidence. |
| Inactive node discovery result | The CLI refuses discovery when `active` is false. |

Continue with [registry deployment and journals](https://docs.paxeer.app/registry), [owner identity](https://docs.paxeer.app/identity) or [agent spending controls](https://docs.paxeer.app/agent-budgets).

**Technical references:** `services/platform/hosted/registry/src/routes.rs`, `services/platform/hosted/registry/src/head_attestation.rs`, `services/platform/hosted/registry/src/node_state.rs` and `tools/platform/cli/src/programs.rs`.
