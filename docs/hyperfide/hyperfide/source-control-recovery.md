# Native source recovery

A recovery profile states the failure model before the service claims readiness. It names the metadata acknowledgment mode and failure domains, every required immutable object placement, verified storage capabilities, backup retention, an independent authority journal, and RPO/RTO objectives. The schema is [recovery-profile.schema.json](../../../tools/hyperfide/deploy/hyperfide/source-control/recovery-profile.schema.json).

`FSYNC_PRIMARY` requires an fsync-enabled PostgreSQL primary. `SYNC_REPLICA` also requires an observed streaming synchronous replica when capturing a backup. A profile claiming loss of a failure domain must name distinct metadata and object failure domains. Backend identifiers alone do not establish replication: every required backend must be configured and every retained object is read and verified there.

## Capture and verify

Call `source.backups.create` with an immutable `backup_id`, the profile, expiry and explicit revision/object/byte budgets. Capture locks the source repository while it records the revision inventory and unresolved upload, collection and publication identities. Each root receives a durable backup pin. Exceeding a budget refuses capture rather than truncating its inventory.

The source manifest accompanies the PostgreSQL physical backup. Capture them from the same consistent metadata inventory; restore verification refuses additional or missing revisions as well as changed descriptors. Keep the external authority journal outside the restored database and its snapshot. Preserve the manifest, profile digest and backup identifier with the physical backup inventory.

Call `source.backups.verify` with a bounded batch size until the state is `READY`. Verification follows actual immutable child references and checks every object envelope against its native identity at every required placement. Missing or corrupted bytes leave the backup `FAILED`; an unavailable placement cannot certify readiness. `source.backups.read` reports the retained manifest and progress.

## Restore

1. Use the existing external writer-fencing workflow to isolate prior writers and verify their absence. Restore PostgreSQL into an isolated target and reestablish the acknowledgment mode required by its profile.
2. Complete the core `storage.authority` restored activation against the independent journal. Preserve the original activation operation and predecessor identities while reconciling an uncertain activation.
3. Call `source.recovery.prepare` with the backup and positively verified external activation. This resets object verification for that activation and fences ordinary source mutations until activation completes; copied `READY` metadata never substitutes for a new read of object storage.
4. Run `source.backups.verify` to completion. A missing-object failure must be repaired from a verified retained placement or backup before continuing.
5. Call `source.recovery.activate`. The trusted activation verifier checks the current external journal and committed restored activation. The source authority generation advances only after the new closure verification succeeds. Refresh source credentials for the successor generation.

The manifest retains unresolved upload, collection and Git-publication operation identities. Reconcile those operations against actual external state using their original identities. Completed effects remain completed. An unknown deletion targets its original physical incarnation, while an uncertain Git export retains its exact publication operation and destination state.

## Routing

Every source service uses `SourceRoutedObjectCatalog`. An unspecified owner may write only while the partition is unassigned; after ownership is configured, that service must identify the matching owner. Mutating catalog transactions hold the partition route generation while committing, and initial ownership assignment waits for already admitted unconfigured transactions. `source.routing.move` advances the owner with an expected-generation check; obsolete owners cannot begin or complete uploads after the move. Existing immutable reads continue through the shared object placements. The route response declares shared storage explicitly; changing an owner does not imply a copy to another backend.

## Qualification

Run the component recovery drill from the monorepo root:

```sh
cd tools/hyperfide && corepack enable && python3 tests/hyperfide/cases/source_control/6_1.py --component
```

This uses owned PostgreSQL 18 processes and a signed HTTP S3 fixture. It kills actual source workers, stops the primary, promotes its physical synchronous standby, and restores a physical backup. The receipt reports `PROCESS_LOSS` and `PRIMARY_INSTANCE_LOSS` as `PASS` only after those checks succeed. Both PostgreSQL instances share one host, so `FAILURE_DOMAIN_LOSS` remains `UNRUN`. Distinct-host or region loss requires a separately qualified topology and external fencing evidence; a live object store alone cannot qualify it.

For the live S3 variant, supply a fresh isolated bucket or a bucket allowing isolated native source scopes and set `HF_SOURCE_S3_ENDPOINT`, `HF_SOURCE_S3_REGION`, `HF_SOURCE_S3_BUCKET`, `HF_SOURCE_S3_ACCESS_KEY_ID`, and `HF_SOURCE_S3_SECRET_ACCESS_KEY` in the environment or `tools/hyperfide/.env`. Set `HF_SOURCE_S3_SESSION_TOKEN` when required. The existing `HF_ARTIFACT_S3_*` configuration aliases are also supported. Credentials need conditional object creation, reads, conditional deletion and multipart upload/abort permissions. Then run this exact command from the repository root:

```sh
cd tools/hyperfide && corepack enable && python3 tests/hyperfide/cases/source_control/6_1.py
```

The live variant additionally exercises a real multipart upload and reconciles its abort after a lost acknowledgment. It needs object-store credentials, PostgreSQL 18 binaries and permission to run owned PostgreSQL processes as the `postgres` user; provider and model credentials are not required by this drill. Component and live S3 variants have separate receipts. The live variant requires the configured object backend.

Private receipts under `tools/hyperfide/build/source-control/6.1/` retain the verified revision, acknowledgment inventory, manifest, profile, backend identity, adapter version and compiled implementation digest, PostgreSQL version, fault schedule and measured recovery duration. The component backend also records its fixture source digest. The S3 object API does not expose the provider server version; that observation remains explicitly `UNRUN`. `deterministic_faults` records injected missing objects, lost acknowledgments, stale credentials, concurrent ownership movement and delayed deletion separately from `real_infrastructure` outcomes. Every declared `failure_domains` entry is `PASS`, `FAIL` or `UNRUN`; unresolved external qualification and cleanup debt remain visible even when the component checks pass.

During rebalance, continuous immutable reads and revision commits run before, during and after the actual route move. The drill observes the move waiting on an admitted partition transaction, requires both workers to make progress while it waits, then verifies every acknowledged revision and its exact operation replay. The obsolete owner must receive typed `STALE_ROUTE` for upload admission and completion. After restore activation, the SDK sends `source.acceptance.accept` over real HTTP with the retained signed prior-epoch credential. Authentication remains valid, but authorization returns typed `SCOPE_DENIED`; neither acceptance inventory nor the attempted operation may change.

Recovery separately reconciles external effects captured by the backup. `source.recovery.reconcile` accepts the backup, the current preparation operation, and a bounded item limit. It observes the exact upload placement, finishes only an already recorded conditional deletion, and uses readback for an issued Git publication. It never creates a replacement upload or sends another push. An unavailable observation leaves the effect pending. Unacknowledged uploaded bytes remain unacknowledged after reconciliation.

The backup manifest includes active retention roots as well as revision roots, so retained evidence, custody, imports, exports and holds outside a revision tree are included in closure verification. A completed object scan does not authorize reopening writes: activation also requires every captured external effect to have a durable resolved outcome. The recovery profile applies to the captured recovery point; effects issued after that point require their independent retained journal or inventory.

Backup manifest version 2 adds the complete retained-root and external-effect inventories. This version is independent of native source object format version 1. Older backup manifests remain inspectable, but verification and preparation return `BACKUP_FORMAT_UNSUPPORTED`; they cannot establish restoration readiness. Capture and verify a new version-2 backup while the current authority is healthy before relying on the upgraded service for recovery.

After external activation and `source.recovery.prepare`, call `source.recovery.reconcile` repeatedly with the same preparation operation and a bounded `limit` until `remaining_effects` reaches zero. Replay of preparation does not reset resolved effects. An unknown remote head or unavailable object observation keeps the relevant effect pending and source mutations fenced. Only then may activation reopen source writes; object verification can finish while effect debt remains. An effect outside the captured recovery point returns `BACKUP_INCOMPLETE` and requires a recovery point or independently retained inventory that includes it.
