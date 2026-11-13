# Git remote interoperability

## 1. Existing implementation and transition boundary

The [Git adapter](../../../tools/hyperfide/core/control-plane/src/source_control/git/) implements object conversion, import, projection, transport and publication. [Export](../../../tools/hyperfide/core/control-plane/src/source_control/git/export.ts) constructs Git objects from native source; [transport](../../../tools/hyperfide/core/control-plane/src/source_control/git/transport.ts) sends the admitted update. [GitHub](../../../tools/hyperfide/core/control-plane/src/source_control/git/github.ts) and [GitLab](../../../tools/hyperfide/core/control-plane/src/source_control/git/gitlab.ts) provide remote adapters.

Publication retains exact source inputs, expected remote-head checks, destination authorization and uncertain-outcome reconciliation. Native acceptance and remote publication have separate receipts.

## 2. Projection identity and publication intent

A projection maps authorized native source roots to a Git destination. [SourceProjectionPolicy](../../../tools/hyperfide/core/control-plane/src/source_control/git/provenance.ts) defines its exact fields.

| Field | Supported value or meaning |
|---|---|
| `schema_version`, `policy_id`, `version` | Version 1, UUID policy identity and positive integer policy version |
| `project_id`, `repository_identity` | Logical project and remote repository identities |
| `mappings` | One to 32 `{source_path, target_path}` mappings |
| `dependencies` | At most 64 paths, treated as `INCLUDE` or `EXTERNAL_PIN`; external pins carry `revision_id` |
| `no_visible_change` | `KEEP` or `COLLAPSE_TO_FIRST_PARENT` |
| `merge_parents` | `ORDERED_UNIQUE` |
| `public_metadata` | `NATIVE_PUBLIC_V1` |
| `signature_policy` | `REJECT` or `PRESERVE_OBSERVATION` |
| `tag_signing` | `REJECT` |
| `object_format` | `sha1` |

Policy versions are immutable. Target mappings cannot overlap, including after case folding and Unicode normalization. Included dependencies must be within mapped source roots; external dependencies require explicit native revision pins.

[SourceGitDestination](../../../tools/hyperfide/core/control-plane/src/source_control/git/types.ts) binds source scope, destination ID, provider, repository identity, configuration digest, remote key, allowed refs and transport. Credentials remain protected server configuration.

A publication request pins `acceptance_id`, `destination_id`, `target_ref` and `expected_old_oid`. The resulting record binds source revision/root, generated Git OID, operation identity, generation and publication status. Native object IDs use `sha256:<64hex>`; projected Git IDs use `git:sha1:<40hex>`. They are distinct identity types.

## 3. Deterministic object construction

Export constructs blobs from exact file bytes, and Git trees from the projected paths, entry types, executable modes and child Git object IDs. Regular files and executable files map to modes 100644 and 100755; symlink target bytes map to mode 120000 without dereferencing the link. Gitlink mode 160000 requires an explicitly pinned external Git commit mapping and remains rejected until that capability is qualified. Device nodes, sockets, unsafe paths and unrepresentable native entry types fail with a path-specific diagnostic rather than disappearing.

The projection path policy uses the native canonical path representation. Legacy non-UTF-8 Git paths are preserved in import archives and reported; activation of an affected projection is rejected until a reversible, collision-free mapping is selected and qualified. Export must never replace undecodable bytes, normalize distinct paths into one, or follow symlinks while constructing source trees. Empty directories have no Git representation; their omission is recorded in the projection report, and a project requiring them must provide an explicit source placeholder policy.

Commit serialization pins tree ID, ordered parent IDs, author/committer names and addresses allowed by the public metadata schema, integer timestamps with timezone offsets, message bytes and encoding policy. Values derived at intent creation, such as a publication committer timestamp, are persisted once and reused on retry. Traversal order, retry time, worker identity and transient pack layout cannot change a generated commit ID. Native private task packets, session metadata, memory, credentials and conversation contents are outside the export schema; the public message and optional provenance trailer are explicit allowlisted fields.

Durable mappings retain native identities, projected Git OIDs, policy and publication lineage. Confirmed export checkpoints support incremental projection. Native source history remains keyed by native identities.

### Filtered history, merges and tags

With `COLLAPSE_TO_FIRST_PARENT`, a revision with one projected parent and an unchanged projected tree maps to that parent. `KEEP` retains a commit. Parent ordering is preserved and duplicate projected parents collapse deterministically. Merges with multiple distinct projected parents retain that ancestry.

Imported annotated commit tags retain their tagger, message and target provenance. Signed input follows the selected signature policy; preserved signature observations do not grant current authority. The supported tag-signing policy is `REJECT`.

## 4. Bounded transport and state machine

Initial export can traverse the complete authorized projection, but keeps bounded pages, memory and spool segments; it does not create a full checkout or retain a full Git clone. Incremental export reuses native/Git mappings and fetches only objects required by the destination. A changed entry in a huge Git directory can require reconstructing that directory's Git tree: this compatibility cost has its own budget and cannot force native commits to adopt Git's tree representation.

The native store contains no Git packfiles. The adapter emits a transient Git pack stream because Git remote transport requires that format. The first implementation may emit ordinary non-delta objects to keep generation deterministic and memory bounded; pack encoding is independent of object identity. Optional delta encoding requires its own measured bound. Spool files are explicit operation artifacts with size, ownership and retention limits. No native repack cycle, full-history checkout or mandatory shared Git object directory is introduced.

Publication states are `QUEUED`, `READY`, `DISPATCHED`, `OUTCOME_UNKNOWN`, `PUBLISHED`, `REMOTE_DIVERGED`, `REJECTED` and `CANCELLED`. Native acceptance remains a separate fact while a remote is unavailable. Publication backlog and retention have independent budgets.

1. Persist an intent and retention roots before generating or sending external objects. Read the current destination identity, capabilities and protection profile. Reject a changed repository identity or unauthorized target.
2. Validate exact native acceptance/evidence inputs and current export authorization, then finish the deterministic object construction manifest. Avoid a database transaction around storage downloads or transport.
3. Admit at most one unresolved mutation per destination/ref. Check expected old ref and generated ancestry; publish using the protocol's exact old/new/ref update precondition. Expected-old matching alone does not authorize non-fast-forward replacement.
4. Persist transport identity, credential lifetime and generation before sending. Credentials are least-scope, bounded-lifetime and unavailable to agent workspaces. Cancellation after dispatch requests reconciliation; it does not claim to retract an issued remote effect.
5. Verify the remote result and settle the durable receipt. Retain cleanup debt until credential revocation/expiry and operation cleanup are established. A crash after remote acceptance is reconciled from the same immutable intent.

A timeout or lost response enters `OUTCOME_UNKNOWN`. Observing the expected old head does not prove an already-issued push has stopped. The destination/ref remains fenced until transport and credential effects are settled. Observing the exact generated commit establishes an external source outcome only when bound to the admitted intent; observing a descendant requires a verified ancestry/publication chain and must not be reported as a fresh exact-head match. Unrelated changes enter `REMOTE_DIVERGED`. Recovery imports/proposes the external delta or creates an explicitly authorized repair intent; it never silently force-pushes or manufactures a successful receipt.

GitHub and GitLab implement one capability interface for identity, advertised object format, ref observation, protection evidence, send/readback, credential cleanup, and optional pull/merge request creation. Unsupported provider features are explicit capability failures. Provider capabilities and credential profiles are explicit. A pull/merge request URL is a collaboration artifact, separate from branch publication and protected-target acceptance.

## 5. Dependencies and public projections

Project export identifies the source roots and the closure of dependencies required to build or consume them. Each dependency is explicitly included from authorized roots, pinned as a public external dependency, or declared unavailable so publication fails the relevant project gate. The exporter cannot silently include another project's source to make a public build work. Shared code appearing in multiple exports is deduplicated internally but has projection-specific public path/history mappings.

Before publication, an export report identifies included/excluded roots, path remaps, external dependency pins, metadata fields and artifact digests. Native parent commits from excluded private projects are never attached to the public Git graph. The export role can read only the permitted source/history/projection inputs; content existence, cache keys or a native hash do not grant access to excluded objects. Garbage collection retains objects needed by active and uncertain export operations.

## 6. Incoming contributions

GitHub pull requests and GitLab merge requests are optional ingestion sources. An importer pins provider repository identity, ref/commit, object format, base mapping and observed contribution metadata. It streams and verifies selected objects into a quarantined import namespace without executing hooks, submodules, filters or repository code. It enforces byte, object-count, expansion and path limits and requires explicit policy for LFS payloads and gitlinks.

After validation, it creates a native change proposal with an immutable base, explicit path edits/deletions, contributor provenance and import receipt. Imported source is untrusted input; it undergoes the same scope authorization, qualification and acceptance as agent changes. Remote force-pushes produce another pinned proposal revision. They do not mutate an existing native revision or grant acceptance. Re-import is idempotent by provider identity, exact source commit and projection/base mapping.

## 7. Qualification evidence

Task 5.1 implements object conversion, export intents and adapters; task 5.2 implements projection/import provenance. The drivers exercise real Git hashing, strict fsck, receive-pack, deterministic retries, filtered history, annotated tags, symlinks/modes, empty-email identities, denied source, policy changes, remote divergence and lost publication responses. The GitHub canary publishes accepted native source to a private destination, independently reads it back, and reconciles a lost response without another push.

GitHub is the required hosted destination. GitLab remains an optional adapter with a separate live driver and credential profile. Each receipt retains its exact provider, source and execution scope; the canonical SCM-015 registration combines the required GitHub and local interoperability checks.
