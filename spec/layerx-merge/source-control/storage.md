# Native source storage and durability

## 1. Storage services

[SourceObjectService](../../../tools/hyperfide/core/control-plane/src/source_control/objects.ts), [PersistentTree](../../../tools/hyperfide/core/control-plane/src/source_control/trees.ts) and [SourceMetadata](../../../tools/hyperfide/core/control-plane/src/source_control/metadata.ts) implement the storage and metadata boundary. The [S3 adapter](../../../tools/hyperfide/core/control-plane/src/source_control/s3.ts) handles immutable object transfers. Object identity and request formats are shared through the [SDK contracts](../../../tools/hyperfide/sdk/source-control/src/contracts.ts).

Object bytes and transactional metadata have separate responsibilities. Native identities remain distinct from Git OIDs and legacy local snapshots. Project roots are logical source scopes rather than physical storage partitions.

## 2. Immutable objects and canonical identity

An object is addressed by `sha256:<64 lowercase hexadecimal digits>`. Its digest covers a type-separated, versioned canonical envelope. The v1 envelope is ASCII `hyperfide-source-control`, NUL, ASCII `v1`, NUL, kind, NUL, canonical decimal payload length, NUL, then payload bytes. The supported kind set is closed and versioned. Object identity is independent of S3 key spelling, upload time, compression, encryption, backend version ID and ETag.

Blob and chunk payloads are exact bytes. File descriptors and tree nodes use tuple encodings with fixed field positions, UTF-8 strings, decimal strings for large integers and canonical JSON array serialization. Canonical fixtures cover escaping, Unicode, empty values, integers, type separation and invalid input. Decoders validate the tuple shape, canonical serialization and field bounds.

The object service accepts the following upload kinds:

| Kind | Logical contents | References |
| --- | --- | --- |
| `blob` | Exact small/ordinary file bytes | None |
| `chunk` | A bounded range of a large file | None |
| `file` | Encoding version, logical byte length, whole-file payload digest and ordered chunk IDs/lengths, or a blob ID | Content children |
| `tree-leaf` | Directory-prefix discriminator and sorted name/type/mode/object-ID entries | File, symlink-content or child directory root |
| `tree-index` | Radix prefix and sorted non-overlapping child ranges | Tree nodes |

The protocol kind enumeration also includes `manifest` and `revision`; the object service returns `UNSUPPORTED_OBJECT_KIND` for uploads of either kind. [Revision metadata](../../../tools/hyperfide/core/control-plane/src/source_control/metadata.ts) is stored separately. Its identity hashes a `revision` envelope containing a tuple of schema version, source scope, root tree ID, ordered parent revision IDs and public provenance (author name, author email, message and timestamp).

Stable change IDs and authorization identities are scoped metadata identities, not content hashes. Large mutable indexes and runtime status do not go inside a source object. Public source provenance and private fleet context have separate schemas and references.

Native paths are repository-relative UTF-8 sequences. The format rejects NUL, absolute paths, empty components, `.` and `..`; it preserves exact permitted bytes and never silently folds case or normalizes Unicode. Directory comparison uses unsigned UTF-8 byte order. The importer records unsupported names rather than silently renaming historical files. A client whose filesystem cannot represent a valid native path reports a typed incompatibility or uses API access. Executability, file kind and symlink target are explicit; timestamps, UID and host inode numbers are not source identity. Symlink resolution policy belongs to the materializer and must not permit path escape.

The ordinary commit path accepts only bounded objects. Tree leaves contain at most 256 entries and encoded tree nodes at most 256 KiB. The blob threshold and chunk size are both 4 MiB; encoded objects are bounded at 8 MiB. These are versioned format parameters; deployment settings cannot reinterpret old file descriptors. The file descriptor supports exact reassembly and range queries, including zero-byte files and a final short chunk.

## 3. Persistent trees and partitioning

A directory is a deterministic radix map over its entry-name bytes. Leaves split at a specified bound and carry enough prefix information to reconstruct one canonical representation; indexes have non-overlapping ranges. The complete split/merge algorithm and empty-root encoding are golden-vector contracts. Updating a few entries rewrites the affected radix paths and ancestors, sharing unchanged subtrees. An enormous single directory must not become one enormous object.

The [routing service](../../../tools/hyperfide/core/control-plane/src/source_control/routing.ts) selects one of 16 partitions from the first digest byte modulo 16, within the organisation, repository and security domain. A route records its owner and generation. Moving a route checks the expected generation and records an idempotent ownership change over shared immutable placements. Object IDs and placement keys stay unchanged. A request to an obsolete owner returns `STALE_ROUTE` with `REFRESH_AUTHORITY`.

Each accepted tree is verified inductively. The service checks the changed nodes' canonical encoding, bounds, paths and direct child readiness receipts. An unchanged child root already has a verified transitive closure. This avoids rewalking all files at each revision. A corruption audit may traverse broader retained roots as a separately admitted, resumable maintenance operation. Traversal/query workers carry time, byte, node and memory budgets and return continuation handles instead of silently truncating answers.

## 4. Physical S3 layout and access

The [object catalog](../../../tools/hyperfide/core/control-plane/src/source_control/object_catalog.ts) uses keys `source/v1/<security-domain>/<kind>/<digest-prefix>/<digest>/<incarnation>`, where `digest-prefix` is the first two hexadecimal digest characters and `digest` omits the `sha256:` prefix. The digest identifies logical content; `incarnation` identifies a physical placement generation and is not part of the content identity. A catalog maps each logical ID to its verified placement and lifecycle generation. Deletion addresses the exact incarnation.

Deduplication is confined to an explicit security domain. A hash is an identity and integrity check, not a read capability. Clients cannot issue unrestricted bucket LIST or guess arbitrary object keys. An authenticated API issues short-lived grants for identified objects or upload sessions, or streams through a gateway. Direct transfer grants bind domain, object/key generation, method, size/checksum bounds and expiry; durable credentials stay with trusted services.

Normal reads and workspace population resolve pinned source roots. They never discover repository membership by bucket listing. Caches verify content against the native identity and track placement availability separately from immutable source identity. Reads use a verified available placement or return a typed unavailable result.

AWS documents strong consistency for object reads/writes and atomicity per key, but no cross-key transaction: [S3 consistency model](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html). Conditional create prevents blind overwrite: [conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html). S3-compatible endpoints require their own protocol qualification; a matching HTTP shape is insufficient. Concurrent transfer and range reads are supported as bounded optimizations: [performance guidelines](https://docs.aws.amazon.com/AmazonS3/latest/userguide/optimizing-performance-guidelines.html). These statements do not establish fleet throughput, regional replication latency or cost.

## 5. Upload and publication protocol

1. `source.uploads.prepare` authenticates scope and creates an idempotent upload session with a service-clock expiry. Its input contains `kind`, `object_id` and `size_bytes`; the request envelope supplies the scope and operation identity. The session pins its prospective content against collection. Managed workspace clients send bytes through `source.objects.put`.
2. The uploader writes to a fresh physical incarnation using conditional creation. Large payloads use a tracked multipart upload. Parts do not make an object readable; completion and validation do. Retries observe and verify an existing matching object rather than replacing it. Conflicting bytes at an expected identity are an integrity failure.
3. The verifier reads the bounded canonical envelope and checks its length and digest. A backend ETag, successful HEAD or client-supplied digest alone does not prove native content. A lost PUT/completion response leaves an `UNKNOWN` upload outcome until reconciliation.
4. The catalog marks the placement `VERIFIED` after format/type validation and direct-child protection where applicable. Verification reads the configured object backend. That state is distinct from a revision becoming authoritative.
5. The revision service uses short metadata transactions to validate the current attempt/custody where applicable, the idempotency binding, root readiness and durable pin receipts, then records the immutable revision and committed outgoing event intent. Upload/network/merge/build work never occurs inside that transaction. A retry with the same operation and request digest returns its stored result; a different digest is rejected.
6. Only that committed result acknowledges a native checkpoint/revision. The acknowledgment states the durability profile. Native acceptance and remote Git publication remain additional states. A client retains pending local writes until acknowledgment or explicit abandonment; cache eviction cannot discard them.

A client killed after upload but before metadata commit leaves protected orphan objects, not a partial revision. A server killed after metadata commit but before response returns the recorded result on retry. Failure to verify one child prevents root publication. The metadata API never exposes a snapshot whose closure contains unverified or collected objects.

## 6. Reference safety and garbage collection

Reference ownership is explicit. Each newly verified parent object holds durable pins for its direct child incarnations. The catalog records roots for acknowledged revisions/history, active workspaces, submitted custody, candidates, qualification evidence, export/import operations, retention policy, backups and explicit holds. Parent readiness requires all direct-child protection receipts, so transitive closure safety follows inductively without a fresh whole-tree walk.

Within one metadata owner, publishers/read-grant issuers and collectors synchronize through an object lifecycle generation with short per-object transactions. A publisher obtains a pin only while the target incarnation is referenceable; collection atomically stops new pins before physical deletion. An indexed pin/reference ledger is authoritative. Cached reference counts are hints until validated under the lifecycle fence.

Across metadata owners, child-pin reservations are durable idempotent operations keyed by parent construction identity and child position. A parent becomes referenceable only after every reservation is acknowledged. Reservations remain protective if the parent outcome is unknown. Reconciliation adopts or releases them using persisted parent outcome; age alone cannot release a reservation whose parent may have committed. No cross-shard network call runs while a local SQL transaction is open.

| Lifecycle state | May accept new references? | Collector action |
| --- | --- | --- |
| `UPLOADING` | Upload session only | Reconcile outcome and active lease before abandonment |
| `VERIFIED` | Yes, under current generation | Consider only when no roots, incoming parent pins, read pins or holds remain |
| `QUARANTINED` | No; explicit atomic rescue allowed only before deletion is authorized | Wait grace interval, recheck pin ledger and holds |
| `DELETE_PENDING` | No | Reconcile deletion of the exact physical incarnation |
| `ABSENT` | No | Release outgoing child pins, retain tombstone/reconciliation record |

A collector first persists a deletion intent and generation, then calls S3 outside the transaction, then observes the exact physical incarnation. A lost response remains cleanup debt. Outgoing child pins are released only after parent absence is reconciled, preventing premature child collection. Re-upload after deletion uses a fresh incarnation. A stale collector cannot address that new key.

A reference arriving during quarantine either wins the rescue transaction before `DELETE_PENDING` or fails with a retryable reclaimed-object result and uploads/pins a new incarnation. A reference cannot merely clear a cached tombstone. Short-lived read/export grants hold a pin for their validity interval plus a bounded delivery margin. Lease renewals and pin release use service time and idempotent identities. Backup retention roots protect the catalog restore horizon, including object versions needed by an older metadata backup.

Collection is incremental, partitioned, resumable and independently budgeted. It does not require bucket-wide listing, global tracing or Git repacking on the foreground path. Conservative leaks are visible debt; missing referenced source is a failed invariant. Administrative or automatic S3 lifecycle deletion may affect only classes proven outside retained source roots, multipart debris and explicitly disposable derived artifacts. Bucket versioning is an additional recovery facility, not a substitute for reference safety or a claim that bytes have been erased.

## 7. Durability, replicas and restore

The recovery profile declares metadata acknowledgment (`FSYNC_PRIMARY` or `SYNC_REPLICA`), failure domains, object placements, backup retention, RPO and RTO. `SYNC_REPLICA` requires declared replica failure domains. The profile accepts `PROCESS_LOSS` and `PRIMARY_INSTANCE_LOSS`; declaring `FAILURE_DOMAIN_LOSS` returns `RECOVERY_PROFILE_UNQUALIFIED`. Recovery activation checks PostgreSQL fsync and synchronous-commit settings, and requires a streaming synchronous replica for `SYNC_REPLICA`.

Metadata failover fences the old writer before a new authority accepts mutations and advances an authority epoch. Catalog, pins, revision results and event intents recover together. Read replicas may serve immutable queries after the required readiness barrier, but cannot issue source ownership or acceptance. Cross-region object replication is observed explicitly and does not silently upgrade a single-region acknowledgment.

A backup consists of a consistent metadata checkpoint, routing/codec/policy versions, retained source roots and verified availability of their object closures. Restore loads metadata into a new authority epoch, verifies roots against preserved placements, reconciles incomplete upload/deletion/export intents, and reauthorizes transfer grants before writes resume. If some retained object is missing, affected roots report unavailable and restoration fails its completeness gate. Cache contents and a copied worker disk are not backups.

The [backup service](../../../tools/hyperfide/core/control-plane/src/source_control/backup.ts) captures revision and retained-root inventories, verifies object placements in bounded batches and reconciles pending external effects during recovery. The [recovery profile](../../../tools/hyperfide/core/control-plane/src/source_control/recovery.ts) supplies metadata, placement and retention requirements. Security and destructive-operation authority are specified in [identity-security.md](identity-security.md).

## 8. Resource observations

The [metrics service](../../../tools/hyperfide/core/control-plane/src/source_control/metrics.ts) returns scoped samples for objects, uploads, operations, publications, imports, graphs and transfers. `max_rows` defaults to 1024 and accepts 1–4096 rows per family. Results carry `semantics = "BOUNDED_OBSERVATIONS"`, coverage and observed counts. Persistent-tree operations report `nodes_read` and `nodes_written`; these describe the tree work of that operation.
