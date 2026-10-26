# Build report (final build, J100)

## Result

- build.py build: `0 error(s), 19 warning(s), 93073 block(s)`; 0 requires cycles after wiring expansion (Tarjan SCC in build.py).
- wrote out/spec.kvx: 170345 lines (15350 blocks); wrote out/interfaces.kvx: 774914 lines (77723 interface blocks).
- Hyperfide CLI (`cg`) acceptance, started detached by build.py: `cg spec status` only. out/cli.rc = 0 (written 23:20:07; out/spec.kvx written 23:17:29), and out/cli.out reports tasks: 2545 of 2545. `cg spec ready` is not part of the acceptance (amendment 22:46 below); J98's report said the acceptance ran status then ready, but out/cli.out of its run held status output only.
- `cg spec lint` is not run: its cycle check enumerates dependency paths and cannot finish on this plan (logs/coordinator-notes.md 21:53, ruling 21:56). out/lint.rc = 143 is the stopped run of 21:06, kept as evidence. build.py rejects the two things the lint checked: touches paths that are neither at HEAD nor in planned_paths, and cycles.
- Delivery gate: build.py build at 0 error(s) and out/cli.rc reading 0 and newer than out/spec.kvx.

## Document shape: spec.kvx and interfaces.kvx

- out/spec.kvx holds every block except interface blocks; it is the file the Hyperfide CLI reads. out/interfaces.kvx holds the 77,723 interface blocks in the same grammar. Line and block counts: out/spec.kvx: 170345 lines (15350 blocks); out/interfaces.kvx: 774914 lines (77723 interface blocks). Both are delivered to spec/layerx-merge/.
- Reason (CONTRACT.md ruling 22:40): the CLI parser cost grows about quadratically with block count. `cg spec status` took 40 s on the repository's current spec (8,517 blocks), 605 s on the same spec inflated to 48,517 blocks, did not return in 30 min on the merged 93k-block document, and took 216 s on the 15.2k-block spec.kvx without interface blocks. The CLI never consumed interface blocks.
- Interface blocks stay the authority for interfaces: tasks cite them in `interfaces`, contracts/*.md tabulate them, and build.py check and build validate both files exactly as before. They are not merged back.
- Amendment 22:46: `cg spec ready` did not return within 900 s on spec.kvx, so the acceptance is `cg spec status` exiting 0 and reporting every task.

Block counts: interface 77723, acceptance 4025, task 2545, capability 2450, req 2388, override 1333, decision 615, capability_group 599, resolution 599, wiring 536, contract_context 200, service 40, feature 17, meta 1, design 1, execution 1.

Tasks per wave: w0 260, w1 441, w2 439, w3 221, w4 320, w5 408, w6 432, w7 24.

Tasks per domain: kernel 141, ledger-auth-pay 153, markets 170, launchpad 186, node-runtime 232, chain-modules 253, rpc-api 116, platform 213, agent 148, human 125, interop 125, wallet 117, programs 128, web-explorer-bridge 163, cosmos-fork 57, tools-ops 153, programme 65.

Interface status: stable 68959, retire 4359, change 2944, new 1461.

## Hyperfide CLI defects (cg 2.2.0), measured on this plan

1. `spec lint` enumerates dependency paths in its cycle check (exponential). A 24-task DAG of 8 layers of 3 (2,187 paths) lints in 12 s; 48 tasks in 12 layers of 4 (4.2 million paths) and 96 tasks in 16 layers of 6 do not return in 120 s; the repository's own 1,483-task spec did not return in 40 min (logs/coordinator-notes.md 21:53). Fix: Tarjan SCC or Kahn topological sort.
2. The parser and indexer are about quadratic in block count: status 40 s on 8,517 blocks, 605 s on 48,517, over 900 s on 93.5k (repository spec plus 85k decision blocks), over 30 min on the 93k-block merged document (22:40, 22:43).
3. `spec status` is about quadratic in task count: 6 s, 23 s, 43 s and 65 s for 181, 848, 1,205 and 1,424 tasks; 216 s for 2,517 tasks on spec.kvx (22:40).
4. `spec ready` did not return in 900 s on spec.kvx (2,517 tasks, 260 eligible in wave 0), while it takes 52 s on the repository spec (1,483 tasks, 63 eligible) and 7 s on the 48-task synthetic DAG; its cost follows the eligible set, so the parallel conflict view is what grows (22:46, 22:49).

Repro inputs are in the coordinator scratchpad: lint-synth/s4x12, lint-inflate/i40k, lint-noiface.

## Operational precondition for the fleet

Per-call CLI cost on this plan: `cg spec status` about 216 s, `cg spec next` 180 s (correct: task 1.0.1, wave 0), `cg spec ready` over 900 s. `claim-next` computes the same eligibility as `next`, so every sandbox claim costs about 3 min and 500 sandboxes calling claim-next serialise on that cost. Until the CLI is fixed, the fleet cannot schedule this plan at the intended parallelism; `ready` is unusable.

## Sweeps (J98)

- (a) forbidden words over out/spec.kvx and out/interfaces.kvx: no email, vendor or model name remains. Every remaining hit is a literal of the repository, allowed by the 19:28 ruling: TODO (146 + 74) quotes code comments that tasks cite or delete; IPs are loopback, RFC1918 or TEST-NET literals of tests and configs (127.0.0.1, 10.0.0.0, 192.168.1.0, 0.0.0.0; 5.0.8.2 is a task id); URLs are test fixtures (http://127.0.0.1:18081, https://bad.test) and the payload of tests/modules/test_web_intake.c:171; dates occur only inside file names (ethereum-testnets-2026-08-31.json) and the Go layout constant 2006-01-02; 'etc' is the /etc path; 'release/' hits are scripts/release paths, not branch names; '.railway.json' and 'foundry.paxeer.toml' are file names; 'placeholder' describes placeholders in committed example configs. Fixed: the public endpoint host in task.12.0.1.do_2 became a path citation, and an 'etc..' in interface.5.met-metrics-timername-phase-latency-seconds.notes became a full list.
- (b) contract anchors: logs/J98-scripts/check_anchors.py (GitHub heading slugs plus explicit <a id>) reports 0 unresolved in out/spec.kvx (673 anchors, 6705 references) and out/interfaces.kvx (6446 anchors, 77723 references); 21 + 98 references resolve only through an explicit <a id> in rpc.md, sequencer.md and services.md (other jobs' files). logs/J101-scripts/check_anchors.py: 0 unresolved, 0 malformed, current_inputs=True after one rerun of logs/J101-scripts/gen_contracts.py (85 files, 46277 pinned citations verified).
- (c) verify_cmd against sandbox_profile (logs/J98-scripts/sweep_verify.py): every Rust command carries --locked and --exact; the go -run selectors are anchored; vitest -t selectors use a word-boundary group; tasks whose toolchain lies outside their profile use mixed. Remaining and accepted: 19 cargo commands without -p run single-package manifests; 158 node --test-name-pattern tasks carry a GUARD do line, because that runner exits 0 when no case matches; 347 commands with an environment prefix (CGO_ENABLED=1 go test, dotnet, mix, swift, gradle) are outside the sweep's runner parser and were checked by profile only.
- (d) line count: out/spec.kvx 170345 lines; out/interfaces.kvx 774914 lines.

## Warnings (19, all accepted)

Each is a wiring whose path is a file that its fulfilling task creates, so it cannot exist at HEAD:

- [wiring.J22.2]: path modules/layerxledger/keeper/keeper.go does not exist at HEAD
- [wiring.J37.1]: path precompiles/layerxprograms/layerxprograms.go does not exist at HEAD
- [wiring.J40.3]: path precompiles/layerxledger/abi.json does not exist at HEAD
- [wiring.J42.5]: path daemon/committed-fixture/vectors does not exist at HEAD
- [wiring.J49.1]: path precompiles/layerxledger/abi.json does not exist at HEAD
- [wiring.J49.2]: path precompiles/layerxauth/abi.json does not exist at HEAD
- [wiring.J51.1]: path sdk/store/rootmulti/testdata/chain_proof_v1.hex does not exist at HEAD
- [wiring.J51.2]: path consensus/types/testdata/light_commit_v1.json does not exist at HEAD
- [wiring.J51.3]: path consensus/types/testdata/results_hash_v1.json does not exist at HEAD
- [wiring.J51.4]: path modules/lxphost/testdata/store_keys_v1.json does not exist at HEAD
- [wiring.J51.14]: path consensus/crypto/ed25519/testdata/zip215_v1.json does not exist at HEAD
- [wiring.J60.1]: path scripts/gates/wave-0.sh does not exist at HEAD
- [wiring.J60.2]: path scripts/gates/wave-1.sh does not exist at HEAD
- [wiring.J60.3]: path scripts/gates/wave-2.sh does not exist at HEAD
- [wiring.J60.4]: path scripts/gates/wave-3.sh does not exist at HEAD
- [wiring.J60.5]: path scripts/gates/wave-4.sh does not exist at HEAD
- [wiring.J60.6]: path scripts/gates/wave-5.sh does not exist at HEAD
- [wiring.J60.7]: path scripts/gates/wave-6.sh does not exist at HEAD
- [wiring.J60.8]: path scripts/gates/wave-programme.sh does not exist at HEAD

## Changes made by J98

- 2417 in-place fragment edits, one line each in logs/J98.md with block id and reason; no block with content was deleted.
- New fragments: fragments/J98/decision.kvx (decisions 1713-1728), fragments/J98/programs.kvx (task 13.98.1 with req 13981, acceptance 13.981.1 and wiring.J98.1: ReceiptAuthorities, decision.192), fragments/J98/migrate.kvx (27 caller-migration tasks 8.98.1-8.98.13 and 10.98.1-10.98.14 for wiring.J44.20/21/25, decisions 990, 992 and 1720).
- Cycles: eight requires cycles broken (decision.1713). Wave gates: fulfilled per wave (decision.1715). Merge-table statuses and addresses corrected (decision.1714). Code-wins corrections to contract context, services, seal, receipt KeyID, ReleaseRetired, the activation plan, goldens and refusal codes (decisions 1716, 1717, 1721, 1722, 1728). Duplicate J11 launchpad Solidity inventory marked against J16 (decision.1723). Verify-command shapes (decision.1724). The lock entry of 8.9.2 (decision.1726).

## Drift audit (J100)

- 150 of 2,545 tasks sampled uniformly (python random, seed 100100) across all 17 domains and waves 0-7. Each sampled task: every cited path:line read at HEAD, planned test names grepped for an existing definition, verify_cmd checked against its toolchain (Go package and module, cargo workspace member and test target, package.json runner), acceptance Given checked at HEAD, task checked as not already landed. Method, per-category table and every flag's disposition in logs/J100.md.
- Sample defect rate: citation truth 2 of 150 tasks (1.3%; tasks 6.6.4 and 3.50.1); test name already defined 0; verify_cmd shape 0; Given untrue at HEAD 0; already landed 0. Domains 3 (1 of 13) and 6 (1 of 15) exceeded 2 percent and were audited whole for citations: 141 flagged citations read by hand, 9 drifted keys fixed in 4 tasks and 2 reqs (6.6.4, 3.0.3, 3.50.1, 3.42.5, req.6040, req.3001), all off by one to three lines. Whole-domain rate after the audit: domain 3 3 of 170 tasks (1.8%), domain 6 1 of 253 (0.4%).
- Whole document: no two tasks plan the same Go test name in one package, and no planned Go test name is already defined in another file of its package.
- fragments/J96/_gen/stats.json held 2,517 tasks; it was regenerated with stats.py and docs/design.body.md rewritten by gen_design.py (449 lines, 0 missing references; citecheck 0 errors, 159 citations), so the design document's per-domain table matches this build.

## Stale generators (do not rerun; their fragments were edited in place)

fragments/J44/_gen, J47/_gen, J48/_gen and J60/_gen/gen_services.py (the last now refuses to run, decision.1725). J98 also edited in place the fragments of: J01, J03, J08, J10, J11, J17, J22, J24, J25, J27, J28, J30, J31, J33, J36, J40, J41, J44, J45, J46, J49, J50, J51, J53, J55, J56, J58, J59, J60, J61, J67, J79; their _gen scripts, where present, would overwrite these edits. J100 edited in place fragments/J25, J27 and J34 (logs/J100.md); fragments/J96/_gen/stats.json was regenerated and J96's generators may be rerun.

## Open

- Replay retirement versus idempotent-receipt consumers in domains 1, 2 and 13 (decision.1718): the consumers or decision.101 must change before wave 3.
- File-driven merge activation carries no plan Info, so the sequencer genesis is missing (node/upgrades.go:616 against :434; decision.1727).
- Recorded without edit by J98 (decision.1727): pay and programs gas tables; programs native runtime binding and genesis visibility; standby signed replay, accepted-without-receipt, adoption header, replication auth, archive-node; 3.93.1 against wiring.J24.14 and the SQL bundles; operation_task breadth; paxai-worker key source and px_requestFunds; 27 file-only citations; the J86 client-edge findings; the J87 cosmetic citations; transport-first JSON-RPC notes; N.0.9 gates that check only retirement and file existence; C test binaries named as their only test.
- Accepted gaps: no interface kind exists for Python SDK items (none invented); 180 public items in files deleted by 9.24.3, 9.24.6, 9.24.7, 9.25.1 and 9.25.2 have no interface block (fragments/J70/_gen/skipped_deleted.json).
- From logs/J99.md: tasks 17.10.1/17.10.2 edit duplicate findings that the reset qualification.kvx no longer holds; task 17.3.3 emits the previous five-key finding form while the new header follows workflow.kvx entry_keys; docs/plans/test-plan.md and wave-plan.md state 2,517 tasks and 3,997 criteria against 2,545 tasks and 4,025 acceptance blocks built.
- `cg spec lint` is still owed at release once the Hyperfide CLI defects above are fixed.
