# Recovery contracts and local checks

This directory supplies launchpad application recovery contracts: versioned
offline configuration, disabled resource declarations, content-bound backup
inventories, exact reconciliation and durable PostgreSQL restoration epochs.
The offline contracts do not execute their command templates. The
[operator tooling](phase2/runbook.md) can invoke explicitly configured providers;
its activation updates database authority and leaves traffic switching to the
traffic owner. Checked-in resources do not establish a production deployment.

These are application restoration procedures on the single Paxeer network, EVM
chain ID `125`. The kernel participates in block proposal and execution, and
block commit determines receipt finality. A restoration epoch fences application
writes and broker acknowledgements; it does not change chain finality. See the
[launchpad deployment overview](../../README.md).

## Local checks

Run fresh checks from the repository root:

```sh
python3 deploy/launchpad/production/recovery/check.py
```

The source root is `deploy/launchpad/production/recovery`; provider commands
live in `deploy/launchpad/production/recovery/phase2/providers.py`. Resolve
linked bootstrap and observability instructions under
`deploy/launchpad/production`, and launchpad load tooling under
`tools/launchpad/production/load`.

The runner requires the authorized local `kindle-bootstrap-local-postgres-1` Docker fixture described in the [bootstrap guide](../bootstrap/README.md); it verifies Compose project/service labels before creating a random schema inside the exclusively assigned namespace. It checks the schema ownership token before destruction. No user database, bucket, production credential, or global role is accessed. Each subprocess has a deadline. Tests clean only their exact owned schema.

## Offline contracts and backup sources

`recovery.configuration` validates required settings; `resources` validates every disabled operational group and rejects missing or weakened WAL, replica, replay, reconciliation, IAM and cutover gates. Targets default to 60-second WAL age, 300-second replication/RPO and 1800-second RTO. These are targets, not measurements. Retention is 35 daily points, 12 monthly snapshots and 365 archive days; changing it requires a reviewed policy revision.

`manifest` binds physical-base and WAL object inventories, PostgreSQL system identity/version/timeline/LSNs, immutable object version IDs, a single database snapshot, all five authoritative table groups, and chain 125/source/interface/deployment/checkpoint digests. The offline `restore_plan` requires a manifest digest from an independently trusted inventory; a self-provided hash is not authentication. It rejects missing coverage, wrong source identity, target times outside archived WAL coverage, or changed encryption references. It always leaves traffic cutover disabled. The provider must verify physical base manifests and WAL continuity/content, target timestamp semantics and actual committed checkpoint after replay. Metadata declarations alone do not prove any of these.

`verify_objects` is a local POSIX object-fixture adapter: bounded chunked checksum reads, regular files only, descriptor-relative no-follow path traversal, and explicit missing/corrupt errors. It does not implement remote object-version lookup, encryption, object locking, replication or IAM enforcement. The production adapter must retrieve the exact pinned version and prove independent-region accessibility. Required authoritative metadata/media replicas block health/cutover when missing; ephemeral CDN and live-playback sessions are rebuilt, never restored as authorities.

The physical backup layout is a plain PostgreSQL directory, produced by
`PostgreSQL.physical_backup` with `pg_basebackup --format=plain
--wal-method=stream --checkpoint=spread --manifest-checksums=SHA256 --no-password
--pgdata=<isolated-owned-backup-directory>`. It contains `backup_manifest`,
`PG_VERSION`, `global/`, `base/` and streamed `pg_wal/` files; it is not a tar
archive. The command vectors in `resources.json` match this provider path.
`pg_verifybackup --exit-on-error <isolated-owned-backup-directory>` verifies the
directory before its manifest digest is returned. `PostgreSQL.restore_physical`
rechecks that digest, rejects links and special files, verifies the directory
again and checks its system identifier before copying it to a fresh owned data
directory. It replaces inherited configuration, binds an owned Unix socket and
explicit port, and starts with `pg_ctl --pgdata=<isolated-owned-data-directory>
--wait --timeout=60 --log=<isolated-owned-data-directory>/recovery.log start`.
Recovery pauses at the pinned time and timeline; traffic cutover remains
disabled until independent reconciliation and fencing succeed. No extraction
or implicit format conversion is supported.

`raw_partition` validates bounded, ordered chain 125 raw-log identity with committed block-hash and transaction-position consistency. Archive rebuild never substitutes application events for chain events: verified stream backups cover only their exact epoch/hot window, raw-chain archives supply chain history, and PostgreSQL business/outbox/inbox supply application effects. These sources require provider-specific streaming readers and replay integration.

`reconcile` compares exact counts, arbitrary-size unsigned integer amount strings, event-identity digests and the entire fixed checkpoint; missing fields and mismatches block success. `measured_evidence` requires timestamped metric samples, deployment/source/input/inventory hashes, fault interval and explicit outcome/limitations. Accepting the evidence shape does not certify its measurement provenance or evaluate RPO/RTO; the release evaluator must bind fresh trusted runner outputs and enforce targets.

## Restoration authority

[epoch.sql](epoch.sql) is a SQL resource requiring registration in the deployment. Its epoch guard takes a shared row lock through the caller transaction; activation takes an exclusive lock, compares the prior epoch/revision, requires a validated proposal and externally verified fencing-proof digest, advances authority, and writes audit atomically. The guard uses a fixed `pg_catalog` search path and schema-qualified objects under a restricted owner. Revoke table writes from application roles and grant only guard execution. Only the recovery coordinator receives activation/proposal/control privileges. An application-supplied actor string or proof digest is not authorization. The [operator](phase2/runbook.md) authenticates the database principal and
validates proposals and provider fencing. Register the SQL resources and grants
for the selected deployment.

Every authoritative write/inbox effect must call the guard in the same transaction. Broker acknowledgements additionally require epoch-scoped broker authority, subjects and credential revocation; a PostgreSQL check followed by an unrestricted external acknowledgement is insufficient. Old-primary traffic/network/publishing authority must be revoked before new-region activation. Exactly one region can write; return/failback requires a new reviewed migration and is never automatic.

## Evidence boundaries

The local tests exercise logical `pg_dump`/restore consistency for marker, exact monetary amount, pending outbox, duplicate inbox and watermark; stale epoch/region rejection; one winner in competing cutovers; guard/cutover lock serialization; and authority rollback when audit insertion fails. They do not prove physical base-backup/WAL PITR, real message publishing/ack rejection, lost broker-disk rebuild, provider IAM/KMS denial, object-store replication, monthly/quarterly drills, 72-hour datasets, region loss, RPO/RTO, deployment or production readiness.

The v3 adversarial path in `phase2/` validates native results for all ten recovery scenarios, including explicit denied operations, exact before/after state reconciliation and measured recovery capacity. It preserves v1/v2 interfaces. Signed scenario receipts bind the source, deployment, interface, inventory, provider executable and actual fault interval; absent or failed receipts block activation. `activate_fenced_v3` additionally binds the authenticated database principal and the exact fresh validation digests. The focused v3 gate and isolated provider configuration are described in the [operator runbook](phase2/runbook.md); these checks do not activate traffic or certify a deployment.

`phase4/release.py` adds a read-only live measurement collector and an evaluator for the shared workload capture. It records exact native metric responses, actual pinned Kubernetes allocations and PostgreSQL snapshots; requires independently signed real recovery receipts and fixed-checkpoint reconciliation; and emits ten machine-readable requirement outcomes. Its [runbook](phase4/runbook.md) describes bounded component measurements and
shared 24-hour evaluation. Results retain false capacity and monthly-availability
qualification flags.

Use the [observability guide](../observability/README.md) for metric collection
and alert contracts alongside recovery evidence.
