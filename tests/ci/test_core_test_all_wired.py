"""Keep the complete core C gate and its repaired sources reachable from CI."""

from pathlib import Path
import re
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
CORE_PREREQUISITES = set(
    """test test-kernel test-module-ctx test-dispatch test-receipts
    test-state-root test-ledger-accounts test-ledger-transfer test-ledger-set
    test-ledger-send test-ledger-receive test-ledger-send-allowance
    test-ledger-receipt test-asset-registry test-asset-balance test-asset-transfer
    test-asset-deposit test-asset-withdraw test-asset-reserve test-escrow-open
    test-escrow-capture test-escrow-timeout test-escrow-dispute
    test-escrow-invariants test-budget-create test-budget-period test-budget-spend
    test-budget-delegate test-budget-revoke test-stream-open test-stream-accrual
    test-stream-meter test-stream-settle test-stream-lifecycle
    test-wave-8 test-wave-9 test-wave-10 test-wave-11 test-wave-12
    test-bridge-deposit test-bridge-withdraw test-reserve test-gateway
    test-gateway-send test-gateway-kv test-genesis test-genesis-import
    test-genesis-reconcile test-legacy-readonly test-shadow test-contracts
    qualify-faults qualify-arith reproducible scan-consensus test-sanitizers
    test-crypto-sanitizers test-core-ports""".split()
)
PORTED_SUITES = (
    "codec/lxp_test_handover_retired",
    "codec/lxp_test_state_feed_unbound",
    "fuzz/lxp_prop_payer_grant",
    "fuzz/lxp_prop_prepare_workers",
)


def test_ci_runs_core_test_all():
    workflow = yaml.load(
        (ROOT / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader
    )
    assert {"push", "pull_request"} <= workflow["on"].keys()
    job = workflow["jobs"]["c17"]
    assert "if" not in job
    assert "continue-on-error" not in job
    assert set(job["strategy"]["matrix"]["compiler"]) == {"gcc", "clang"}
    steps = job["steps"]
    test_step = next(
        step
        for step in steps
        if shlex.split(step.get("run", "")) == ["make", "build", "test", "scan-consensus"]
    )
    core_step = next(
        step
        for step in steps
        if shlex.split(step.get("run", "")) == ["make", "core-test-all"]
    )
    foundry_step = next(
        step
        for step in steps
        if step.get("uses")
        == "foundry-rs/foundry-toolchain@908c540300062bd5a7e473851cdb4282204cee09"
    )
    assert steps.index(foundry_step) < steps.index(core_step)
    assert "if" not in foundry_step
    assert "continue-on-error" not in foundry_step
    assert foundry_step["with"] == {"version": "v1.8.1", "cache": "false"}
    solidity_foundry_step = next(
        step
        for step in workflow["jobs"]["solidity"]["steps"]
        if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    )
    assert foundry_step["uses"] == solidity_foundry_step["uses"]
    assert foundry_step["with"] == solidity_foundry_step["with"]
    proof_step = next(
        step
        for step in steps
        if shlex.split(step.get("run", ""))
        == [
            "sudo",
            "apt-get",
            "update",
            "&&",
            "sudo",
            "apt-get",
            "install",
            "--yes",
            "cbmc",
            "z3",
            "clang",
        ]
    )
    assert steps.index(proof_step) < steps.index(core_step)
    assert "if" not in proof_step
    assert "continue-on-error" not in proof_step
    assert steps.index(test_step) < steps.index(core_step)
    for step in (test_step, core_step):
        assert "if" not in step
        assert "continue-on-error" not in step
        assert "working-directory" not in step
        assert step["env"]["CC"] == "${{ matrix.compiler }}"
    assert job.get("defaults", {}).get("run", {}).get("working-directory", ".") == "."


def test_core_test_all_prerequisites_exist():
    rules = {}
    recipes = {}
    pending = [ROOT / "Makefile"]
    visited = set()
    while pending:
        path = pending.pop()
        if path in visited:
            continue
        assert path.is_file(), path
        visited.add(path)
        current = []
        for line in path.read_text().replace("\\\n", " ").splitlines():
            if line.startswith("\t"):
                for target in current:
                    recipes.setdefault(target, []).append(line.strip())
                continue
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            current = []
            include = re.match(r"^include\s+(.+)$", line)
            if include:
                for name in include[1].split():
                    assert "$" not in name, f"Unresolved Make include: {name}"
                    pending.append(ROOT / name)
                continue
            if re.match(r"^(?:export\s+)?[A-Za-z_][A-Za-z_0-9]*\s*[:+?]?=", line):
                continue
            match = re.match(r"^([^\s:][^:]*):\s*(.*)$", line)
            if match is None:
                continue
            current = match[1].split()
            for target in current:
                rules.setdefault(target, []).extend(match[2].split())
    prerequisites = set(rules["core-test-all"])
    assert CORE_PREREQUISITES <= prerequisites
    for prerequisite in prerequisites:
        assert prerequisite in rules or (ROOT / prerequisite).is_file(), prerequisite
    assert "core-test-all" in rules[".PHONY"]
    assert "test-core-ports" in rules[".PHONY"]
    ports = set(rules["test-core-ports"])
    assert ports == {f"$(BUILD_DIR)/tests/kernel/{suite}" for suite in PORTED_SUITES}
    pattern = "$(BUILD_DIR)/tests/kernel/%"
    assert "tests/%.c" in rules[pattern]
    assert "$(LIBRARY)" in rules[pattern]
    assert "$(PROGRAMS_RUNTIME_LIB)" in rules[pattern]
    assert "programs-build" in rules[pattern]
    assert any("$(CC)" in recipe and "$<" in recipe for recipe in recipes[pattern])
    commands = [shlex.split(recipe) for recipe in recipes["test-core-ports"]]
    assert len(commands) == len(PORTED_SUITES)
    for suite in PORTED_SUITES:
        source = ROOT / "tests" / f"{suite}.c"
        assert source.is_file(), suite
        binary = f"$(BUILD_DIR)/tests/kernel/{suite}"
        (command,) = [words for words in commands if binary in words]
        cases = re.findall(
            r'\{\s*"([A-Za-z_][A-Za-z_0-9]*)"\s*,\s*([A-Za-z_][A-Za-z_0-9]*)\s*\}',
            source.read_text(),
        )
        assert cases, suite
        assert all(name == function for name, function in cases), suite
        assert command == ["$(RUN_PREFIX)", binary, *(name for name, _ in cases)]
        if suite.endswith(("state_feed_unbound", "prepare_workers")):
            assert "tests/programs/test_call_activity.c" in rules[binary]
