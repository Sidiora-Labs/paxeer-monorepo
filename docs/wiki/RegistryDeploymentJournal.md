# Registry deployment journal

`POST /__registry/deployments` admits an authenticated Programs deployment
proof under the registry's request bearer policy. It accepts raw canonical
`DeploymentProof` bytes or a JSON object containing only `admission`, an array
of byte values. A transaction hash alone is not deployment authority.
The route records an existing deployment; it does not submit a transaction.

The registry verifies the supplied proof against its configured native chain
trust and a current authenticated head from `/v1/protocol/account-state/head`.
`NodeProgramStateSource::verify_stored_deployment` binds the historical
admission and current head through the chain trust steps and
`ProtocolDeploymentVerifier`. Program version history must also agree:
a first deployment has version 1, and an upgrade requires its verified
predecessor. An identical stored version can be replayed; conflicting evidence
is refused.

Success returns HTTP 200 with `program_id`, `code_hash`, `abi_version`,
`version`, `transaction_hash` and `state: "deployed"`. The transaction hash
is the verified evidence's activity ID. Invalid admission or version history
returns `422 deployment_proof_refused`; an expired request returns
`503 request_deadline_exceeded`; journal or projection-rebuild failure returns
`503 journal_unavailable`.

See [Hosted registry](HostedRegistry.md) for listener authentication,
configuration and metadata routes.

## Durable records and exports

`LAYERX_REGISTRY_JOURNAL` holds sealed `.envelope` commit units containing
the canonical deployment record and proof. An append publishes the envelope;
verified replay rebuilds the registry projection and exports consumer pairs.
Replay re-verifies stored deployment evidence against the current chain
head. Incomplete, corrupt or misfiled units are quarantined while complete
units remain available.

The `pairs/` directory exports two files per unsigned receipt digest, using
lowercase 64-digit hex names:

| File | Encoding |
| --- | --- |
| `<receipt-digest>.admission` | `DeploymentProof::canonical_encoding()` |
| `<receipt-digest>.deployment` | `DeploymentRecord::canonical_encoding()` |

The envelope remains the commit authority. Export holds `.pairs.lock`, copies
the protected existing pairs into a sibling staging directory, writes the new
pair, syncs the staged directory, and atomically publishes the directory.
An existing `pairs/` directory is exchanged with the stage using
`RenameFlags::EXCHANGE`; an initial export uses rename. The journal parent is
then synced. Directories have mode 0700 and files mode 0600. Verified replay
can regenerate an interrupted export; consumers must refuse incomplete pairs.

Configure the Human assembler's journal input to the exported `pairs/`
directory. The internal directory containing envelopes and head metadata is
not that input. A separate chain transaction lookup helper can persist
`chain-deployments/<transaction-hash>.json`, but it requires a matching
authenticated envelope; this metadata does not replace proof admission.

## Hosted consumers

The [agent boundary](HostedAgentBoundary.md) serves registry-only
`GET /internal/v1/deployment-proof/{hash}`. Its response carries a verified
signed receipt, not the complete `DeploymentProof`. The former internal
POST deploy and upgrade routes are retired and do not execute deployments.

The registry StatefulSet and Human owner-provisioning Job mount the
`layerx-registry-journal` PVC, subpath `journal`, at
`/var/lib/layerx-registry-journal`, and set `LAYERX_REGISTRY_JOURNAL` to that
path. The registry initializes it as UID 4030, mode 0700. The Job mount is
read-only. Registry pod affinity places the ReadWriteOnce volume on the node
pod's host, where the owner Job is pinned.

The host-side Human assembler consumes a protected local journal export owned
by its executing user. A PVC mount does not copy that export to the invoking
host. Preserve the file protections when materializing the paired records;
UID 4020 cannot directly read UID 4030 mode-0600 files.

Sources:

- [`routes.rs`](../../services/platform/hosted/registry/src/routes.rs)
- [`node_state.rs`](../../services/platform/hosted/registry/src/node_state.rs)
- [`programs_chain.rs`](../../services/platform/hosted/registry/src/programs_chain.rs)
- [`journal.rs`](../../services/platform/hosted/registry/src/journal.rs)
- [`registry deployment`](../../services/platform/hosted/registry/deployment.yaml)
- [`Human deployment`](../../services/platform/hosted/human/deployment.yaml)

[Home](Home.md)
