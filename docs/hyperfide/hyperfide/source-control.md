# Native source-control service

The source service stores immutable content and persistent trees in S3, with repository, revision, operation and authorization metadata in PostgreSQL. Its online API is `POST /api/v1/source-control`; the SDK works without a checkout or Git history.

On the release builder, build the service from `tools/hyperfide/core/control-plane`:

```sh builder
npm run build
```

On a worker host, put the extracted runtime’s `bin` directory on `PATH`, supply the deployment environment described below, and start its packaged launcher:

```sh source
source-control
```

`HF_SOURCE_CONTROL_CONFIG` names an owner-only regular JSON file. TLS file paths resolve relative to that file. The server accepts Ed25519 **public** verification keys; the credential issuer retains signing keys.

```json
{
  "host": "0.0.0.0",
  "port": 9443,
  "trust": {
    "issuer": "https://identity.example.test",
    "audience": "hyperfide-source",
    "keys": {
      "source-1": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n"
    },
    "max_ttl_seconds": 300,
    "clock_skew_seconds": 0
  },
  "tls": {
    "certificate": "source.crt",
    "private_key": "source.key"
  }
}
```

The private key also requires owner-only permissions. Local development can explicitly set `"host":"127.0.0.1"` and `"allowLoopbackHTTP":true` instead of TLS. External listeners require TLS 1.3.

Supply the existing PostgreSQL connection in `DATABASE_URL`. The complete source-control migration catalog must already be applied through the migration lifecycle. The server checks schema presence when starting. Repository scope, source authority, capabilities and active principal credentials must be provisioned by the authorized control plane.

The S3 backend reads `HF_SOURCE_S3_ENDPOINT`, `HF_SOURCE_S3_REGION`, `HF_SOURCE_S3_BUCKET`, `HF_SOURCE_S3_ACCESS_KEY_ID` and `HF_SOURCE_S3_SECRET_ACCESS_KEY`; `HF_SOURCE_S3_SESSION_TOKEN` is optional. Existing `HF_ARTIFACT_S3_*` configuration is also supported. Source objects use their own `source/v1/` prefix and physical incarnations. Normal source reads follow pinned tree/object identities rather than bucket listings.

Set `HF_SOURCE_CURSOR_KEY` to the base64url encoding of 32–64 random bytes, and supply explicit positive workspace budgets:

| Variable                       | Unit                           |
| ------------------------------ | ------------------------------ |
| `HF_SOURCE_CACHE_BYTES`        | Immutable cache bytes          |
| `HF_SOURCE_CACHE_ENTRIES`      | Immutable cache entries        |
| `HF_SOURCE_MAX_FILE_BYTES`     | Materializable file bytes      |
| `HF_SOURCE_MATERIALIZED_BYTES` | Materialized workspace bytes   |
| `HF_SOURCE_DIRTY_BYTES`        | Retained dirty workspace bytes |

The listener emits a bounded `source-control.ready` event containing its address. SIGTERM and SIGINT stop new requests, drain the listener and close the PostgreSQL pool.

## SDK access

The issuer supplies a short-lived `HF-SOURCE-1` bearer token for the exact organization, repository and security-domain scope. Its current credential and capability records are checked in PostgreSQL.

```ts
import { SourceControlClient } from "@hyperfide/source-control";

const source = new SourceControlClient({
  endpoint: "https://source.example.test:9443/api/v1/source-control",
  scope: {
    organisation_id: organizationId,
    repository_id: repositoryId,
    security_domain_id: securityDomainId,
  },
  token: () => credentialProvider.currentSourceToken(),
});

const capabilities = await source.capabilities();
const revision = await source.revision(pinnedRevisionId);
const entry = await source.lookup(pinnedTreeId, "sdk/example/src/index.ts");
```

Mutations carry explicit operation and idempotency identities plus current source authority. Retain those identities across an uncertain response and query the original operation before retrying. Revision creation, qualification, acceptance and remote Git publication have separate receipts.

## Causal reads and batched metadata

Revision receipts carry a `causal_token`. Pass it through `SourceCallOptions` on a dependent read to require that committed revision under the same principal, scope and current authority epoch. The server verifies the retained operation, receipt and root storage before answering. Restored or advanced authority invalidates old tokens; unavailable content or expired authority produces an explicit refusal within the bounded validation deadline.

`source.batch({items, budget})` groups up to 16 metadata reads. Each item has a distinct `id`, an allowed read `action`, and its `input`. Authorization runs independently for every item. The result retains item identities and individual errors, with a shared maximum of 512 KiB and five seconds. Mutations, nested batches and raw object downloads require separate calls.

## Committed event subscriptions

`subscribe` binds a consumer to an immutable bootstrap revision and bounded retention. `pollEvents` discovers committed, unseen events; `acknowledgeEvent` uses the exact subscription epoch, event and lease. Late commits remain discoverable, and repeated acknowledgments preserve the original inbox outcome.

The default retention is 10,000 deliveries and 24 hours; explicit limits can be at most 100,000 deliveries and seven days. Expiry or exhausted delivery budget returns `RESNAPSHOT_REQUIRED` with the current boundary details. Read the desired immutable bootstrap and call `subscribe` with its revision and `expected_previous_epoch` to establish a new boundary. Bounded cleanup can return `RESNAPSHOT_PENDING`; retry the same request. Old subscription epochs cannot acknowledge the new boundary. Source event history remains immutable, and subscriber delivery work occurs when polling rather than on each source commit.

## Merge, rebase and revert

`source.transforms.merge`, `source.transforms.rebase` and `source.transforms.revert` operate on pinned revision IDs. Their SDK methods are `merge`, `rebase` and `revert`. Supply the operation and idempotency IDs through the SDK call options; the request body contains only transformation inputs. These operations require repository-wide source authorization and the capabilities used to read revisions, create revisions, resume transformations and inspect their results.

Transformation results include persisted `timing` for the last executed call step. `queued_ms` measures metadata admission, and `executing_ms` covers input reads and source calculation before the commit transaction; neither includes time waiting for a caller to resolve a conflict or resume a stack. Replays return the original measurement.

Merge uses three-way comparison of immutable tree identities, reuses unchanged subtrees, and merges bounded UTF-8 text edits. Conflicts retain typed references to each side, including binary, add/add, delete/modify, symlink, type and executable-mode conflicts. Rename detection is reported as `NOT_PERFORMED`. Multiple maximal merge bases require an explicit `base_revision_id`.

Rebase accepts an ordered linear `revision_ids` stack and an `onto_revision_id`. It retains each completed step and its original-to-successor mapping under the original change identity. `max_steps` bounds work per call. An unchanged replay maps back to its existing revision with `unchanged: true` and preserves its original history.

Inspect progress with `transformation(operationId)` and continue with `resumeTransformation({operation_id: operationId, max_steps: 1})`. A `CONFLICT` result includes an aggregate `conflict_id`; resolve it by passing `resolution: {conflict_id, resolved_tree_id, provenance}` to `resumeTransformation`. The resolved tree must already be uploaded and verified. Optional `expected_generation` rejects a stale continuation. Reusing a conflict identity with a different resolution is refused.

Revert applies the inverse of a pinned revision onto a pinned target. Reverting a merge requires `mainline_parent_revision_id`; reverting a root revision requires an explicit source patch. Cancellation through `source.operations.cancel` preserves already completed steps and prevents later commits. After a lost response, inspect the original transformation before resuming. New revisions require fresh qualification and acceptance.

## Qualification and acceptance

An authorized native operator or coordinator calls `registerAssurancePolicy` with the accepted run, task and specification generation. The server derives executable checks from that accepted proof registry. Register each independent checker with `registerAssuranceChecker`, naming its current native credential. These operations bind the policy and checker to the repository, run and authority epoch; callers cannot substitute arbitrary checks or a source token for a native identity.

An integration domain holds an accepted revision and generation. `createCandidate` pins the expected domain generation, candidate revision, ordered contributors, dependencies, and source, configuration, toolchain, graph and contract digests. Concurrent acceptance of another candidate changes the domain generation and rejects the stale candidate.

Registered independent checkers publish execution receipts through `publishSourceEvidence`. Receipts bind the exact candidate and inputs, command identity, observed exit status, output and artifact objects. The authorized control plane provisions checker registrations and policies; ordinary source writers cannot register themselves as checkers. Contributor custody and evidence objects remain pinned independently of the original worker.

`qualifyCandidate` verifies the registered checks. `acceptCandidate` rechecks the evidence, policy and custody in the same transaction that advances the domain and records the acceptance receipt. `candidate`, `domain` and `acceptance` inspect these separate states. Source acceptance does not complete a Hyperfide task.

## Speculative candidate batches

`enqueueCandidates` registers an explicit bounded set of proposals with a deadline and retry limit. `claimCandidateBatch` issues a lease using persistent fairness accounting across domains and original proposers. Limits cover actual contributor count, dependency references and serialized candidate bytes.

Build a fresh combined candidate with the exact ordered contributors, then call `bindCandidateBatch`. `completeCandidateBatch` requires that candidate's current qualification. A failed checker result goes through `failCandidateBatch`; `isolateCandidateBatch` partitions all original proposals into fresh child work that requires new candidates and new qualification. Original proposals, failed evidence and accepted source retain their identities. Queue completion does not accept source; use the separate acceptance API after qualification.

## Git publication

Publication accepts a native acceptance receipt, a configured destination ID, an allowed ref, and its exact expected old Git ID. Git IDs use the `git:sha1:` namespace; use `null` for an absent ref. Every newly exported revision requires current accepted-source authorization. A mapping confirmed at that remote checkpoint can reuse already-public history under current source-read and destination authority. Conversion preserves file bytes, executable modes, symlinks, public commit metadata and ordered parents. Git cannot represent empty directories.

`createPublication` records a queued intent. `resumePublication` projects source objects, streams a temporary Git transport pack, and reads the remote ref back. Confirmed mappings allow later publications to reuse remote objects. `publication` inspects the durable state; `cancelPublication` cancels before dispatch or reconciles an update that may already have reached the remote. Generic operation cancellation is refused for publications. An uncertain push retains its destination/ref reservation.

The native service stores source objects in S3 and metadata in PostgreSQL. Git packs exist only at the transport boundary. GitHub is the required hosted destination; the GitLab adapter is optional and requires its own qualification before use. A local adapter uses a configured Git repository for self-hosted destinations and integration tests.

Set `HF_SOURCE_PUBLICATION_CONFIG` to an owner-only JSON file containing trusted destinations:

```json
{
  "destinations": [
    {
      "id": "public-project",
      "scope": {
        "organisation_id": "11111111-1111-4111-8111-111111111111",
        "repository_id": "22222222-2222-4222-8222-222222222222",
        "security_domain_id": "33333333-3333-4333-8333-333333333333"
      },
      "provider": "github",
      "repository_identity": "owner/project",
      "remote_url": "https://github.com/owner/project.git",
      "allowed_refs": ["refs/heads/main"],
      "credential_file": "publication-credential.json"
    }
  ]
}
```

The credential file is also owner-only and contains `authorization_header` and `expires_at`. It is read when credentials are needed, allowing rotation without copying credentials into requests or source metadata. Relative file paths resolve against the configuration directory. GitLab destinations use `provider: "gitlab"` and a `https://gitlab.com/...git` URL. Local destinations use `provider: "local"` and `path` instead of `remote_url` and `credential_file`.

## Fleet source

Primary bootstrap requests may include `initial_revision_ids` in each `source_capabilities` entry when `source.revisions.read` is granted. The server verifies those explicit native revisions and issues a separate read-only discovery capability. Primary observations expose their scoped revision/root identities and a permission-checked bind request template; the model chooses and dispatches the binding operation. The ordinary source capability keeps its original mutation scope.

The Primary binds a run to an explicit native revision with `source.fleet.bind-run`. `selectFleetSource` advances the run to an accepted descendant using an explicit generation check and a qualified predecessor. Existing attempts retain their original source; new installations and assignments use the selected revision. Register the next task’s proof policy against that revision before dispatch. Changing an already registered policy requires a new specification generation.

The active model-led assignment carries its source scope, base revision, environment and package digests, task and attempt generations, and a private workspace identity. `source.fleet.assign` issues its narrow source capability after checking the admitted assignment. Fleet authority actions run through the authenticated control-plane component and its transaction gateway.

`nativeFleetManifest` reads selected native paths with explicit file, byte and traversal budgets. The manifest includes verified file descriptors and content, capped at 64 MiB of file bytes and 96 MiB encoded. The existing `hyperfide install` pipeline uses that manifest for source, dependency, configuration and interface preparation. Native selection never falls back to Git. API-only installation carries source identity without files.

Every installed worker view retains its readiness-attested `.neron` bootstrap configuration. An active assignment supplies a separate workspace ID and current capability, so reusing a physical slot does not reuse an old worker's source authority. Workers use that assignment for checkpoints and `submitFleetSource`; `fleetAssignment` and `fleetSubmission` inspect the retained state.

Native credentials use the control service's signed `POST /api/v1/source-control` endpoint. Requests carry the normal source envelope and an Ed25519 proof over its exact bytes, endpoint and origin. The endpoint checks the current credential, source capability and task authority. Object transfers accept up to 12 MiB encoded; ordinary metadata requests retain their 1 MiB bound.

The native launcher completes the source assignment from the recorded manager decision before starting the worker, then provides its private `source-client.json` through `HF_SOURCE_CONFIG`. Its `transport` object contains `schema_version`, `endpoint`, `scope`, `credential_id`, `ca_file` and `key_file`; its `assignment` object carries the active workspace and authority. Credentials are file references, not embedded key material.

Workers can send an explicit versioned source request with:

```sh source
hyperfide source invoke --request-file request.json
```

`--config /absolute/path/source-client.json` selects a configuration explicitly. The command verifies the request scope and adds the assigned authority when the request omits it. Keep the original `operation_id` and `idempotency_key` when reconciling an uncertain response. The Node SDK exposes the same transport through `createSignedSourceClient(config.transport)` from `@hyperfide/source-control/signed`; pass `config.assignment.authority` in its call options for worker mutations.

Submission accepts only the exact durable checkpoint of the assigned workspace. It pins the revision in coordinator custody before releasing compute in the same transaction. Repeating the original submission recovers its receipt after a lost response. A replacement attempt cannot write through its predecessor's assignment. Native cleanup uses the retained submission and the runner's verified process and credential cleanup to release the worker slot.

`qualifyFleetTask` is a Hyperfide authority operation: it verifies current task proof obligations, custody, candidate inputs and independent execution evidence before advancing the task to qualified. Dependency admission rechecks the proof. Native acceptance and Git publication remain separate source states.

## Packaged runtime

The [native runtime package](../runtime/source-control.md) ships the service, SDK, host adapters and native clients with a checksummed dependency closure. Worker hosts consume the extracted runtime without compiling the repository. The package smoke driver exercises API-only and sparse source, checkpointing, independently checked acceptance and disposable Git publication; the final release gate additionally requires full-case evidence and operator acceptance.

## Graph freshness

Configure `HF_SOURCE_GRAPH_BINARY`, `HF_SOURCE_GRAPH_BINARY_DIGEST` and `HF_SOURCE_GRAPH_SCRATCH` together. `source.graph.build` pins the native revision and root, selected paths, parser executable, resolver and build configuration. An overlay also pins its workspace generation and checkpoint revision; changing or closing that workspace refuses the build with `GRAPH_OVERLAY_STALE`.

Inspect `source.graph.query` in `status` mode before using symbols, context or dependencies. Supply the exact build, revision and root identities. `coverage.complete` and `resolution_complete` are separate observations; unresolved imports and uncovered paths remain explicit. A graph channel’s generation or a prior successful scan does not establish freshness for a different revision. Publish with `source.graph.publish` and the expected channel generation. Rebuild after source, parser, resolver or configuration changes; graph coverage alone cannot qualify a candidate.

## Retention

`source.retention.configure` pins the backend identity, minimum object age, quarantine delay and backup hold. Independent bucket expiration must remain disabled. Retained revisions and history, active workspaces, uploads, coordinator custody, candidate evidence, imports, publications, backups and explicit holds protect their exact object incarnations. Immutable cache eviction does not discard the dirty workspace journal.

Use `source.retention.acquire` and `source.retention.release` for explicit roots. Collection proceeds through bounded `source.collection.discover`, quarantine, pin recheck, exact-incarnation delete intent and reconciliation. `source.collection.inspect` reports pending work. Preserve the original operation ID after uncertainty; neither an old reference count nor a delayed deletion authorizes removal of a newer incarnation. Resolve admitted deletion work before changing retention policy. Keep metadata and object retention at least as long as every supported backup and recovery point.

## Restore procedure

Use the [recovery runbook](source-control-recovery.md) with an operator-owned profile matching the [recovery schema](../../../tools/hyperfide/deploy/hyperfide/source-control/recovery-profile.schema.json). A structurally complete profile is a declaration; only an executed recovery campaign can qualify its failure model and RPO/RTO.

1. Stop write admission and fence former writers using the independent external authority journal. Preserve that journal outside PostgreSQL backup restoration, along with original upload, delete and publication identities.
2. Select a verified version-2 backup manifest and restore its metadata, placements, routing, retained roots and object closure together. Native source format 1 and backup manifest version 2 are independent; a legacy backup returns `BACKUP_FORMAT_UNSUPPORTED`.
3. Establish the current external authority and call `source.recovery.prepare`. Writes remain fenced. Run `source.backups.verify` to completion against actual object storage; copied READY metadata cannot replace verification. Missing content or an expired recovery point refuses readiness.
4. Call `source.recovery.reconcile` with the same preparation operation and a bounded limit until no captured effects remain. Retain unknown outcomes and reconcile the actual remote state; do not upload again or push a replacement publication.
5. Call `source.recovery.activate` only after closure and external-effect reconciliation succeed. Obtain credentials for the successor authority epoch, verify retained content and operation replay, then reopen writes. An older database or prior-epoch capability cannot authorize recovery writes.

## Profile qualification

This table covers every required and optional variant in the current [claims inventory](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json). **UNQUALIFIED** means its full-case qualification is UNRUN at the documented inputs, even where focused component receipts exist. No live S3, FUSE, GitHub/GitLab, fleet, scale, endurance or disaster-recovery guarantee follows from a package smoke result. A successful 7.1 run qualifies only its recorded packaged component checks; its private receipt names the exact input digest and executed documentation commands.

| Profile                                                           | Variant               | Qualification | Case                                                                                          |
| ----------------------------------------------------------------- | --------------------- | ------------- | --------------------------------------------------------------------------------------------- |
| Canonical immutable object and tree identity                      | full                  | UNQUALIFIED   | [SCM-001](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-001) |
| S3 uploads and durable revision boundaries                        | real-s3               | UNQUALIFIED   | [SCM-002](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-002) |
| Idempotent native operations and divergence                       | full                  | UNQUALIFIED   | [SCM-003](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-003) |
| Attempt fencing and online authority                              | full                  | UNQUALIFIED   | [SCM-004](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-004) |
| Retention and garbage-collection races                            | full                  | UNQUALIFIED   | [SCM-005](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-005) |
| API-only history queries and pinned pagination                    | full                  | UNQUALIFIED   | [SCM-006](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-006) |
| Committed event replay and resynchronization                      | full                  | UNQUALIFIED   | [SCM-007](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-007) |
| Server merge rebase and revert                                    | full                  | UNQUALIFIED   | [SCM-008](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-008) |
| Atomic acceptance of exactly qualified candidates                 | full                  | UNQUALIFIED   | [SCM-009](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-009) |
| Sparse materialization and writable-cache isolation               | full                  | UNQUALIFIED   | [SCM-010](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-010) |
| Virtual workspace coherence and bounded cache                     | full                  | UNQUALIFIED   | [SCM-011](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-011) |
| Incremental source-pinned graph coverage                          | full                  | UNQUALIFIED   | [SCM-012](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-012) |
| Scoped source authorization and export privacy                    | full                  | UNQUALIFIED   | [SCM-013](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-013) |
| Hyperfide task source custody and taskless clients                | hyperfide-fleet       | UNQUALIFIED   | [SCM-014](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-014) |
| GitHub and GitLab remote export reconciliation                    | github                | UNQUALIFIED   | [SCM-015](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-015) |
| GitHub and GitLab remote export reconciliation                    | gitlab                | UNQUALIFIED   | [SCM-015](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-015) |
| Contribution and history import provenance                        | full                  | UNQUALIFIED   | [SCM-016](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-016) |
| Monorepo migration cutover and writable rollback                  | cutover               | UNQUALIFIED   | [SCM-017](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-017) |
| Supported service and metadata failure recovery                   | metadata-failover     | UNQUALIFIED   | [SCM-018](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-018) |
| Backup restore and external-effect reconciliation                 | restore               | UNQUALIFIED   | [SCM-019](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-019) |
| Growth invariance and within-repository sharding                  | growth                | UNQUALIFIED   | [SCM-020](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-020) |
| Concurrent mixed-load and admission profiles                      | mixed-load            | UNQUALIFIED   | [SCM-021](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-021) |
| Concurrent mixed-load and admission profiles                      | open-admission        | UNQUALIFIED   | [SCM-021](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-021) |
| Real S3 and elapsed-time endurance                                | real-s3-72h           | UNQUALIFIED   | [SCM-022](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-022) |
| Real S3 and elapsed-time endurance                                | real-worker           | UNQUALIFIED   | [SCM-022](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-022) |
| End-to-end native Hyperfide source-control journey                | api-cli-ui            | UNQUALIFIED   | [SCM-023](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-023) |
| Strict harness and independent evidence closure                   | full                  | UNQUALIFIED   | [SCM-024](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-024) |
| Category layout and existing-system behavior preservation         | full                  | UNQUALIFIED   | [SCM-025](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-025) |
| Manifest and interface schema with source-bound subset resolution | full                  | UNQUALIFIED   | [SCM-026](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-026) |
| Provider sandbox install and worker isolation                     | provider-sandbox      | UNQUALIFIED   | [SCM-027](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-027) |
| Primary and Wave Manager adaptive orchestration                   | primary-wave-managers | UNQUALIFIED   | [SCM-028](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-028) |
| Supervisor loss and final prerequisite evidence closure           | supervisor-failover   | UNQUALIFIED   | [SCM-029](../../../tools/hyperfide/tests/hyperfide/source-control/claims.json#/cases/SCM-029) |

## Documentation execution contract

Fences labelled `sh source` are executable source examples. The 7.1 driver extracts every non-comment line as an argument vector and resolves its executable exclusively from the extracted runtime’s `bin` directory. It requires service readiness and a clean shutdown, and executes `source invoke` against the packaged signed HTTPS control service with a private configuration and a real scoped `source.capabilities` request in `request.json`. Supply the equivalent current request and `HF_SOURCE_CONFIG` when using that example in a deployment. Shell operators, substitutions, unlabelled runtime examples, unknown executables and failed commands are refusals. Builder and release-closure fences require their separate environments.

The declarations below pin current source references. Evidence references, when present, require a content-pinned PASS receipt for this exact packaged input closure and executed command inventory. No historical receipt is declared current here. Recovery-profile references must validate against the packaged schema and its retention constraints; their validity does not promote a profile to qualified. Raw receipts and deployment profiles remain private.

```json source-release
{
  "schema_version": 1,
  "source_refs": [
    {
      "path": "tools/hyperfide/tests/hyperfide/source-control/registry.json",
      "sha256": "082f41fff8be19a89b0593349d376a15fc7f9c0c60a211a4012213dd56e85d0f"
    },
    {
      "path": "tools/hyperfide/tests/hyperfide/source-control/claims.json",
      "sha256": "c305a628a204c85f9deeafb6a768032d60535e469a887bf95a5c833436ec65e6"
    },
    {
      "path": "spec/layerx-merge/source-control/source-spec.kvx",
      "sha256": "c8474867ab2767e5d8ac9f4fd2b04160bd26f51c70ae5b59175f9f8878b33f7b"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/contracts.ts",
      "sha256": "3b5e07ce2d65a10293ae78660aac10dd31310ee813e35b09cf0414e6fdd3aa7d"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/graph-adapter.ts",
      "sha256": "290fbff086a0277035a196fd0202bc276f07ede7e38157d481cbfad4caf1dc4b"
    },
    {
      "path": "tools/hyperfide/core/control-plane/src/source_control/recovery.ts",
      "sha256": "11c7d9d3c633cb999c60602868094300131d2e6899db44f90a4aceaf03d9857b"
    }
  ],
  "evidence_refs": [],
  "recovery_profiles": []
}
```

## Running release closure

The [7.2 release manifest template](../../../tools/hyperfide/tests/hyperfide/source-control/manifests/release.json) uses schema version 2. It pins the 7.1 archive and package manifest, complete source identity, closure packet, private runtime profile, running-system receipt and explicit operator acceptance. Its sentinel pins require replacement with actual evidence. The [operations procedure](operations.md#running-source-release-and-restore-drill) describes the exact private profile, restore boundary and acceptance sequence.

The deployment phase starts the extracted `bin/source-control` and uses the packaged SDK/host modules through `native_source_full_journey.mjs`. It revalidates actual SCM-023 assignment, replacement, retained checkpoint, qualification, custody and accepted-source records, then exercises a new sparse checkpoint and an ambiguous publication. The original operation ID survives the deliberately dropped publication response. Receiver Git readback and the reflog require exactly one ref update after resume and replay. The restore drill restarts the packaged service against restored PostgreSQL/object metadata and compares accepted roots, canonical object bytes and remote state with observations taken before the drill. This running-system receipt supplements the canonical SCM-023 API/CLI/UI campaign.

Closure requires nonempty, separately listed evidence classes:

| Class         | Required evidence                                                      |
| ------------- | ---------------------------------------------------------------------- |
| Component     | Passing 7.1 packaged runtime receipts for the pinned archive           |
| Live backend  | SCM-002 `real-s3`, SCM-022 `real-s3-72h`                               |
| Live fleet    | SCM-014 `hyperfide-fleet`, SCM-022 `real-worker`, SCM-023 `api-cli-ui` |
| GitHub        | SCM-015 `github`                                                       |
| Consolidation | SCM-017 `cutover`                                                      |
| Growth        | SCM-020 `growth`, SCM-021 `mixed-load` and `open-admission`            |
| Recovery      | SCM-018 `metadata-failover`, SCM-019 `restore`                         |

Full evidence class entries name the exact case, variant, receipt ID and receipt hash independently verified from the closure packet. A component receipt cannot replace any of these entries. The packet must still cover every canonical case and required variant, including those outside this table. Operator acceptance binds the runtime receipt and profile as well as the source revision and packet. Closure rechecks current retained custody; a previously passing receipt cannot hide a revoked, superseded or differently qualified source.

The release checker validates input pins, evidence classes and current custody, source, generation, policy and qualification. Full closure records every required variant’s outcome and the separate evidence classes in its closure report.
