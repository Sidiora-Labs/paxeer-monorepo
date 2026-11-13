# Native source control: verification and qualification

## 1. Qualification interfaces

The [qualification runner](../../../tools/hyperfide/tests/hyperfide/qualify.py) executes registered subprocesses, captures source and environment inputs and invokes the [independent evidence checker](../../../tools/hyperfide/tests/hyperfide/check-evidence.py). The [manifest schema](../../../tools/hyperfide/tests/hyperfide/manifest.schema.json) supports native-source evidence and registry schema v2 accepts `V-NNN` and `SCM-NNN` case identities.

Feature registrations live under [tests/hyperfide/source-control](../../../tools/hyperfide/tests/hyperfide/source-control/). Executable drivers live under [cases/source_control](../../../tools/hyperfide/tests/hyperfide/cases/source_control/). From `tools/hyperfide/`, the runner accepts:

```sh
python3 tests/hyperfide/qualify.py --registry tests/hyperfide/source-control/registry.json --case SCM-020 --manifest <immutable-campaign-manifest.json>
```

The manifest pins the actual campaign inputs. Component, full-case and campaign evidence have separate scopes. Native revision and tree identities remain distinct from Git commits. Backend and provider profiles are explicit; a local component result does not establish live backend or fleet capacity.

## 2. Evidence contract

Every attempt creates an append-only record with:

- Case and assertion IDs; requirement clauses; task, full-case, or campaign scope; feature/spec revision; implementation and runner revision; schema and protocol versions.
- Native source revision and root as `sha256:<64hex>`, scope manifest digest, dependency closure, accepted-source record where applicable, and exact merge/rebase candidate inputs. Stable change identity alone is insufficient.
- Actual backend identity and capabilities; database deployment/epoch and durability policy; object namespace/region and encryption configuration; client/server/host versions; hardware, network placement, cache budgets and toolchain digests.
- Fixture corpus identities, seed, operation distribution, intended arrival schedule, offered and completed load, concurrency time series, fault schedule, and expected assertion inventory.
- UTC start/end, monotonic durations, invocation and exit status; complete redacted logs and raw measurement artifacts with content hashes; server traces and independent observations.
- Acknowledged revision/object sets before each injected failure, authoritative acceptance history, remote Git reference observations where relevant, and inventory of temporary/retained resources.
- Observed assertions, diagnostics, coverage gaps, cleanup outcomes, and checker identity/result. Private credentials are references only; exported results contain no source beyond the selected publication scope.

An implementation handler's response is one observation, not the sole correctness oracle. The checker reconstructs reachable trees from immutable objects, compares histories to a recorded client operation schedule, verifies acceptance preconditions and source evidence, and inspects independently retrieved remote state. Generated test programs use actual source-control APIs and persistent backends. A replay workload can reproduce failures but cannot prove physical fleet density, native-agent activity, S3 account capacity, or real elapsed-time endurance.

An acknowledged checkpoint must be retrievable after every fault within the configured durability contract. The campaign records its actual replication and failure domain. Zero missing acknowledgments in a tested schedule is reported as that finite result; it is not a claim of survival under arbitrary regional destruction or a backup whose stated recovery point precedes the acknowledgment.

Case state starts UNRUN. A real attempt yields PASS, FAIL, or BLOCKED; historical records remain immutable. A component PASS never automatically marks the parent case, requirement, integrated revision, or release PASS. Changing relevant source, storage policy, authorization rules, dependency manifests, environment or toolchain invalidates affected evidence reuse. Reuse of unaffected receipts requires an explicit comparison of their inputs.

## 3. Executable workload profiles

The [campaign driver](../../../tools/hyperfide/tests/hyperfide/source-control/workloads/campaign.mjs) validates explicit corpus sizes, changed-file and byte counts, clients, operations, arrival rate, duration, hardware digest and backend identity. Its campaign kinds are `component`, `growth`, `mixed`, `admission` and `endurance`.

| Kind | Admission requirements |
|---|---|
| `component` | Positive bounded workload inputs and explicit corpus sizes |
| `growth` | File counts 10,000, 1,000,000 and 10,000,000; ten modified files and 1,048,576 modified bytes |
| `mixed` | At least 5,000 clients, 100 submissions per second, 300 seconds and 30,000 operations |
| `admission` | At least 20,000,000 actual workspace-open operations |
| `endurance` | At least 259,200 seconds and 300,000 operations, a real backend, a worker canary and throttle, credential-rotation, collection and recovery hooks |

Every non-component kind requires `real_backend = true`. These are input requirements for the selected workload, not measured service capacity. [Campaign manifests](../../../tools/hyperfide/tests/hyperfide/source-control/manifests/campaigns/) provide the selected workload inputs. The registry also contains workload summaries; the selected executable registration and manifest define the run.

The [deployment driver](../../../tools/hyperfide/tests/hyperfide/source-control/workloads/deployment.mjs) reads pinned corpus revisions and measures requests against a configured source authority. Its [workload documentation](../../../tools/hyperfide/tests/hyperfide/source-control/workloads/README.md) describes deployment inputs, renewable credential transports, observers and recovery hooks. Logical opens, hydrated bytes, newly authored revisions and worker revisions have separate accounting.

## 4. Measurement results

Campaign results include offered and completed work, latency distributions and resource observations. Deployment inputs supply `max_checkpoint_p95_ms` and `max_growth_ratio`. Growth compares the same ten-file, one-MiB change across the selected corpora. Hardware and deployment identity accompany the measurements.

The campaign driver validates latency samples and records p50, p95, p99 and maximum values. Missing backend capabilities produce a typed blocked outcome. The [durable ledger](../../../tools/hyperfide/tests/hyperfide/source-control/workloads/durable.mjs) retains operation and revision identities for resumed runs. Workload results remain bound to their source, backend and configuration inputs.

## 5. Stable product case registry

The case identity covers the complete observable outcome; component tasks may implement registered subsets without qualifying the entire case. The [registry](../../../tools/hyperfide/tests/hyperfide/source-control/registry.json) records each case's assertion inventory, requirement clauses, tasks and oracle.

| Case | Scenario and fault schedule | Required independent oracle |
|---|---|---|
| SCM-001 | Encode/decode trees with large directories, Unicode names, binary content, executable modes and symlinks; submit malformed/noncanonical objects and mismatched typed digests. | Canonical bytes reconstruct the intended tree and identity; invalid/path-escaping inputs are rejected; fixed small changes only create affected tree nodes. |
| SCM-002 | Upload objects and finalize revisions; interrupt multipart upload, lose S3/metadata replies, corrupt content, and crash at every upload/registration/finalization boundary. | Every acknowledged revision references verified readable objects; retries reconcile the same result; incomplete uploads and unreferenced objects remain explicit and cannot become published source. |
| SCM-003 | Retry identical requests after restart, reuse a key with different content, submit concurrent revisions under one stable change identity, and cancel a resumable operation. | Durable idempotency returns identical outcomes for identical requests, rejects conflicting reuse, preserves explicit divergence, and records a recoverable terminal operation state. |
| SCM-004 | Reassign an attempt, expire its workspace lease and replay uploads, commits, acceptance and cached authority from the old worker. Disconnect and reconnect clients. | Current server-side scope/generation checks fence unauthorized effects; old source stays readable under policy; no offline client can renew authority or make a local save appear durably committed. |
| SCM-005 | Race GC mark/sweep with uploads, checkpoint pinning, workspace renewal, result custody, restore pins, export and resurrection of previously unreachable content. | Acknowledged and pinned reachable objects remain retrievable; quarantined deletions obey epochs/grace; sweep cannot delete newly rooted content; expired unreferenced uploads reclaim without repository-wide stop. |
| SCM-006 | API-only clients execute ancestry, merge-base, history, diff, blame, bisect and revert queries; paginate through inserts and request missing/stale index versions. | Results match immutable snapshot fixtures; pagination has no missing/duplicate entries for its pinned view; unavailable coverage is explicit; no client checkout or full local history is required. |
| SCM-007 | Delay, duplicate and reorder committed events; commit an older allocated sequence after a newer one; reconnect after retention expiry. | Each committed effect is eventually observed once at consumer state level; aborted events remain invisible; durable resume cannot skip late commits; retention gaps trigger explicit resynchronization. |
| SCM-008 | Merge/rebase/revert disjoint edits, overlapping edits, renames, mode/symlink changes and cross-project dependencies; interrupt and retry server workers. | Result trees match an independent three-way reference corpus or return structured conflicts. Original revisions remain immutable. Path disjointness alone never certifies semantic compatibility. |
| SCM-009 | Prepare competing candidates, change the accepted base during qualification, replay stale receipts, and test individually green changes that fail together. | Atomic acceptance admits only current authorized candidates with evidence bound to their exact source/input set; stale or incompatible candidates do not advance accepted state. |
| SCM-010 | Materialize concurrent sparse scopes with warm/cold caches; omit required inputs, delete a visible file, and leave an unrelated path outside the slice. | Included files match the pinned snapshot with correct modes/types; sparse absence creates no deletion; omitted dependencies produce explicit widening/refusal; immutable cache data cannot be modified through writable workspaces. |
| SCM-011 | Virtual reads, directory pagination, rename/delete notifications, editor atomic saves, watcher overflow, cache eviction and server loss occur while files are dirty. | Reads retain snapshot identity; change reconciliation finds all modifications or explicitly rescans affected scope; dirty data is never evicted as cache; unavailable reads fail visibly without fabricating content. |
| SCM-012 | Incrementally index changed projects and shared consumers, query a stale generation and interrupt graph publication. | Responses identify source/extractor/configuration and coverage; changed contracts invalidate relevant consumers; incomplete generations do not masquerade as complete; unaffected indexed objects are reused. |
| SCM-013 | Unauthorized clients probe blobs, ranges, cached objects, search, blame, history, cross-project manifests and guessed identical-content digests. Revoke a capability during a job. | All routes enforce selected source scope, revocation and epoch; deduplication does not become an object-existence oracle; public export contains only explicit allowed schema/content. |
| SCM-014 | Hyperfide assigns a real task to a native workspace, checkpoints edits, replaces its worker, transfers result custody, qualifies and integrates. Also exercise a taskless authorized client. | Task/attempt/source lineage and fencing are preserved; compute lease expiry cannot destroy submitted custody; task completion and source acceptance remain distinct; taskless operations have valid independent authorization. |
| SCM-015 | Export project scopes to a disposable GitHub destination (GitLab is optional and separately qualified when used); drop push responses, advance remote refs externally, revoke permission, and restart exporter mid-stream. | Independently fetched Git objects match the selected native tree, deterministic mapping and parentage; expected-head divergence is explicit; ambiguous pushes reconcile; native acceptance remains distinct from remote publication. |
| SCM-016 | Ingest external contributions and histories with duplicate identities, merge parents, rename/mode changes and project-prefix mappings; replay the import. | Imported proposals and provenance reconstruct the intended trees and ancestry; idempotent replay adds no duplicate source history; imported history counts remain separate from newly authored revisions. |
| SCM-017 | Consolidate several repositories into one monorepo, migrate live workspaces, interrupt cutover, attempt legacy writes and request rollback after new native writes. | Original-to-native identities reconcile; exactly one writable authority remains; pending work and export mappings retain custody; rollback preserves/reconciles post-cutover revisions rather than reopening stale history. |
| SCM-018 | Kill API/merge/acceptance workers and fail over metadata under concurrent acknowledged writes; interrupt S3 access and deliver stale leader requests. | Durable operation history satisfies the declared fault model and acceptance order; all acknowledged checkpoints survive supported failures; obsolete authority is fenced; uncertain outcomes reconcile before retry. |
| SCM-019 | Restore metadata, object references and retained source from a selected backup while old workers and GC exist; deliberately omit an object and replay pending exports. | Restore verifies all required reachable objects, reports loss against its declared recovery point, uses a fresh authority epoch and reconciles external effects before writes reopen; a partial restore cannot claim service recovery. |
| SCM-020 | Repeat fixed changes while file count, hot-directory fan-out and ancestry grow; inspect reads, writes, memory and storage partitions before/after adding capacity. | Growth budgets and relevant-scope work bounds hold; no service creates or requires a full repository materialization; within-repository sharding and independently scaling tiers are measured. |
| SCM-021 | Run declared mixed traffic with at least 5,000 active workspaces, sustained revision target, burst submissions and a separate twenty-million-open admission profile. Saturate one tier. | Correctness invariants hold with measured offered/completed load, latency distributions, bounded queues/cache/connections and explicit overload; logical opens and physical hydration are reported separately. |
| SCM-022 | Run real S3-backed steady traffic for 72 elapsed hours, with object reads/uploads, GC, credential rotation, throttling and recovery drills. Execute a separately enumerated real Hyperfide worker canary. | At least 300,000 new revisions meet the daily target and declared durability checks; actual S3 requests/bytes and resource inventory reconcile; synthetic clients, actual workers and imported revisions are counted separately. |
| SCM-023 | Use CLI/API and native Hyperfide surfaces to create a scoped workspace, edit, inspect diff, checkpoint, submit, resolve a real conflict, qualify, accept and observe remote publication; include failure/retry and restart. | A second client reconstructs the same source/status without local history; operator-visible states match durable records; the real user journey succeeds on the release candidate with no fixture-only substitutions. |
| SCM-024 | Exercise harness false-success traps, tamper evidence, omit a required variant, then assemble closure from all cases, cleanup and current source. | Missing/zero/tampered assertions and stale or mismatched evidence fail nonzero; an independent checker enumerates every required clause and variant; only a complete current pack can establish release qualification. |

SCM-015 needs a real qualification variant for each required remote provider; a local Git receiver proves only protocol behavior. SCM-022 requires real object-storage requests and actual elapsed time; accelerated replay is a distinct variant. SCM-014 and SCM-023 require real Hyperfide assignment and qualification rather than a sequence of direct SCM API calls labeled orchestration. A missing live credential blocks that variant without invalidating any separately established component result.

## 6. Evidence closure

The [closure checker](../../../tools/hyperfide/tests/hyperfide/source-control/closure.py) reads registered case and assertion inventories and their evidence. Component, full-case and campaign results retain their own scopes and pinned inputs. The [full-case registrations](../../../tools/hyperfide/tests/hyperfide/source-control/full_case/) define the executable full-case inputs.

## 7. Repository-layout coverage

SCM-025 covers category layout and package preservation. [Repository layout](repository-layout.md) describes the package boundaries; the [mapping](repository-layout.json) defines source-to-category identities. Layout drivers use the product's build and runtime interfaces independently of S3 source storage.

## 8. Fleet and sandbox coverage

The fleet and sandbox cases bind source mode, environment installation, planning authority and recovery to their actual execution inputs.

| Case | Required exercise | Independent oracle |
|---|---|---|
| SCM-026 | Parse and resolve `.neron`, `package.kvx` and selected `interfaces.kvx` blocks; reject ambiguous versions, malformed paths, stale source bindings, missing consumers and forged catalogue inclusion. | Exact source/manifest/interface identities and actual parser/consumer outcomes; bounded subset lookup without complete catalogue transfer. |
| SCM-027 | Fresh and repeated provider sandbox installation, interrupted preparation, missing dependencies/configs and multiple isolated workers. | Actual source/dependency/config bytes, toolchain and readiness checks match the receipt; incomplete installs never launch workloads and dirty overlays survive retry. |
| SCM-028 | Actual Primary and two Wave Managers on the same box delegate waves, choose provider sandboxes and revise a plan after changed observations. | Persisted agent-selected actions and observed provider/worker effects agree; policy violations are rejected without a hidden deterministic planner. |
| SCM-029 | Manager and whole supervisory-box interruption, stale-owner commands, unknown provider outcomes, model unavailability and final receipt closure. | Current ownership is unique, persisted effects reconcile, active resource accounting is complete, and no missing required assertion is labeled successful. |

Scripted planners or parser-only checks do not qualify agent-led fleet execution. Record actual model/provider profiles, host/process/sandbox boot identities, environment generations, exact contract versions, action receipts and runtime outcomes. Do not record private model reasoning as required evidence. Source modes retain their own installation and worker evidence identities.
