# Registry deployment journal

The hosted registry accepts deployment evidence at
`POST /__registry/deployments` under its request bearer and mTLS policy.
Send canonical `DeploymentProof` bytes or JSON with exactly one `admission`
field containing the canonical bytes as a numeric array. A transaction hash
alone does not authorize a deployment record.

`services/platform/hosted/registry/src/routes.rs` verifies the proof through
`NodeProgramStateSource::verify_stored_deployment`. It checks program identity,
code hash, ABI, version and upgrade predecessor before committing the record.
The verifier authenticates transaction, result and mounted-state evidence
against configured chain trust. Paxeer is one BFT network, EVM chain ID 125;
receipt finality comes from block commitment.

## Atomic envelopes

`LAYERX_REGISTRY_JOURNAL` holds sealed `<receipt_digest>.envelope` units.
Each contains the canonical deployment record and proof under domain
`LayerX/deployment-envelope/v1\0`, with a SHA-256 seal over both framed
values. Publication writes and syncs a temporary file, renames it into place
and syncs the directory. A complete envelope is the durable commit unit.

On restart, the registry loads complete units, verifies their proofs, sorts
accepted deployments by authenticated sequence, program and version, and
rebuilds the catalog and interface history. Incomplete or corrupt units are
quarantined; local files alone do not establish a verified deployment.
See `src/journal.rs` and `src/routes.rs` under
`services/platform/hosted/registry/`.

## Human consumer exports

Verified replay exports a canonical pair beneath `pairs/`:

| File | Encoding |
| --- | --- |
| `<receipt_digest>.admission` | `DeploymentProof::canonical_encoding()` |
| `<receipt_digest>.deployment` | `DeploymentRecord::canonical_encoding()` |

Digest names are 64 lowercase hexadecimal characters. The export directory
uses mode `0700`; files use `0600`. Publication stages, syncs and renames
each file and syncs the directory. Replay regenerates an interrupted export
from the committed envelope. Consumers require both matching halves.

Configure the Human evidence assembler to read an authorized protected copy
of `pairs/`. Its executing user must own that copy. Preserve the registry's
owner-only permissions when transferring the evidence.

The registry StatefulSet and Human owner-provisioning Job mount the
`layerx-registry-journal` PVC at `/var/lib/layerx-registry-journal`, with a
read-only mount for the Job. The registry initializes the journal as UID 4030.
The host evidence assembler consumes its configured local export; a shared
volume mount does not itself transfer that export to the host.

[Hosted registry](registry.md) · [Programs](../programs/index.md)
