# Hyperfide integration and product surfaces

## 1. Existing seams

The [fleet adapter](../../../tools/hyperfide/core/control-plane/src/source_control/fleet-adapter.ts) binds native workspaces, checkpoint generations and result custody to task attempts. [Run-source bindings](../../../tools/hyperfide/core/control-plane/src/source_control/fleet-run-source.ts) and [source selection](../../../tools/hyperfide/core/control-plane/src/source_control/fleet-source-selection.ts) pin native inputs for dispatch. The [graph adapter](../../../tools/hyperfide/core/control-plane/src/source_control/graph-adapter.ts) supplies source-pinned graph access.

The [source-control plugin](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) exposes the shared client through the app and CLI. Legacy source environments retain their own format and authority bindings.

## 2. Authority ownership

Hyperfide owns task assignment, attempt generations, execution budgets, qualification policy, review and dependency-release policy. Source control owns immutable revisions, authorized workspace mutations, valid source acceptance and export operation state. The source engine serves taskless callers as well as fleet agents. It has no second task scheduler and cannot mark a task done merely because a revision was accepted.

Source requests carry authenticated principal, repository, operation identity and optional task/attempt association. The source service validates an authority-issued capability or performs a current authority check according to its pinned policy. Workspace generation, source authority epoch and attempt generation are checked server-side for each mutation. A local plugin KV entry, thread state or client cancellation is never the write fence. An authority epoch change invalidates grants according to the recovery contract without rewriting historical revisions.

A workspace's read scope, write scope and execution limits are explicit. Shared build inputs may be readable and immutable without being writable. A manager receives source API access and can request candidate operations without a full checkout. Integration and review jobs materialize only what their declared tooling needs. Cross-project revisions retain one source transaction even when Hyperfide schedules affected project checks independently.

## 3. Task-to-source lifecycle

1. Dispatch pins the spec revision, task/attempt generation, exact base source revision, write scope, dependency/build-closure manifest, qualification policy version and environment profile. Worker images supply tools; their baked source is not authoritative for the assignment.
2. The host opens a native workspace through the source API. Its readiness receipt names the actual mode, scope and base revision. Hyperfide records that receipt before launching the worker. An API-only job skips filesystem provisioning.
3. The worker edits its overlay or uses source APIs. Durable checkpoints return immutable revision receipts. Heartbeats and telemetry may report local dirty progress, but checkpoint acknowledgment requires server-held content and revision metadata.
4. Submission first validates current compute ownership and transfers an exact revision set plus evidence references into coordinator-owned result custody. It records the source roots that retention must preserve. A repeated submission returns its original receipt. After this transfer, worker cleanup or lease expiry cannot discard the submitted source.
5. The integration coordinator requests a native merge/rebase candidate. Conflicts become structured path/content results for a repair task or an authorized decision flow. A model-generated resolution is a new revision requiring evidence. Disjoint paths are scheduling information, not semantic proof.
6. Qualification uses an independent environment at that exact candidate revision, toolchain and closure manifests. The native qualification adapter does not reconstruct Git merely to run checks. Invalid source IDs, stale evidence and incomplete required coverage are explicit refusals.
7. Acceptance checks the candidate, current integration-domain precondition and required evidence. It yields an immutable acceptance record. Hyperfide applies its dependency-release and task-completion policy from that result; acceptance alone does not close the task.
8. Remote publication follows an explicit export policy and its own authorization. Native acceptance is retained during a GitHub/GitLab outage. The product presents queued, ambiguous, diverged, failed and published export outcomes separately.

There is no transaction held open across an agent execution, object download, build or remote push. Durable requests and idempotent reconciliation connect those steps. The service does not claim exactly-once physical execution; it provides fenced authority and idempotent accepted outcomes.

## 4. Handoff, failure and disconnected behavior

Worker handoff revokes or supersedes the old attempt and workspace generations, then assigns a replacement from the latest acknowledged checkpoint. Pending unacknowledged local bytes may be recoverable from the original host, but must be labeled as unacknowledged and explicitly captured under current authority. The old worker may continue running physically; its later source mutations and result submissions are refused. Client-side abort is cleanup, not the security guarantee.

If the control plane becomes unreachable, already downloaded files may remain readable and local edits may continue within the host's declared operational policy. The client cannot acquire ownership, renew authority, create an acknowledged server revision or accept source while disconnected. Workspace detail reports the last acknowledged checkpoint and unavailable operations. On reconnection the host reconciles operation outcomes and generation validity before resuming mutation. The service never silently rebases dirty files onto a newer accepted snapshot.

Durable result custody must survive host cleanup, worker retries and compute lease expiry. Source-control retention receives roots from active workspaces, acknowledged checkpoints, submissions, candidates, acceptance records, export operations and release manifests; root removal uses the storage contract's transactional protocol. A task stop may stop further execution while retaining source and receipts needed for review. Explicit discard and retention expiry have different semantics from task cancellation.

## 5. Graph, context and qualification identity

A graph generation identifies repository, root tree/revision, optional immutable overlay revision, extractor version, schema version, resolver/build configuration and coverage manifest. Every context, search, reference and impact response returns those identities and freshness. Mutable local overlay observations receive a distinct overlay generation and cannot masquerade as a graph of the accepted source.

Extraction reuses immutable file facts keyed by content plus extraction configuration and language/build context. Path-dependent facts retain their path context; identical bytes in two projects do not imply identical resolved symbols. Resolution invalidates changed declarations, imports, generated inputs and affected consumers, including edges crossing project roots. A large interface change can legitimately require broad analysis. The cost contract avoids unconditional full-repository extraction for an unrelated local change; it does not promise that semantic impact is bounded by changed line count.

The native graph API exposes `status`, `symbols`, `context` and `dependencies` query modes, with bounded pagination and exact revision/tree bindings. Graph extraction reads selected source through the native manifest adapter and supports cancellation. A source snapshot with missing graph coverage stays readable as source while impact analysis reports its limits. Qualification policy may require broader checks; stale graph data must not silently narrow a required check set. Published generations remain immutable, and partial staging cannot replace a complete generation. Source authorization applies to snippets, search hits, caller/callee edges, caches and derived artifacts.

The native graph adapter preserves imported Git provenance separately from native revision and tree identities. Its build identity binds the parser, resolver, configuration and optional workspace overlay; coverage reports extracted and reused files, unresolved imports and affected paths.

## 6. CLI, SDK and UI journeys

The public CLI is `hyperfide platform source`. It reads `HF_SOURCE_CONFIG` or an explicit `--config` file. The [plugin guide](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) lists the supported verbs and flags. A shared client package supplies the C facade bridge, TypeScript SDK, host adapters and API automation with the same authentication, idempotency, pagination, cancellation and typed errors. Native read/query operations work with repository and revision identifiers and no current working directory inside a checkout. Git is invoked only by explicit compatibility adapters.

The app and CLI expose the following journeys:

| Journey | Required user-visible result |
|---|---|
| Open workspace | Select a configured project and source revision, declare read/write scope and mode, and inspect dependency closure and workspace generations |
| Save durable checkpoint | See upload progress followed by a durable revision receipt; local dirty state remains distinguishable |
| Submit and integrate | Inspect change summary, dependencies, conflicts, qualification status and accepted source revision |
| Resume or transfer | Inspect operations, transfer progress and the last durable checkpoint; retry retained requests by their original identities |
| Publish externally | Choose an allowed export destination and view independent native acceptance and remote publication state |
| Inspect source | Read immutable revisions, paginated history and diffs without opening a workspace |

Human-facing screens use existing page headers, forms, status components and navigation conventions. Primary controls use human-readable project, task and revision summaries. Raw hashes remain available through copy/detail controls, not as a mandatory manual input workflow. Permission failures and conflicts identify the affected operation and recovery action without exposing unauthorized paths. Large directory/diff/history views are paginated and do not transfer a complete repository manifest to render a page.

The app retains pending mutation identities and requests in browser session storage until a definitive response. Users can retry an uncertain request with its original identity or inspect the operation result.

## 7. Observability and budgets

The service metrics action samples objects, uploads, operations, publications, imports, graph builds and transfers. Each family reports coverage, observed rows, bounded state counts and available byte, age or duration measurements. Transfer queue depth, graph work and remote lag are reported for their respective families. The response identifies its observation time and sample budget; incomplete samples are marked `PARTIAL`.

The app and CLI record operation counts, durations and acknowledged content bytes. Acknowledged content bytes describe successful logical checkpoint content. Attached hosts supply cache, materialization, dirty journal and virtual filesystem counters. Metric labels use bounded mode, status and partition values.

The product request budget reserves two slots for cancellation and resume operations. Admission refusals carry the limiting budget and retry classification. Workload profiles live in [verification.md](verification.md).

## 8. Implementation locations

The service modules live under `tools/hyperfide/core/control-plane/src/source_control/`, the shared client and host implementation under `tools/hyperfide/sdk/source-control/src/`, and host integration under `tools/hyperfide/core/host-daemon/src/source-control/`. The source-control plugin connects those services to product journeys.

## Fleet prerequisite ownership

[Agent-led orchestration](fleet-orchestration.md) supplies the assignment and environment authority used by native source integration. Primary and Wave Managers choose wave dispatch, task assignment, provider placement and recovery through scoped tools; Hyperfide's engine validates and records those actions. Existing wording that Hyperfide owns scheduling refers to this complete system, not an engine-owned fixed planner.

The [sandbox environment contract](sandbox-contracts.md) survives native adoption. Every sandbox keeps `.neron` source configuration and `package.kvx`; `hyperfide install` calls the configured source adapter, pins exact inputs and returns a readiness receipt before workload launch. Native integration supplies snapshot/object reads and workspace/capture operations behind that contract. Change backend only through an explicit generation transition; retain legacy receipt provenance and requalify real install/worker paths. Source revisions and fleet environment package versions remain distinct.
