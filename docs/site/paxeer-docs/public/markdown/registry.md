<!-- Source: https://docs.paxeer.app/registry/ -->

# Program Registry

Publish receipt-verified program deployments, versioned interfaces and reproducible source across Paxeer X.

## A verified record of deployed programs

The Paxeer X registry connects a program identity to its deployment receipts, executable code hashes, ABI versions, upgrade policy, lifecycle and value accounts. Applications resolve the current verified version before preparing calls. Publishers attach source that the registry rebuilds in a pinned environment and compares with the registered artifact.

Registry records derive from signed activities, signed receipts and committed program state. A successful admission acknowledgement starts the deployment workflow; the verified execution evidence establishes the published deployment.

## Registry API

| Method and path | Purpose |
| --- | --- |
| `GET /v1/programs/registry` | List registered program IDs with current verified head and freshness information. |
| `GET /v1/programs/registry/{program_id}` | Resolve deployment history, current version, lifecycle and accounts. |
| `GET /v1/programs/registry/{program_id}/interface` | Read the interface bound to the current verified version. |
| `POST /v1/programs/registry/{program_id}/source` | Verify mirrored source against the deployed program. |
| `POST /__registry/deployments` | Register an already-deployed chain transaction by hash; the registry reads the deployment from the chain and journals it. |
| `POST /__registry/sources` | Publish an encoded source archive, build plan and source URI. |
| `POST /__registry/head` | Refresh the journal head from independently verified protocol state. |

Program IDs contain 32 hexadecimal-encoded bytes. Direct registry access requires mutual TLS and the appropriate Bearer credential. Request and source-publication credentials are separate. Source publication also resolves the `LayerX-Key` header to an authorized principal.

## Deploy and register

1. Build the program and deploy it on chain through the ordinary Programs deployment path.
2. Send the deployment transaction hash to `POST /__registry/deployments`, either as a raw 32-byte body or as JSON `{"transaction_hash": "0x…"}`. Any other body is refused with `400 invalid_argument` (*request must carry a bytes32 transaction_hash*).
3. The registry reads the deployment back from the chain RPC configured in `LAYERX_REGISTRY_CHAIN_RPC_URL`. It never submits an activity itself; with no chain RPC configured the route returns `503 deployment_proof_unavailable`.
4. A refused chain read returns `422 deployment_proof_refused`; an unavailable one returns `503 deployment_proof_unavailable`. On success the deployment is journaled — a journal failure returns `503 journal_unavailable` — and the route answers `200`.
5. Resolve the program and its current interface before enabling application calls.

The success response carries `transaction_hash`, `program_id`, `code_hash`, `abi_version`, `version`, `policy`, `authority`, `block_hash`, `block_number` and `state: "deployed"`, all read from chain state. Upgrades are registered the same way, after the upgrade transaction has landed on chain.

### Legacy deployment proofs

The sealed-envelope journal and its native deployment proofs (signed activity and receipt inclusion paths, batch header, Programs subtree witnesses) now replay legacy stored units: on rebuild the registry re-verifies each stored proof against node state and regenerates its export. New registrations through `POST /__registry/deployments` are chain-ingested and do not create a native proof.

## Read a program record

| Field group | What it tells an application |
| --- | --- |
| `program_id`, `latest_version`, `versions` | Stable program identity and numbered deployments with code hash, ABI and receipt digest. |
| `upgrade_policy` | Immutable, or upgradeable with a named authority. |
| `lifecycle`, `lifecycle_history`, `exit_routes` | Active, deprecated or tombstoned status and relevant exit information. |
| `value_accounts` | Program account bindings and proof-verified balance information for account-capable ABIs. |
| `state_root`, `observed_sequence`, `observed_at`, `valid_through` | The authenticated state and permitted freshness window. |
| `receipt`, `source` | Deployment evidence and source verification status. |

ABI 1 programs with no value accounts report `account-incapable-abi1`. ABI 2 account reads require current account-primary and state proofs matching the program, lifecycle, bindings and registry head. Account profile 2 additionally identifies the verified owner.

## Publish and verify source

Publish the archive into the operator-controlled mirror using `source_uri`, encoded `plan` and `archive_hex`. The response returns the archive digest. The URI is an identifier for mirrored content; verification uses the retained archive and pinned build plan.

```
POST /v1/programs/registry/<program-id>/source
Content-Type: application/json
Idempotency-Key: verify-program-release-0001
Authorization: Bearer <request-token>
LayerX-Key: <publication-principal-key>

{
  "source_uri": "<mirrored-source-uri>",
  "source_digest": "<32-byte-source-digest-hex>"
}
```

The idempotency key contains 16–128 ASCII letters, digits, hyphens or underscores. Source verification binds the request to the resolved principal and verified deployment provenance. A changed deployment cannot inherit an earlier verification result.

### Reproducible build verdicts

The builder runs offline in an isolated process tree using the pinned environment, runtime and supervisor digests. CPU, memory, process count, file size, workspace quota and deadline bounds constrain each build. The reproduced artifact digest must match the registered code hash.

Source status is `unpublished`, `verified` or `mismatch`. Verified results include source and environment digests and pipeline information. A mismatch returns the expected deployed hash and reproduced artifact hash. Durable verification state and a publication outbox allow recovery to finish publication without relabeling unrelated evidence.

## The deployment journal

The journal stores sealed `.envelope` units containing the canonical deployment record and proof under a domain-separated envelope with a SHA-256 seal. Publication writes a temporary file, synchronizes it, renames it and synchronizes the directory. The envelope is the atomic commit authority.

Restart replay re-verifies complete records, rebuilds the registry projection and quarantines incomplete, corrupt or misfiled units. The verified source store is compared again with registered protocol state. Cached program state is hash-checked and refreshed against a verified current head before publication.

### Export evidence to consumers

The journal's `pairs/` directory exports matching `<unsigned-receipt-digest>.admission` and `.deployment` files. These use canonical DeploymentProof and DeploymentRecord encodings. Startup replay regenerates interrupted exports from committed envelopes; a consumer refuses an incomplete pair.

Point evidence assemblers at the exported pair directory. Preserve owner-only file modes and materialize exports through an authorized process when consumers run under another identity. Keep journal, mirror, verification state and sequencer trust history in protected durable storage.

## Operate and troubleshoot

`GET /healthz` reports listener health. Verify actual program reads and a current independently authenticated head to establish usable registry state. The default freshness bound is 300 seconds and is configurable through `LAYERX_REGISTRY_MAX_STALENESS_SECONDS`.

| Refusal | Action |
| --- | --- |
| `deployment_proof_unavailable`, `receipt_authority_unavailable` | Reconcile the signed deployment and restore its evidence dependencies. |
| `stale_read`, `protocol_head_unavailable` | Restore head synchronization before using the record for a new call. |
| `source_mismatch`, `build_not_reproducible` | Correct source or build inputs and publish a new verification request. |
| `publication_principal_unresolved` | Resolve the publication key to its authorized identity. |
| `journal_export_unavailable` | Recover export storage while retaining the committed envelope. |

Continue with [program discovery and safe resolution](https://docs.paxeer.app/discovery), or [identity and ownership](https://docs.paxeer.app/identity).

**Technical references:** registry routes, deployment journal, node-state verifier, head attestation and isolated source builder.
