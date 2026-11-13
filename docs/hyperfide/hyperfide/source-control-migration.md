# Native source migration

A migration pins a complete import for each project, its original Git repository and tip, its target path, and one native baseline. Imported history keeps its original provenance and object archive. It does not count as newly authored fleet work.

## Import and projection

Configure readable Git object databases in the owner-private file named by `HF_SOURCE_IMPORT_CONFIG`:

```json
{
  "sources": [
    {
      "id": "project-a",
      "scope": {
        "organisation_id": "<uuid>",
        "repository_id": "<uuid>",
        "security_domain_id": "<uuid>"
      },
      "repository_identity": "project-a",
      "path": "/srv/source/project-a.git"
    }
  ]
}
```

The importer reads bounded Git objects through `cat-file`; it does not check out code or execute repository hooks, filters or submodules. `source.projections.create` registers an immutable policy version. A policy declares source-to-target path mappings, dependencies, handling of unchanged commits and ordered merge parents, public metadata, and signature treatment. Changing a policy requires a new version.

`source.imports.create` pins the configured source, exact tip, policy version, fixed native baseline, and separate object/byte/commit/depth budgets. `source.imports.resume` commits bounded batches. `source.imports.read` reports progress; `source.imports.mappings` pages original-to-native commit mappings. Exact original Git objects remain retained archives. Unsupported modes, missing LFS content, unsupported encodings and signing policies produce explicit diagnostics.

`source.contributions.create` adds a provider contribution identity to those pins. Completion yields a native proposal. Normal qualification and acceptance decide whether that proposal becomes accepted source.

A publication destination may include a `projection_policy` using the same versioned policy shape. Publication revalidates each mapped path and uses the policy digest when reusing Git mappings. Annotated tag conversion keeps the original tagger, timestamp and message and resolves its exact native target; signed tag conversion is refused by the current policy.

## Consolidation and shadow parity

`source.migration.stage` composes a candidate before writer admission closes. Run the native pilot and independent qualification against that exact candidate, accept it, and queue its remote export. `source.migration.prepare` then takes the same versioned manifest with:

- `migration_id` and `base_revision_id`;
- projects containing `project_id`, `import_id`, `source_id`, `source_ref`, `tip_oid` and `target_path`;
- the complete `legacy_writer_ids` inventory and `legacy_source_epoch`;
- public revision provenance;
- `workflow_transition`, naming `source_mode: "native"` and only the legacy source mechanics being superseded.

At least two pinned projects are required. Overlapping or case-colliding roots are refused. The native write gate closes normal mutation admission while the migration is a shadow. Trusted historical ingestion can continue without granting ordinary workers a second source authority.

`source.migration.consolidate` composes the imported project subtrees over the fixed baseline and records their ordered native parents. `source.migration.parity` reads the pinned original Git objects and native objects independently, comparing complete selected file bytes, entry types, modes and ancestry mappings. It also checks the current configured source refs against the final pinned tips.

The manifest records the intended workflow transition. Preparing a manifest does not edit the repository workflow or activate its source mode.

## Quiesce and activate

Activation requires an explicit owner-private profile named by `HF_SOURCE_MIGRATION_CONFIG`:

```json
{
  "deployment_id": "<uuid>",
  "legacy_source_epoch": "<uuid-or-null>",
  "writer_id": "configured-legacy-writer"
}
```

`HF_LEGACY_WRITER_CONFIG_FILE` supplies the matching protected legacy topology used by Hyperfide's existing writer admission and kernel isolation observer. The profile must cover every writer in the manifest. Each declared writer requires its own configured topology and fresh isolation receipt; unknown writers are refused.

Before `source.migration.quiesce`, close active workspaces, finish active imports/uploads, and reconcile pending remote publications. Quiesce disables legacy admission and requires fresh kernel evidence that legacy storage aliases are read-only and no writable descriptors or shared mappings remain. It then records the route as `CLOSED`.

The observer skips defunct processes whose kernel stat state is `Z` or `X`, since they retain no descriptors or mappings. An unreadable live process still refuses isolation; the visibility diagnostic names its PID, proc file and error code.

`source.migration.activate` takes the pilot `acceptance_id`, verifies that its current evidence names the exact consolidated revision and root, repeats the final source-tip and isolation checks, and atomically advances both native source authority epochs with the route switch to `NATIVE`. Old source capabilities stop authorizing requests. Obtain a capability for the new epoch before continuing.

Every phase carries `migration_id`, a stable `operation_id`, and `expected_generation`. Reusing an operation recovers its recorded result. A different request under the same operation is refused.

## Abort and recovery

Before activation, `source.migration.abort` requires drained work and no acknowledged native writes. It closes native admission and restores the matching legacy admission descriptor. Restoring any external read-only mount is a separate operation under the configured deployment controls.

After activation, rollback to a stale legacy writer is refused. `source.migration.recover` preserves acknowledged revisions, source maps and retained archives, requires external operations to be reconciled, and advances to a fresh native authority epoch. `source.migration.read` exposes the active route, phase, generation and retained work inventory.

Epoch transitions create explicit successor policy and integration-domain IDs. The activation/recovery response contains `adoption.policy_map`, `adoption.domain_map` and versioned `adoption.source_bindings` for the new authority. Only current, unrevoked policies and checker registrations are carried forward; domains without an eligible policy are reported in `skipped_domains`. Historical domains, candidates, qualifications, and acceptance receipts retain their original identities and epochs.

Reading or publishing historical acceptance follows the recorded adoption chain to a current policy and domain, then revalidates the original evidence, including expiry and revocation. A new capability does not bypass revoked historical evidence. New changes must qualify and be accepted against the successor domain before publication.

If legacy refs advance during shadowing, isolate legacy storage, import each final tip under its existing projection policy, and call `source.migration.refresh` with the current generation and replacement manifest. Refresh changes only import/tip pins: project identities, target paths, refs, base revision, writer coverage and transition policy stay fixed. Every prior tip must remain in the new imported ancestry. The server repeats exact ref checks and native parity under observed isolation, records a linked immutable manifest generation, and leaves the route closed.

The refreshed consolidated revision requires its own independent qualification and acceptance before activation. Candidate, domain and assurance metadata remain available during this closed phase; ordinary authored revision creation remains fenced. The previous pilot cannot authorize activation of a changed root. `source.migration.read` returns the current manifest; immutable manifest history remains in `source_cutover_manifests` alongside the retained phase receipts.

## Inventory and retained evidence

`source.migration.read` returns the declared writer IDs, installed source/evidence table columns and observed record schema versions, selected project history counts (including collapsed and mapped commits), open workspace identities, custody holders, and unresolved publication operation identities. Empty schema-version lists mean no records of that kind have been retained. Counts and detailed inventories are bounded; overflow refuses cutover instead of truncating the drain decision.

Workspace `dirty_paths` conservatively lists writable paths. `dirty_paths_complete: false` explicitly means the server cannot observe a local overlay through metadata alone. Every open workspace blocks quiesce, even if it last reported a checkpoint. Preserve local files, checkpoint legacy work through its source repository, close its old workspace, and import the checkpoint in the final delta. After activation, open a new workspace at that imported revision and verify its content. Native API checkpoints must finish before preparing the shadow, while ordinary native mutation admission is still open.

Parity compares projection mappings with current native read/write path grants. Missing coverage refuses parity. Each project also compares its retained evidence's candidate, source revision, root, manifest and execution bindings against the canonical receipt bytes read from the object backend. Project and aggregate `access_digest` and `evidence_digest` fields bind those checks into the parity receipt. Additional retained project evidence or a revocation changes the digest; run parity again before quiesce after such a change.

Additional writers use `writer_profiles` in the private migration profile:

```json
{
  "deployment_id": "<uuid>",
  "legacy_source_epoch": null,
  "writer_id": "primary",
  "writer_profiles": [
    {
      "writer_id": "secondary",
      "legacy_writer_config_file": "/srv/private/secondary-writer.json"
    }
  ]
}
```

The primary topology still comes from `HF_LEGACY_WRITER_CONFIG_FILE`. Each additional profile is a separate private topology with the same deployment and source epoch, its own writer ID, and its actual canonical storage roots. All configured writers must exactly match `legacy_writer_ids`. Quiesce, refresh and activation require one fresh isolation receipt for each writer; unknown, missing and duplicate writers are refused. Abort restores admission for each configured writer.

Epoch adoption also records `source_bindings` for graph builds, findings and qualifications. Each binding records its unchanged source revision/root and original binding digest, `from_epoch`, `to_epoch`, and the preceding adoption digest. These immutable successor versions are retained individually in `source_audit` and in the operation response; original graph, finding and qualification rows remain historical. Findings bound to original imported Git IDs resolve through the retained import map; findings bound to native digest IDs resolve to that native revision. Optional binding schemas that are absent are explicitly listed in `uninstalled_binding_kinds`.

## Component crash evidence and deployment status

The 5.3 driver kills a real service actor with SIGKILL before and after the quiesce transaction commits, before and after the activate transaction commits, and after recovery commits. Each restart uses the same operation ID. It checks rolled-back or retained operation receipts, identical replay results, both authority epochs, readonly legacy roots and exactly one admitted source route. The fixture also preserves dirty workspace bytes through a real legacy checkpoint and a native import.

Inside the driver's private PID namespace, an init process reaps the case and orphaned descendants and returns the case's exit status. Root runs retain their existing user namespace so the isolation observer can inspect inherited process handles.

The receipt records the exercised `crash_boundaries`, component `status`, and a separate `deployment_migration: "UNRUN"`. A component PASS does not report a deployment migration as executed.
