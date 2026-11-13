# Native source-control migration and monorepo consolidation

## 1. Existing source and scoped supersession

The [migration service](../../../tools/hyperfide/core/control-plane/src/source_control/migration/) implements inventory, consolidation, parity and cutover. It consolidates pinned Git imports into native revisions and records the source-authority transition. The [Git importer](../../../tools/hyperfide/core/control-plane/src/source_control/git/import.ts) admits imported history with provenance.

A source-authority transition is explicit. Legacy source remains authoritative until a cutover receipt pins the native repository, snapshot and authority epoch. Task ownership, qualification and remote publication retain their separate permissions.

## 2. Consolidation manifest

The initial target is one large monorepo containing many independently owned projects. Large existing projects are ordinary subtrees of that shared namespace. Storage partitions distribute objects and metadata within that repository; a project root, an imported repository, a bucket prefix or a tenant is not assumed to fit on one server.

The [consolidation parser](../../../tools/hyperfide/core/control-plane/src/source_control/migration/consolidation.ts) accepts these exact fields:

| Field | Meaning |
|---|---|
| `schema_version` | `1` |
| `migration_id`, `base_revision_id` | Migration UUID and immutable native base |
| `projects` | Between 2 and 128 project records |
| `legacy_writer_ids` | Between 1 and 128 distinct writer identities |
| `legacy_source_epoch` | Legacy epoch UUID or `null` |
| `provenance` | Public author, message and timestamp |
| `workflow_transition` | Native source mode and the named source mechanics being replaced |

Each project record contains `project_id`, `import_id`, `source_id`, `source_ref`, `tip_oid` and `target_path`. Project identities are unique, source refs name heads or tags, and tips use `git:sha1:` identities. Target roots cannot overlap, including after case folding and Unicode normalization.

Consolidation requires completed imports matching the pinned source, tip and base. It composes their target subtrees into a new native revision with ordered unique imported parents. A target already present in the base is rejected.

Git import supports regular files, executable files, directories and symlink target bytes. Unsupported modes, including gitlinks, are rejected. LFS pointers require payload resolution before import. Non-UTF-8 paths or metadata are rejected rather than converted.

## 3. History and provenance without mandatory clones

Import workers stream source objects or bounded batches from an authorized repository endpoint or supplied archive. Native output is content-addressed; duplicate file bytes may reuse storage within the permitted authorization domain. Deduplication is not proof that histories, authorship or project identities are equivalent.

The importer records each configured repository identity and Git object identity alongside its native revision mapping. It archives original Git objects and retains author, committer, message and parent observations. Signed input is either rejected by policy or retained as `PRESERVED_UNVERIFIED` alongside the archived original object.

Imported revisions preserve source history without acquiring task or acceptance authority. Source-map lookup exposes the retained import mappings under the same repository/security scope.

The importer traverses only selected reachable history with bounded memory, disk, pagination and checkpoint state. A full import may read all selected history over time; no worker needs a full checkout or full history on local disk. Failed batches leave discoverable staged objects protected by upload/import leases and are resumed idempotently. Partially imported roots are invisible to ordinary accepted-source readers until their completeness checks pass.

After per-project histories are available, an immutable consolidation revision composes target project subtrees. It retains each project's original ancestry through source maps and, where representable, explicit native parent edges. It never pretends independent repository histories originally shared a parent. The consolidation manifest bounds the project list and resulting parent fan-in.

## 4. Pilot and parity

Parity compares each pinned source ref with its imported and consolidated native tree. It reads selected file bytes, modes and symlink targets, checks path mappings and retained evidence/access digests, and returns a `MATCHED` receipt bound to the manifest and consolidated revision. A source ref that has moved from its pinned tip is rejected.

Activation requires an authorized pilot acceptance naming the exact consolidated revision and root. Open workspaces, pending uploads, incomplete imports and unresolved publication effects prevent quiescence and activation.

## 5. Single-writer cutover protocol

The [cutover service](../../../tools/hyperfide/core/control-plane/src/source_control/migration/cutover.ts) records `SHADOW`, `READY`, `QUIESCED`, `NATIVE` and `ABORTED` states. Routing is recorded separately as `LEGACY`, `CLOSED` or `NATIVE`. Every transition is an idempotent metadata operation carrying operation identity, expected state/generation, source and target epochs, and immutable input manifest digest.

1. Prepare the manifest against completed imports with the exact pinned source, tip and native base. The resulting migration starts in `SHADOW`.
2. Consolidate the project subtrees and compare parity. Matching parity advances the migration to `READY`.
3. Quiesce at the expected generation. The service checks drained work, isolates configured legacy writers, repeats final parity, and records fresh isolation receipts with state `QUIESCED` and route `CLOSED`.
4. Activate at the expected generation with the pilot acceptance identity. The service repeats isolation, parity and drain checks, verifies the pilot's revision/root, advances the source-authority epoch and records state and route `NATIVE`.

Transitions retain operation identities, canonical input digests and replayable results. External writer isolation and source comparisons run outside the final metadata transaction; the transaction rechecks generation, authority and isolation freshness before committing.

## 6. Recovery and rollback

Abort is available in `SHADOW`, `READY` or `QUIESCED` at the expected generation when work is drained and no native revisions have been acknowledged. It records `ABORTED` with route `LEGACY` and restores admission through the configured legacy writer controls.

After activation, recovery requires the current `NATIVE` generation and drained work. It advances the authority epoch, adopts retained source bindings, and returns the preserved migration inventory. Existing native source and publication records remain retained.

The [backup](../../../tools/hyperfide/core/control-plane/src/source_control/backup.ts) and [recovery](../../../tools/hyperfide/core/control-plane/src/source_control/recovery.ts) services provide separate source-closure verification and restore-authority operations.

## 7. Migration interfaces

The [Git import service](../../../tools/hyperfide/core/control-plane/src/source_control/git/import.ts) supplies imported history and provenance. [Migration inventory](../../../tools/hyperfide/core/control-plane/src/source_control/migration/inventory.ts), [consolidation](../../../tools/hyperfide/core/control-plane/src/source_control/migration/consolidation.ts) and [parity](../../../tools/hyperfide/core/control-plane/src/source_control/migration/parity.ts) prepare the pinned inputs used by cutover. `HF_SOURCE_MIGRATION_CONFIG` selects protected migration controls; `HF_LEGACY_WRITER_CONFIG_FILE` identifies the explicit legacy writer topology.
