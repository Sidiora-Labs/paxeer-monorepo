# Issue triage reports

Each open GitHub issue of the repository is triaged by one task of the layerx-merge
programme. The task reads the issue as recorded in `input/<N>.md` (title and body at
the time the triage was opened), checks every claim against the current revision of
main, and writes one report `<N>.kvx` in this directory. The report is checked by
`check.py <N>` and is the only file the task writes.

## Report shape

```kvx
[issue.<N>]
number = <N>
title = "<issue title>"
revision = "<40-hex commit of main the reading was made at>"
verdict = "<task | already-fixed | not-relevant | duplicate | spec-edit | needs-owner>"
evidence = "<one sentence with path:line or commit cites that settles the verdict>"
close_comment = "<one or two sentences in product wording, postable on the issue when it is closed>"
```

When the verdict is `task`, the report continues with exactly three blocks, blank-line
separated, JSON lists for list values, every string double-quoted on one line:

```kvx
[task.18.<N>.1]
title = "Issue #<N>: <imperative one-line title, product wording>"
status = "pending"
mode = "BUILD"
wave = 1
requires = []
reqs = ["18<N>.1"]
capabilities = []
interfaces = []
do_1 = "<what changes, with path:line cites at the report's revision>"
do_2 = "<the test or check that proves it>"
symbols = ["<symbol the task adds or changes>"]
touches = ["<every file the task will change, and nothing else>"]
tests = ["<path>::<test name>"]
planned_paths = []
build_cmd = "<one build of the touched package>"
verify_cmd = "<one focused test run>"
gate_kind = "planned"
sandbox_profile = "<go | go-cgo | rust | node | python | solidity | elixir | jvm | mixed>"
owner_role = "<role>"
rollback = "Revert the task's commit."
max_builds = 1
max_verify_runs = 1
max_repair_cycles = 2
time_limit_minutes = 30
stop_condition = "Stop on the bounded gate result; record exact revision, command, exit code and private evidence location. Do not mark complete without the declared evidence."
path_policy = "Existing paths are modified in place. Planned paths must be created or explicitly mapped to an existing module before implementation; a planned path never authorizes a duplicate implementation tree. A task touches only the paths it declares."
evidence_policy = "Retain credentials, operational inventories and raw qualification records in access-controlled storage; publish only reviewed technical product material."

[req.18<N>]
title = "<what the fix guarantees>"
ac_1 = "<testable statement with path:line cites>"

[acceptance.18.<N>.1]
feature = "18"
req = "18<N>.1"
scenario = "Given ..., when ..., then ..."
contract = "contracts/<file>.md#<anchor>"
test = "<path>::<test name> (planned)"
qualification = "UNRUN"
```

`wave` is 1 unless the fix depends on another issue task, then `requires` names it and
`wave` is 2. Every id in `requires` must exist in `spec/layerx-merge/spec.kvx`.
`symbols` lists the functions or types the task adds or changes and is empty for a
lock-file, deletion or documentation task. `touches` lists only files the task will
change; a directory entry is satisfied by any change under it.

## Verdicts

- `task`: the defect is real at the report's revision and a code or test change repairs it.
- `already-fixed`: main already behaves as the issue demands; the evidence cites the line or the commit.
- `not-relevant`: the code, file, test or spec block the issue is about left the repository, the commit it reviews was superseded, or the behaviour it asks for contradicts a later decision of spec.kvx; the evidence cites it.
- `duplicate`: another open issue under `input/` describes the same defect; the evidence names it, and the task blocks belong to the other issue's report.
- `spec-edit`: a done task's text drifted from the code; no code task, the evidence says in one sentence what the spec must say.
- `needs-owner`: a money, credential, external-account or product-choice decision that no evidence in the repository settles.

Never weaken a test or a check to make an issue go away; a task repairs the behaviour the
issue describes. Cite line numbers at the report's revision only; a permalink pinned to an
older commit is a pointer, not evidence.

## Gate conventions

- Go: `go build ./<pkg>/...` and `go test -count=1 ./<pkg>/ -run '^(TestA|TestB)$'`; profile `go`, or `go-cgo` when the package imports modules/lxphost, precompiles/*, node/ or anything that links the C kernel.
- Rust: `cargo build --locked -p <crate>` and `cargo test --locked -p <crate> --test <file-stem> <test_name>`; add `--manifest-path <workspace>/Cargo.toml` for a nested workspace; profile `rust`.
- TypeScript: `pnpm --dir <pkg> exec tsc --noEmit` and the package's own focused test runner; profile `node`.
- Python: `python3 -m py_compile <file>` and `python3 -m pytest <file>::<test> -q`; profile `python`.
- Elixir: `services/explorer/deploy/tools/mix-in-builder.sh compile` and `services/explorer/deploy/tools/mix-in-builder.sh test <file> --only 'test:test <name>'`; profile `elixir`.
- Solidity: `bash contracts/bootstrap-libs.sh && FOUNDRY_CONFIG=foundry.paxeer.toml forge build --root .` and `FOUNDRY_CONFIG=foundry.paxeer.toml forge test --root . --match-path <file> --match-test '^test_X$'`; profile `solidity`.
- C kernel (src/, include/layerx, tests/): `make -C . build/liblayerx.a` and `make -C . build/tests/<target> && ./build/tests/<target>`; profile `go-cgo`; owner `fm-kernel`.
- Documentation: `true` and a grep that proves the corrected statement; profile `python`; owner `fm-docs`.

Commands never contain a double quote and never run a whole workspace.

## Owner roles by area

services/platform: fm-platform. apps/perps, modules/layerxexchange, precompiles/perps:
fm-markets. services/explorer, apps/explorer, apps/bridge: fm-web-explorer-bridge.
services/wallet, sdk/wallet: fm-wallet. launchpad, services/launchpad: fm-launchpad.
services/human: fm-human. services/agent: fm-agent. services/interop: fm-interop.
rpc, node, api: fm-rpc-api. modules, precompiles, engine: fm-chain-modules. src,
include/layerx, tests (C kernel): fm-kernel. programs: fm-programs. tools, scripts,
.github: fm-tools-ops. docs: fm-docs. spec: fm-programme. apps/web: fm-web.

## Wording

Product facts only. No names of people, no issue authors, nothing about how or where
work is executed. The owner is "the owner".
