The implementation run uses one orchestrator and seventeen feature managers on the control plane, with at most 500 ephemeral task sandboxes. Each admitted task has one owner, one live sandbox, and the image selected by its `sandbox_profile`. Managers remain on the control plane; they do not consume task slots. Replacement sandboxes consume the same global capacity permits as first attempts.

This is an execution plan, not evidence that implementation or qualification has run. Its baseline is repository revision `2a1033aa2` and the merged specification. All acceptance qualifications begin as `UNRUN`. The Hyperfide CLI (`cg`) supplies the task commands; the orchestrator supplies the global wave barrier, publication latch, capacity accounting, and durable attempt records. Examples below describe the future implementation run. They do not authorize changes during specification preparation.

**Control plane and ownership.**

There is one authoritative task board for the feature `layerx-merge`, containing all seventeen domains. Domain numbers are not seventeen separate CLI feature names. Every command passes `-f layerx-merge` and an explicit `--root`. One control-plane dispatcher serializes board mutations, including claims, renewals, releases, integration of task status, and reconciliation repairs. Readiness snapshots may be shared with all managers, but a snapshot never reserves work.

The dispatcher maintains a durable record keyed by task id and attempt: source revision, specification digest, owner role, opaque worker identity, claim attempt id, fencing value, lease expiry, task deadline, repair count, sandbox allocation, publication receipt, and evidence references. The dispatcher accepts an implementation, status transition, or publication only from the current attempt and fence. `heartbeat` exposes attempt and fence flags; the CLI help does not advertise these flags for `done`, `implemented`, or `release`, so the dispatcher checks their enclosing request instead of inventing CLI options.

| Domain | Feature manager | Exclusive scope |
|---|---|---|
| 1 | `fm-kernel` | Native kernel, host shim and proof packages |
| 2 | `fm-ledger-auth-pay` | Ledger, authorization, payment and agent modules |
| 3 | `fm-markets` | Markets, pricing, oracle and related data service |
| 4 | `fm-launchpad` | Launchpad chain surfaces, services, clients and tools |
| 5 | `fm-node-runtime` | Node, sequencer, storage, runtime and root Go dependencies |
| 6 | `fm-chain-modules` | Other chain modules, shared precompile setup and contracts |
| 7 | `fm-rpc-api` | RPC, API registry and root proto generation configuration |
| 8 | `fm-platform` | Platform services, tools, middleware and SDKs |
| 9 | `fm-agent` | Agent services and their SDKs |
| 10 | `fm-human` | Human services and KMS |
| 11 | `fm-interop` | Interoperation services and mirror components |
| 12 | `fm-wallet` | Wallet services, application and SDK |
| 13 | `fm-programs` | Programs, program runtime and program precompile |
| 14 | `fm-web-explorer-bridge` | Web, explorer, bridge and JavaScript SDK |
| 15 | `fm-cosmos-fork` | Assigned SDK fork packages |
| 16 | `fm-tools-ops` | Shared tools, deployment, automation and test infrastructure |
| 17 | `fm-programme` | Specification, documentation, decisions and programme evidence |

The scope column summarizes the ownership table in the contributor contract; the exact task `touches` list and recorded ownership rulings govern admission. A manager cannot expand a task into a neighboring domain. The existing wiring resolution and its fulfilling owner task supply cross-domain changes. Shared files such as `node/app.go`, `precompiles/setup.go`, `go.mod`, and `Makefile` retain their assigned owner. No manager dispatches an independent editor for a shared file outside that owner's task.

The orchestrator owns the global frontier, worker capacity, the immutable base of each wave, integration order, publication acknowledgements, aggregate gate scheduling, and recovery from lost attempts. It prevents duplicate task dispatch and duplicate gate execution. It reads qualified task evidence without running the task's tests again. The domain manager owns the task packet, source and assertion review, fixture readiness, profile selection, exact test-to-criterion mapping, scope review, and acceptance of the current attempt's output. The task owner implements and qualifies the whole declared task, then stops after publication or a recorded incomplete outcome.

**Admission and the 500-sandbox ceiling.**

The merged baseline contains 2,517 tasks. Counts are a planning snapshot; the dispatcher recomputes them from the installed specification after any approved graph change.

| Wave | Meaning | Baseline task count |
|---|---|---:|
| 0 | Foundations and harnesses | 259 |
| 1 | Core logic | 440 |
| 2 | Callable surfaces | 431 |
| 3 | Services | 205 |
| 4 | Integration | 314 |
| 5 | Clients | 407 |
| 6 | Conformance and retirement | 430 |
| 7 | Completion and programme certification | 31 |

For the open wave, admit at most `min(500, eligible conflict-free tasks, image capacity, resource capacity)`. The largest baseline wave has 440 tasks; 500 is a ceiling, not a utilization target. Same-wave dependencies and scarce fixtures reduce the concurrent frontier further. Use fair rotation among managers, admitting one eligible request per nonempty domain before a second round. Prioritize prerequisites that unblock the frontier within that rotation. Never hold a task claim while waiting indefinitely for image or fixture provisioning: prepare capacity first, then claim and start the task deadline.

`cg spec ready` lists eligible tasks across waves. In parallel mode the CLI also treats an implemented prerequisite as sufficient for implementation admission. The dispatcher therefore filters the CLI result through stricter rules: only the currently open wave; every prerequisite qualified, published and integrated; every requested anchor satisfied; every fulfilling wiring present; no overlapping file writer; and sufficient capacity. An implemented-only task never unlocks a successor in this plan. Empty readiness with outstanding tasks triggers dependency or fixture escalation, not forced starts or deletion of dependency edges.

All task sandboxes, replacements, and temporary aggregate gate allocations count toward the 500 limit. Fixture processes run within their task allocation; any fixture that needs a separate execution sandbox also consumes a permit. Aggregate gates begin after task workers for the wave have drained. Budget CPU, memory, disk, dependency downloads and fixture capacity separately from the sandbox count. Rate limits and image shortages reduce admission; they never change acceptance criteria or reset a task's repair budget.

**Images and toolchain versions.**

Each profile resolves to an immutable image digest and a recorded toolchain manifest before admission. The manifest includes architecture, operating-system base, exact compiler and runtime patch versions, package-manager versions, dependency lock digests, and installed native libraries. A repository version range is a compatibility requirement, not an exact patch pin: resolve its patch once when preparing the image and record the result. Do not resolve an unpinned latest version during a task. Image preparation fetches dependencies and tools; it does not run repository builds or tests as a substitute for task qualification.

The following CPU, memory and scratch figures are initial reservations for this plan, not measured performance guarantees. A manager can request a larger allocation before claim if the declared build or fixture needs it. The 30-minute task limit remains unchanged.

| `sandbox_profile` | Toolchain and workload | Initial reservation: vCPU / GiB memory / GiB scratch |
|---|---|---|
| `go` | Go 1.25.6 from `go.mod:3`, shell, Git, Python 3.12 and proto tools when declared. Preserve each nested module's dependency graph; launchpad modules declare Go 1.25.0, for example `services/launchpad/api/go.mod:6`. This profile is for commands that need no native host link. | 4 / 8 / 30 |
| `go-cgo` | Go 1.25.6 with cgo enabled; C compiler 13.3.0 from `tools/release/toolchain.env:1`; Rust 1.91.1 from `rust-toolchain.toml:2`; C17 tools, OpenSSL development files and the declared native link dependencies. Also runs native C tasks, because the profile enumeration has no separate C profile. | 8 / 16 / 60 |
| `rust` | Rust 1.91.1, minimal profile with `clippy` and `rustfmt` as declared at `rust-toolchain.toml:2-4`; `wasm32-unknown-unknown` where required, matching `.github/workflows/ci.yml:51`; native compiler and crypto development files. Use the task's manifest, package and locked dependencies. | 8 / 16 / 60 |
| `node` | An image-pinned Node 24 patch at least 24.3.0, satisfying `apps/web/package.json:7`, `apps/explorer/package.json:7`, `services/agent/sdk/typescript/package.json:32`, and the mobile range at `apps/launchpad/mobile/package.json:40`. Use each package's declared manager and lock; `apps/bridge/package.json:185` pins pnpm 12.15.0. Include the task's declared browser or local-chain binary only when needed. | 4 / 8 / 30 |
| `elixir` | Elixir 1.19.4-otp-27, Erlang 27.3.4.6 and Node 20.17.0 from `services/explorer/backend/.tool-versions:1-3`; Mix and locked Hex dependencies. `services/explorer/backend/mix.exs:14` requires Elixir `~> 1.19`. Keep this Node selection local to the backend image. | 4 / 8 / 40 |
| `solidity` | Foundry v1.7.1 from `.github/workflows/ci.yml:68`, with solc 0.8.27 for `foundry.toml:6` and `foundry.paxeer.toml:10`, and solc 0.8.30 for `contracts/bridge/evm/foundry.toml:8` and `services/interop/contracts/ethereum-mirror/foundry.toml:6`. Select the compiler by the declared task root and configuration. | 4 / 8 / 30 |
| `python` | An image-pinned Python 3.12 patch, matching `.github/workflows/platform.yml:890`; pytest or unittest as declared, shell and task-specific requirements. This also satisfies the `>=3.11` requirement at `sdk/integrations/fastapi/pyproject.toml:10`. | 2 / 4 / 15 |
| `jvm` | An image-pinned JDK 21 patch and Maven, with Kotlin 2.0.20 where used; `sdk/platform/jvm/pom.xml:23-25` fixes the release level and Kotlin version. Add Gradle and Android tooling only for a task that declares them. | 4 / 8 / 30 |
| `dotnet` | An image-pinned .NET SDK 8.0 patch, matching `.github/workflows/platform.yml:1411`, and the `net8.0` target at `sdk/platform/dotnet/LayerX.Sdk.csproj:3`. Preserve warnings-as-errors at `sdk/platform/dotnet/LayerX.Sdk.csproj:6`. | 4 / 8 / 25 |
| `swift` | Swift 6.0.3 from `.github/workflows/platform.yml:55`; SwiftPM packages retain their 5.9 tools declaration at `sdk/platform/swift/Package.swift:1` and their locked dependencies. Linux allocations run declared portable package tests. A declared native UI gate requires a compatible execution environment before admission. | 4 / 8 / 30 |
| `mixed` | A task-specific immutable union of the profiles needed by its `build_cmd`, `verify_cmd` and fixtures, including Python 3.12 for the programme runners. Record every component version. Do not provision an unspecified universal image or install another compiler during qualification. | 8 / 16 / 80 |

The baseline profile counts are: `go` 501, `go-cgo` 744, `rust` 523, `node` 336, `elixir` 31, `solidity` 6, `python` 251, `jvm` 15, `dotnet` 8, `swift` 10, and `mixed` 92. A domain's default profile does not override the profile on an individual task.

Nested package constraints require explicit image variants. The explorer backend declares Node 20.17.0 in its tool versions, while `services/explorer/backend/apps/explorer/package.json:10-12` requires Node 18.x and npm 8.x for that package. A task that invokes those assets needs a pinned Node 18/npm 8 executable path for that invocation alongside the backend's default tools. Do not run those commands under the generic Node 24 path or change their engine declarations to obtain a pass. Likewise, preserve the Foundry version selected by a task-specific CI configuration when it differs from the root CI pin.

The native host link at `modules/lxphost/host.go:6-7` requires generated headers, `liblayerx`, `liblayerx_programs_sandbox`, OpenSSL and platform link libraries. Dependency archives may be restored only with an exact source, compiler, flags and architecture cache key. If the task changes a native input, its single declared build must rebuild that input and the dependent target. An old cached archive cannot qualify changed source. If the declared build cannot produce its required libraries within scope and budget, record the task declaration defect and hold the task; do not add an undeclared build campaign or silently disable cgo.

Proto-capable images contain Buf 1.58.0, as invoked by `scripts/protoc.sh:26-28`, and the generator versions and local outputs declared by `buf.gen.yaml`. Provide access to the approved module proxy or a complete verified cache before claim. Generator output is still limited to the task's declared files. The proto helper's `go get` at `scripts/protoc.sh:19` does not grant a non-owner task permission to modify root Go dependency files. Record missing tool or dependency access as a provisioning failure rather than replacing generation with a source scan.

**Fixtures and the launchpad network allocation.**

Eleven launchpad tasks explicitly need a single-validator network: `4.5.2` through `4.5.9` in wave 2 and `4.10.3` through `4.10.5` in wave 5. Their planned tests use `tests/testutil/network.New`, defined at `tests/testutil/network/network.go:45`; its default configuration is the single-validator configuration at `tests/testutil/network/network.go:60-64`. These are task-scoped real-network behavior tests, distinct from the aggregate cluster matrices.

Reserve 8 vCPU, 32 GiB memory and 80 GiB scratch per such task, including its compiler processes, validator, RPC endpoints and database files. At full frontier this is eight network allocations in wave 2, totaling 64 vCPU, 256 GiB memory and 640 GiB scratch; wave 5 needs three, totaling 24 vCPU, 96 GiB and 240 GiB. If that capacity is unavailable, queue some of these eleven tasks. Give every allocation private process and network namespaces, a separate genesis and data directory, and ports allocated within that namespace. Never share validator state across tasks or attach to an existing operational chain. Provisioning checks tool availability and fixture inputs; the actual chain startup and assertions execute inside the declared test run.

Rust task `10.11.2` needs the real attestor test cluster with a disposable secp256k1 signing key and the configuration needed to exercise its missing-key refusal. Tasks `10.13.3`, `10.13.4` and `10.13.5` need an isolated peer socket and the real peer implementation or the task's declared protocol fixture. Fixture binaries are built from the pinned candidate or a declared dependency revision. They must exercise real decoding, pending nonces, rung bytes and transaction signatures, never a fake implementation of the unit being tested. Give each attempt separate sockets, temporary state and fixture lifecycle; revoke test key access and destroy fixture state at teardown. Store key material and raw runtime inventories outside public artifacts.

Database-backed tasks receive isolated databases, the exact migrations they declare, and a readiness result before claim. The missing launchpad SQL and API bundles are recorded external gaps under task `17.13.1`; a consumer that needs an absent artifact remains unqualified. No fixture replaces a missing authority document or schema with invented data, and no task's zero-test or skipped-test exit is accepted as evidence.

**CLI lifecycle and command ownership.**

The command names and flags below are from the installed CLI's `spec --help` and individual subcommand help. Claim TTL values are minutes; the default is 30. This plan explicitly uses a five-minute renewable lease and a separate 30-minute task deadline. The prepared execution workspace uses parallel mode. The existing workflow describes phase separation at `spec/workflow.kvx:24-28`, qualification at `spec/workflow.kvx:48-53`, and the parallel setting at `spec/workflow.kvx:123`.

There are two kinds of roots. `PLAN_ROOT` is the single authoritative scheduling root on the control plane. `TASK_ROOT` is an isolated checkout in a task sandbox, with the task packet and local CLI state needed for qualification. Managers never claim work from task-local copies of `ready`. Local `implemented` and `done` results are proposals to the control-plane record until fence checks, evidence acceptance and publication finish. The transport adapter synchronizes the current task's lease identity into the qualification root and returns only its own status and evidence; it cannot replace the entire authoritative spec with a stale worker copy. This adapter is an orchestration requirement, not a distributed state service provided by the CLI help.

The task packet records mode, exact allowed files, all criteria and capabilities, contracts and decisions, source and spec digests, prerequisite publication receipts, profile digest, fixture inventory, `build_cmd`, the single `verify_cmd`, allowed repairs, deadline, rollback, and the stop condition. Show the task owner four fields on entry: mode, scope, allowed gates, and stop condition. A retry receives the same packet plus the prior attempt's durable evidence and remaining budget.

1. The dispatcher obtains `ready`, filters it by the admission rules, and claims a specific task with the worker's identity. A failed claim starts no sandbox implementation. If a successful claim leaves the task pending, call `start` without `--force` or `--ahead`; otherwise retain the in-progress state already established by the claim.

   ```sh
   cg spec ready -f layerx-merge --root "$PLAN_ROOT"
   cg spec claim "$TASK_ID" --agent "$TASK_OWNER" --ttl 5 -f layerx-merge --root "$PLAN_ROOT"
   cg spec start "$TASK_ID" -f layerx-merge --root "$PLAN_ROOT"
   ```

2. Save the returned attempt id, fence, owner and expiry. The task supervisor renews every 30 seconds using proof that the worker or its declared gate process remains alive. Renewals continue during a long build or test through a supervisor independent of that process. A manager does not keep renewing an unresponsive worker merely because the manager is alive.

   ```sh
   cg spec heartbeat "$TASK_ID" --agent "$TASK_OWNER" --attempt "$ATTEMPT_ID" --fence "$FENCE" --ttl 5 -f layerx-merge --root "$PLAN_ROOT"
   ```

3. Implement the entire task and all its declared tests before formatting, building or testing. Stay within `touches`; planned files create no authority for a duplicate implementation tree. The manager checks the complete candidate against the criteria, real test entry points, fixture inputs and the No Weakening rule before allowing qualification. That review executes no extra tests.

4. In the qualification root, `implemented` graph-checks the coding work without running `verify_cmd`. Its help describes the production workflow; parallel mode uses the production dependency semantics. The coding-complete result is not qualification and never opens a dependency here. If the installed execution version cannot perform this transition in parallel mode, stop at the compatibility check rather than switching modes during a live fleet.

   ```sh
   cg spec implemented "$TASK_ID" -f layerx-merge --root "$TASK_ROOT"
   ```

5. Format only touched files, freeze the resulting source candidate as a local commit, and record its revision. Run the declared `build_cmd` once, from the task's declared working directory. Keep mutable task-board bookkeeping outside the source candidate being attested. Capture the exact command, environment manifest digest, exit code and logs. If the build fails, follow the repair policy before attempting verification.

6. Run `cg spec done` in the same sandbox and on the same source candidate. This command itself runs the declared `verify_cmd` and graph checks. Do not run `verify_cmd` manually first, and do not call `done` again after a successful result merely to record completion centrally. The manager inspects the resulting log for the declared executed test cases, assertions and exit result. Local CLI success is held from authoritative completion if test execution is empty, skipped, incomplete or unrelated to the criterion.

   ```sh
   cg spec done "$TASK_ID" -f layerx-merge --root "$TASK_ROOT"
   ```

7. The manager accepts the fenced outcome, immediately publishes the single task source commit and reviewed technical material, and obtains the publication receipt. It verifies that the integrated change preserves the qualified source blobs. The dispatcher then incorporates only that task's CLI-produced qualified status into the authoritative board and records the integrated revision and publication latch. This status integration runs no second `done` or verification. Keep scheduler and qualification bookkeeping distinct from the task's exact product file ownership. A task is available as a prerequisite only after all these acknowledgements exist.

8. Release the central lease if still held, upload final private logs, revoke fixture access, terminate the sandbox, and return capacity after termination is acknowledged. Release does not change an unqualified task into a qualified one.

   ```sh
   cg spec release "$TASK_ID" --agent "$TASK_OWNER" -f layerx-merge --root "$PLAN_ROOT"
   ```

Publication occurs for every qualified task, with one task per source commit. Push only that task's source commit from its isolated root; never collect neighboring work into it. The control plane serializes integration and task-status publication so two sandboxes cannot overwrite each other's board updates. If transport or publication fails after qualification, retain the source commit and evidence and retry only the transfer. Do not rebuild, reverify, or claim the unpublished task done. An unchanged merge does not justify another task test run; conflict resolution that changes task source returns to the owner with the original remaining repair and time budget.

**Evidence, failure and retry.**

Each command result has one evidence line: `revision=<source revision> command=<exact command> exit=<integer> log=<private artifact path>`. Capture build evidence and the actual `verify_cmd` result separately from the outer `cg spec done` result. A wrapper exit alone is insufficient. A command not started has no passing evidence. Private attempt metadata additionally holds the spec digest, image digest, source tree identity, argv or shell command text, working directory, fixture digest, executed test case identities, counts, lease fence and publication receipt. Redact private system details and credentials before publishing technical evidence.

The task's 30-minute clock starts at its first successful claim and includes all subsequent active work and recovery for that task. There is one initial bounded build and one verification run, followed by at most two repair cycles across all attempts combined. A repair changes task-scoped code, then reruns only the affected failed build or verification; a passing unchanged gate is reused. Rerun dependent verification only when a relevant source or fixture change invalidates it. A replacement sandbox does not reset the timer, build history, or repair count.

On a task-scoped failure, preserve the complete candidate, exact failed command, exit code, logs and repair counter. End the current attempt and release it only after fencing and termination; provision a fresh sandbox for the same task and restore the candidate without broadening its scope. Each recovery cycle consumes one of the two remaining cycles. Stop after the second failed repair or the deadline, whichever occurs first. Report `implemented-not-qualified` if implementation exists, retain the ordinary non-done board state, and block dependents. This reported outcome is not a new status invented for the generated task schema. A failure before implementation is reported as incomplete with the available evidence.

Two consecutive missed 30-second renewals cause the supervisor to suspend work and the dispatcher to quarantine the attempt. Lease expiry, owner mismatch, fence mismatch, or a failed renewal prevents publication immediately. A fresh attempt starts only after the predecessor is fenced and its execution is terminated or otherwise proven unable to act. Never run two live sandboxes for the same task. Before retrying an interrupted gate, recover its durable outcome; reuse a recorded pass and never repeat a recorded failure on unchanged input just to seek a different result. An unknown interrupted outcome remains unqualified and needs an explicit recovery ruling before another gate execution.

Run report-only reconciliation on startup, after a supervisor failure, and at a 60-second service cadence while tasks are active. Inspect stale declarations against live process acknowledgements, artifact state, lease tokens and publication receipts. The orchestrator alone may run repair after that comparison and after fencing the old owner; managers do not force-release one another's live work.

```sh
cg spec reconcile -f layerx-merge --root "$PLAN_ROOT"
# After the dispatcher has proved the declaration stale and fenced its owner:
cg spec reconcile --repair -f layerx-merge --root "$PLAN_ROOT"
```

A pre-existing failure, missing external authority, cross-task contradiction or proposed weakening creates one bounded qualification finding with revision, exact command, exit code and private log location, plus one sentence each for observation and assumption. Deduplicate by task, input revision and observed failure. Do not re-investigate a recorded unrelated finding. Continue independent eligible tasks in the same wave, but hold the wave barrier until every required task is genuinely qualified and published. Restarting a manager reconstructs this state from the durable record; loss of a user connection does not discard the task.

**Enforced checks and the completion decision.**

Specification validation and task qualification enforce different facts. A successful job-only check is partial and cannot certify cross-job relationships. The complete specification check is the authority for the merged graph. Neither it nor CLI exit zero alone establishes behavioral correctness.

| Check | What specification `build.py` actually checks | What the feature manager checks before accepting `done` |
|---|---|---|
| Shape and scope | Grammar, duplicate sections and keys, required keys, identifier and enum forms, wave range, role and profile, exact-file `touches`, at most twelve files, planned-file consistency and test files included in `touches`. | Actual changed files match the packet, follow domain ownership and create no parallel replacement implementation. Generated output is accounted for. The validator does not itself enforce every domain path prefix. |
| Citations | Repository file existence and line bounds for recognized citations. | The cited defining symbol and baseline behavior are true at the pinned revision. A valid line number is not semantic verification. |
| The 1:1 rule | One acceptance block per criterion, unique acceptance test selector, one owning task per selector, task tests tied to acceptance, acceptance tests owned by tasks in a full check, and test-name text present in `verify_cmd`. | Every criterion maps to exactly one real named behavior test; every listed test executes once through its actual unit and asserts the criterion. Compilation and helper setup do not count as tests. |
| Gate command | Nonempty command and declared tests, plus rejection of specific forbidden broad or masked command patterns. | The command selects exactly the listed tests and preserves exit status. Check actual case identities and counts, including Rust module paths, nested test names, and zero-match behavior in Go, Node and Vitest. No skipped case or source scan replaces behavioral evidence. |
| Dependencies and wiring | Task reference shape and existence, cross-domain anchor restrictions, wave ordering, cycles and same-wave file clashes. `build` additionally resolves wiring to fulfilling tasks and adds dependency edges. | All resolved prerequisites are qualified and published, the current wave is open, actual live ownership is exclusive, and no completed task is admitted again. |
| Interfaces | Status values, recognized kinds, citation bounds, contract reference shape, task coverage of changed/new/retired interfaces and refusal of tasks naming stable interfaces. | Actual callable surfaces agree with contracts, registry and source. Contract anchors and semantics exist; stable surfaces have not silently changed. |
| No Weakening | Structural gate restrictions only; no general assertion, bound or type comparison. | Compare the whole candidate with its base: no relaxed assertion, looser bound, widened type, silent fallback, skipped/deleted test, fake unit implementation or disabled check introduced to obtain a pass. Record disagreements with both sides intact. |
| Evidence and publication | Schema and declared metadata, not a real execution or publication receipt. | Same candidate revision for build and verification; real exit results and durable logs; current attempt and fence; bounded run history; scope approval; successful publication and unchanged integration content. |

The manager does this review as part of the owning task's acceptance, using the declared evidence. It does not start a duplicate reviewer or a separate confidence test campaign. No task owner or manager uses `cg spec done --force`. If the declaration's exact test selector is defective, hold completion and request an owner-approved specification repair; changing a selector to match a different or empty suite is not a qualification repair.

**Merged wave gates and final completion.**

At the end of each wave the orchestrator drains workers, integrates every qualified task, freezes one merged revision, and records that revision with the wave's exact ordered gate commands. The gate runs in a separate release allocation once on that merged revision. Task-level builds and tests are not replayed per merge, per manager, or per rebase. If a mandatory aggregate fails, keep the wave closed, record the evidence once, and assign its actual owner the bounded repair. A relevant repair creates a new merged candidate; only invalidated gates are eligible for another execution. An unchanged candidate never receives a confidence rerun.

| Boundary | Declared script | Script-producing task | Execution ownership |
|---|---|---|---|
| After wave 0 | `scripts/gates/wave-0.sh` | `16.1.1` | One orchestrator release allocation |
| After wave 1 | `scripts/gates/wave-1.sh` | `16.1.2` | One orchestrator release allocation |
| After wave 2 | `scripts/gates/wave-2.sh` | `16.1.3` | One orchestrator release allocation |
| After wave 3 | `scripts/gates/wave-3.sh` | `16.1.4` | One orchestrator release allocation |
| After wave 4 | `scripts/gates/wave-4.sh` | `16.1.5` | One orchestrator release allocation |
| After wave 5 | `scripts/gates/wave-5.sh` | `16.1.6` | One orchestrator release allocation |
| After wave 6 | `scripts/gates/wave-6.sh` | `16.1.7` | One orchestrator release allocation |
| Programme completion | `scripts/gates/wave-programme.sh` | `16.1.8` | One final release allocation |

Runner task `16.0.1`, manifest task `17.5.8`, and the script-producing tasks are in wave 0. Their bounded qualification tests check runner behavior and script declarations; `--list` output never substitutes for actually executing a wave aggregate. Persist an execution ledger keyed by wave, merged source revision, command digest and image/fixture digests. Admission of the next wave requires every mandatory gate's successful, revision-matching evidence. A task test's pass cannot be borrowed for a different revision's aggregate result.

There is a pre-dispatch graph conflict in the merged baseline: tasks `17.5.1` through `17.5.7` are all scheduled in wave 7, require `16.97.4` from wave 5, and instruct their tests to execute the earlier wave scripts. They cannot supply barriers before waves 1 through 6, and executing them after boundary runs would repeat aggregate qualification. The graph owner must reconcile these wrappers with the boundary execution ledger before implementation dispatch, preserving all real aggregate commands and assertions. This plan requires the boundary gates above; it does not silently move those tasks, skip their verification, or claim the conflict resolved. The finding is recorded for the final specification repair job.

Wave 7 closes the real per-domain completion tasks `N.0.9`; the programme completion task is `17.0.9`. The final programme gate requires every domain gate, publication receipt, resolved mandatory finding, acceptance evidence and applicable release matrix on its declared candidate. Aggregate browser, sanitizer, historical, performance and full cluster matrices run at their declared wave boundary or final release, outside ordinary task qualification. Missing external inputs remain explicit blockers for affected criteria; a registry of gaps is not passing evidence.

For specification delivery, Python validation and Hyperfide CLI lint are separate gates. The specification builder may launch lint detached for the large merged document; a successful Python build does not certify that lint passed. Delivery requires the recorded lint exit to be zero and bound to the exact emitted specification, with its result newer than that emission. Keep one lint in flight per document, retain its log, and invalidate its result when the specification changes. No control-plane admission or final handoff treats a still-running lint as success.
