# Source traffic measurement

The source measurement script records clone cost, integration wait, conflicts and optional GitHub rate-limit observations.

Run `tools/repository/hyperfide/measure-source-day.sh` from `tools/hyperfide`. It needs bash,
git (2.38 or later for conflict replay), awk, du and, for GitHub sampling, curl with
`GITHUB_TOKEN` or an authenticated `gh`. It writes one JSON report and never writes
provider env values into it.

## Method

Run it once, on a host with a current clone of the monorepo, at the end of a normal
working day:

```sh
tools/repository/hyperfide/measure-source-day.sh \
  --repo /path/to/monorepo --trunk origin/main --since "24 hours ago" \
  --provider railway --region <region> --provider-env /path/to/provider.env \
  --remote <the URL sandboxes clone from today> \
  --clone-mode full,partial,sparse --samples 4 --interval 7200 \
  --events /path/to/sandbox-events.jsonl \
  --github <owner>/<repo> --maintenance \
  --out source-day.json
```

It measures four things:

1. **Clone cost per sandbox.** Active: `--samples` rounds of clones from `--remote`,
   with the provider env exported, in each `--clone-mode`. `full` materializes a complete checkout. `partial` is `--filter=blob:none`. `sparse` adds `--sparse`, which is
   the lower bound a packet-scoped slice starts from. Each sample records wall
   seconds, bytes under `.git/objects`, bytes on disk and files checked out.
   `--interval` spreads the rounds over the day so they catch peak load. Passive: the
   `clone` events sandboxes reported.
2. **Snapshot-to-integrated wait.** For every first-parent merge on the trunk inside
   the window, every commit it brought in counts once, with wait = the merge's
   committer time minus the commit's committer time. A trunk that is rebased or
   squashed leaves no such trace. Those commits are counted under
   `non_merge_commits`, and their wait must come from `snapshot` events.
3. **Conflicts.** Each of those merges is replayed with
   `git merge-tree --write-tree` on its two parents. A textual conflict means that
   integration needed a human or a strategy option. Rebase conflicts that sandboxes
   hit before they merge are only visible as `rebase_conflict` events.
4. **GitHub and maintenance.** One `rate_limit` sample (core and GraphQL) at the time
   of the run. Push refusals and rate-limit hits come from events, because GitHub
   exposes neither per push outside the enterprise audit log. With `--maintenance`,
   the repository's scheduled maintenance tasks (commit-graph, loose-objects,
   incremental-repack) are run one by one and timed. This changes the repository's
   packs.

### Events file

Optional JSONL, one flat object per line, from the provisioner or sandbox logs:

| kind | fields read | meaning |
|---|---|---|
| `clone` | `seconds`, `bytes` | a sandbox materialised source |
| `snapshot` | `wait_seconds` | a snapshot waited this long to reach trunk |
| `rebase_conflict` | — | a rebase or merge stopped on a conflict |
| `push_refused` | — | GitHub refused a push (protected ref, non-fast-forward, size) |
| `rate_limited` | — | a GitHub call got 403/429 for rate limit |
| `maintenance` | `seconds` | a host ran git maintenance |

## Report fields

Every distribution is `{n, p50, p90, max, mean}`, with nearest-rank percentiles. A
field is `null` when its source was not given. That is different from a measured
zero.

| field | meaning |
|---|---|
| `schema` | `hyperfide.source-measurement/1` |
| `window.since`, `window.until` | the git date window measured |
| `repo.tracked_files`, `repo.count_objects` | size of the trunk tree and the object store |
| `provider.name`, `provider.region`, `provider.env_keys` | where samples ran. Env keys are names only |
| `clone.remote_is_github` | whether the sampled clone remote is GitHub |
| `clone.samples[]` | one row per sample clone |
| `clone.sampled.<mode>.seconds`, `.object_bytes` | sampled clone cost per mode |
| `clone.observed.seconds`, `.bytes` | clone cost sandboxes reported |
| `integration.first_parent_merges`, `.non_merge_commits` | trunk shape in the window |
| `integration.history_wait_seconds` | snapshot-to-integrated wait from history |
| `integration.observed_wait_seconds` | the same from `snapshot` events |
| `conflicts.merges_replayed`, `.textual_conflicts` | merges whose parents conflict |
| `conflicts.observed_rebase_conflicts` | conflicts sandboxes hit |
| `github.rate_limit` | `core` and `graphql` limit, used, remaining, reset |
| `github.observed_push_refusals`, `.observed_rate_limited` | from events |
| `maintenance.seconds`, `.tasks[]` | timed maintenance on `--repo` |
| `maintenance.observed_seconds` | maintenance hosts reported |
| `notes[]` | everything skipped or failed, and why |

### Interpreting the report

Compare `clone.sampled.full`, `.partial` and `.sparse` to measure the bytes and time needed for each checkout mode. Use `clone.observed.bytes` for actual worker materialization when events are available. `integration.*` and `conflicts.*` describe the recorded workload. A small sample describes those observations rather than a fleet capacity guarantee.

## Slices by default

The provisioner’s executor checks the workspace of every admitted worker job before claiming it.
Only preparation jobs, which build the boot seed, are exempt. The slot must hold a READY, unreleased slice materialisation for that run. If
the slot holds a `FULL_TREE` manifest, the derivation's `fallback_reason` is the recorded
reason. If no slice exists, the admitted request must declare
`workspace: {mode: "FULL_TREE", fallback_reason: "<CODE>"}`. Anything else is refused
with `WORKER_SLICE_REQUIRED`, `WORKER_SLICE_MISMATCH`, `WORKER_SLICE_AMBIGUOUS` or
`WORKER_WORKSPACE_INVALID`, and the admission settles as UNKNOWN.

Verification kinds (`BUILD`, `TEST`, `REVIEW`, `INDEX`, `INTEGRATION`) use the same workspace checks. The service's verification producers (documentation closure, assurance checks,
integration candidates and review reproductions) declare
`{mode: "FULL_TREE", fallback_reason: "INDEPENDENT_VERIFICATION"}`, so their full tree is
a recorded reason rather than a silent default. `runner.executor.describe` reports
`workspaces.slice`, `.full_tree` and `.refused`.

The other launch paths carry the same workspace identity:

- Admission (`remote.prepare`) writes `request.workspace` into the immutable packet
  from the slot's current materialisation: `{mode: "SLICE", slice_digest}`, or the
  `FULL_TREE` manifest's reason. An empty slot leaves the request as submitted.
- The executor starts the process in the slot's slice workspace,
  `$HF_PREPARATION_WORKSPACE_ROOT/<sandbox>/<slot>/<boot generation>/workspace`, and claim
  re-checks that the slot still holds that materialisation. A materialised slice with
  no configured root is refused with `WORKER_WORKSPACE_ROOT_UNSET`.
- An agent launch (`runner.dispatch.consume`) is refused before its plan is read unless
  the slot holds a slice or the execution packet records a full-tree reason (a
  `FULL_TREE` manifest derived for that packet).
- Slot occupancy is checked within the run, and a slice's metadata roots include every
  ancestor `.gitignore`. A sealed partial clone cannot fetch a missing ancestor
  `.gitignore`, and without one `git status` fails.

Local fleet workers follow the same default. `cg fleet begin` cuts a sparse worktree
from the task's slice unless `[hierarchy] sparse = "full"` opts out, or the worker spans a
whole wave. A worker that needs a file outside its slice runs
`cg slice <id> --widen PATH... --reason CODE`. That records the path in the task's
`touches` and `widened` fields in its packet and emits a `fleet.widen` event.

### Worker metrics

Compare full-tree and slice measurements using the same repository and workload. `workspaces.full_tree` counts the jobs using a declared full-tree reason.

Per worker, the numbers come from `preparation.slice.inspect`, under `summary`:

| field | meaning |
|---|---|
| `worktree_bytes`, `disk_bytes` | the latest READY materialisation's evidence metrics |
| `derivation_misses` | out-of-slice reads refused while the job ran: inputs the derivation missed |
| `out_of_slice_writes` | refused writes outside the slice; these are never widened |
| `widen_events`, `widened_children` | explicit widenings and the child manifests they produced |

### Widen rules

`planWidening` turns refused findings into widen requests, grouped by reason:

- a read under a `fixtures/`, `testdata/`, `__fixtures__/` or `__snapshots__/`
  directory widens the whole directory (`FIXTURE_DIRECTORY_READ`);
- a build root, or a path the command declared as required, widens alone
  (`BUILD_ROOT_EXCLUDED`);
- any other read widens the file (`OUT_OF_SLICE_READ`);
- a read the access monitor could only name as a Git object cannot be widened. It
  forces the full tree with `UNMAPPED_OBJECT_READ`;
- more than 256 paths in one plan means the closure was wrong. It falls back with
  `WIDEN_LIMIT_EXCEEDED` instead of widening;
- writes are never widened.

The rules are implemented in the [slice widening planner](../../../tools/hyperfide/core/control-plane/src/preparation/slices.ts). Measure worktree bytes, derivation misses and widening events against the project's published graph when selecting its workspace policy.
