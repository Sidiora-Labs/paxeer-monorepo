# Fenced production worker dispatcher

This is a coordinator-operated local workflow tool, not an OS sandbox against malicious code running as the same user. Only the coordinator owns the private state directory, signing key, reviewed check registry and canonical task statuses. Workers receive an isolated workspace and the applicable task/lease identity. No command here edits canonical KVX or marks a canonical task done.

Concurrency is bounded by the configured admission ceiling, resource capacity and currently available worker slots. The coordinator must supply a fresh available-slot count from the actual agent tool, reduced by other active agents. Logical lanes separate ownership and resource namespaces; process overlap in local tests does not establish AI worker capacity or product throughput.

## Coordinator lifecycle

Run commands from the repository root. Set the shell variables `LAUNCHPAD_SPEC_FILE` and `LAUNCHPAD_PLAN_FILE` to a reviewed production-system specification and its matching execution-plan projection. Supply them through `--spec` and `--plan`. The dispatcher expects a two-part task-ID DAG with lane ownership and a matching plan.

Use the current specification and matching execution-plan projection. Graph layout identity excludes status; the execution-plan projection must still match the full current spec SHA and task states. After canonical task states change, the coordinator refreshes that projection before subsequent dispatcher commands. A worker must never do so.

Prepare coordinator-reviewed JSON capacity and per-worker resource files. Units are CPU cores, bytes for memory/disk, and integral PIDs/executor/build/container counts. All seven classes are required for capacity; a lease must reserve positive CPU/memory/PIDs/disk and exactly one executor. The coordinator supplies a fresh available-AI-slot observation from the actual agent tool, never the configured logical lane count. Admission also measures affinity/cgroup CPU, current memory/PID headroom and filesystem free space. These are resource reservations and point-in-time observations, not OS-enforced workload limits. The actual process harness records worker high-water RSS against its reserved envelope.

```bash
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state init \
  --spec "$LAUNCHPAD_SPEC_FILE" \
  --plan "$LAUNCHPAD_PLAN_FILE" \
  --source . --interface-digest "$FROZEN_INTERFACE_SHA256" \
  --capacity /tmp/kindle-wave1-capacity.json --available-ai-slots 20

python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state status
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state observe-ai --available-ai-slots 20
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state claim \
  --task 1.01 --owner worker-L01 --resources /tmp/kindle-worker-resources.json
```

`claim` returns the unique workspace and token file. Each attempt has four probed ports in its lane's reserved range, migration range, and distinct service/schema/subject/bucket namespace. A port probe is not an OS socket reservation: the service must bind it and reject failure. TTL defaults to 1,800 seconds; the coordinator heartbeats every 60 seconds, retaining the refreshed token. Expiry, coordinator restart, OS reboot or candidate rebinding invalidates older tokens. `restart` fences all unfinished leases; it preserves already accepted artifacts. Coordinate cleanup of the old owned processes before starting replacements.

```bash
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state heartbeat --token "$TOKEN_FILE"
```

After reviewing the worker's owned changes, the coordinator binds the candidate, builds the patch and registers actual tests. The registry is coordinator-owned outside worker workspaces. Registration does not qualify anything. Unsupported future task checks remain absent and fail closed.

```bash
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state candidate --token "$TOKEN_FILE"
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state patch --token "$TOKEN_FILE"
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state register-checks \
  --task 1.01 --checks /tmp/reviewed-L01-checks.json
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state qualify --token "$TOKEN_FILE"
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state accept --token "$TOKEN_FILE"
```

A reviewed check file is a nonempty JSON array. Every check specifies a unique `id`, command argv, positive `timeout` and a proof policy: `go-test-json`, `unittest` or `node-tap`. Use meaningful native tests for the task's actual acceptance criteria. The parser requires positive named passes and rejects skipped/empty/failed output. Input paths are included source files; optional output files must be absolute paths outside source. Frozen locks/toolchain and actual interpreter/checker bytes are separately bound. Example shape:

```json
[{"id":"dispatcher-source-policy","command":["python3","-m","unittest","discover","-s","tools/launchpad/production/dispatcher/tests","-t","tools/launchpad/production","-p","test_source_policy.py","-v"],"timeout":120,"proof":"unittest","inputs":["tools/launchpad/production/dispatcher/source_policy.py","tools/launchpad/production/dispatcher/tests/test_source_policy.py"]}]
```

The dispatcher runs check commands from the candidate root. The example names actual dispatcher tests; it does not qualify a product lane or register a production check. Select checks for the owning task's acceptance criteria. A real Go module example uses `go -C sdk/launchpad/shared test -count=1 -json ./secrets`, proof `go-test-json`, and these declared inputs (source paths, not Go import paths):

```json
["sdk/launchpad/shared/secrets/manager.go","sdk/launchpad/shared/secrets/manager_test.go"]
```

That module check requires its reviewed Go toolchain and dependencies; this example is not an executed qualification claim. Protocol coverage source lives at `sdk/launchpad/protocol/coverage`, including its `authorities.json` input. Only direct Go source files under `sdk/launchpad/shared/secrets` are source inputs; credential files and nested secret directories stay excluded. The old `protocol/coverage` and `shared/secrets` layouts are supported solely for isolated fixtures. Bytecode caches, dependency directories, build outputs and secrets follow the shared source policy and never enter signed source snapshots. Actual included source edits and file permission changes invalidate evidence.

Frozen contracts under `sdk/launchpad/shared/interfaces` and `sdk/launchpad/packages/production-contracts`, and production documents under `launchpad/spec`, are reserved. The only lane handoff exception is `launchpad/spec/production-system/handoffs/L01/**` through `L50/**`, subject to the lane's explicit trusted ownership. The reviewed leaf exceptions are `sdk/launchpad/packages/sdk/package.json`, `sdk/launchpad/packages/sdk/src/types.gen.ts` and `apps/launchpad/mobile/package.json`. Each still requires its owning lane; sibling generated files, nested manifests, lockfiles and foreign lane handoffs remain denied.

`accept` validates the live fence, actual test receipts, current candidate, exact trusted ownership and fresh patch. Files are flushed before an atomic SQLite acceptance index and lease retirement. Historical acceptance remains usable after the worker exits and its workspace disappears. Signed filesystem orphans without a committed index are rejected. An interrupted acceptance may leave an unindexed artifact requiring coordinator inspection; it cannot qualify. Accepted artifacts remain under the private state directory; do not move them or edit the index.

Acceptance frees resources, allowing waiting workers to run in another batch. The latest accepted fence for each task is authoritative; superseded artifacts cannot substitute for the selected attempt. Acceptance alone does not integrate code. Before integration, the coordinator runs the reviewed native test commands against the accepted candidate. For the dispatcher source tests, run from the repository root:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s tools/launchpad/production/dispatcher/tests \
  -t tools/launchpad/production -v
```

## Serial integration

Collect every accepted directory from the same baseline/interface/layout and selected wave. Combined checks also require explicit positive native proof policies. Full-wave staging rejects missing/duplicate/wrong-wave artifacts before publication.

```bash
python3 tools/launchpad/production/dispatcher/cli.py --state-dir /tmp/kindle-wave1-state stage \
  --wave 1 --accepted /tmp/kindle-wave1-state/accepted/ATTEMPT_01 \
  /tmp/kindle-wave1-state/accepted/ATTEMPT_02 \
  --output /tmp/kindle-wave1-staged --checks /tmp/reviewed-wave1-combined-checks.json
```

Supply every accepted directory; the abbreviated example needs the remaining directories to stage a complete wave. Staging reconstructs each authenticated candidate, checks the committed index, exact trusted registry/ownership and copied outputs, then applies sorted disjoint patches to a fresh tree. Combined native test commands run on that tree. Failure removes the stage and leaves the baseline/canonical repository unchanged. A successful complete-wave stage still reports `canonical_integrated:false`: the coordinator must review the combined test results and publish the staged changes. Partial staging is available only for explicitly initialized `--fixture` ledgers using `--fixture-partial`; it reports `qualified:false` and `complete_wave:false`.

## Actual local qualification

```bash
PYTHONDONTWRITEBYTECODE=1 python3 tools/launchpad/production/dispatcher/qualification.py \
  --spec "$LAUNCHPAD_SPEC_FILE" \
  --plan "$LAUNCHPAD_PLAN_FILE" \
  --state-dir /tmp/kindle-dispatch-proof-unique/state \
  --evidence-dir /tmp/kindle-dispatch-proof-unique/dispatcher --case all
```

Use a new state/evidence directory for each run. Eleven mandatory cases include real 50-child overlap and socket binds, actual heartbeats and control-operation timings, live-expired-child takeover, process races, OS/coordinator restart, timeout/SIGTERM descendants, path attacks, resource denial, authenticated proof rejection, all native module tests, and a real coordinator CLI acceptance/staging lifecycle. Isolated fixture statuses make wave 1 eligible solely for this test. No canonical task state or production transaction is changed.

`result.json` distinguishes standalone dispatcher-source binding from the root runner's full candidate binding. Root qualification also verifies source before/after, registry and evidence files. This safety proof is not validation of product features, real deployment, 500,000 DAU, or isolation against hostile same-UID code. Intentionally daemonized descendants escaping their process group need stronger OS containment; ordinary inherited descendants are tested and cleaned.

See the [bootstrap baseline](../bootstrap/README.md), [security evidence limits](../bootstrap/security/GAPS.md), [integration assurance](../integration/README.md) and [load workload](../load/README.md) for the separate product checks. These tools target one Paxeer network, chain ID 125; a committed block establishes finality. Local dispatcher fixtures do not exercise network consensus.
