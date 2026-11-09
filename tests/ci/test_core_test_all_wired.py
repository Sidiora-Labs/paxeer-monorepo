"""Keep the complete core C gate and its repaired sources reachable from CI."""

from pathlib import Path
import os
import re
import shlex
import shutil
import subprocess

import pytest
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
RETIRED_PAYER_GRANT_SYMBOLS = (
    "lxp_payer_grant_encode",
    "lxp_payer_grant_decode",
    "lxp_grant_authorization_message",
    "lxp_verify_payer_grant",
)
PAYER_GRANT_ARCHIVE = Path(
    os.environ.get("LAYERX_TEST_LIBRARY", ROOT / "build/liblayerx.a")
)


def assert_retired_payer_grant_symbols_absent(archive):
    archive = Path(archive)
    try:
        assert archive.is_file(), f"Missing archive: {archive}"
        assert archive.stat().st_mode & 0o444, f"Unreadable archive: {archive}"
        with archive.open("rb") as stream:
            assert stream.read(8) in (b"!<arch>\n", b"!<thin>\n"), (
                f"Invalid archive: {archive}"
            )
        result = subprocess.run(
            ["nm", "--defined-only", "-A", "--", str(archive)],
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError as error:
        raise AssertionError(
            f"Cannot inspect archive {archive} with nm: {error}"
        ) from error
    assert result.returncode == 0, (
        f"nm failed for {archive} (exit {result.returncode}): {result.stderr}"
    )
    assert not result.stderr.strip(), (
        f"nm could not inspect all of {archive}: {result.stderr}"
    )
    for line in result.stdout.splitlines():
        fields = line.split()
        if fields and fields[-1] in RETIRED_PAYER_GRANT_SYMBOLS:
            raise AssertionError(f"Retired payer-grant definition in {archive}: {line}")


def test_payer_grant_archive_inspects_built_library():
    assert_retired_payer_grant_symbols_absent(PAYER_GRANT_ARCHIVE)


def test_payer_grant_archive_accepts_absent_definitions(tmp_path):
    assert_retired_payer_grant_symbols_absent(PAYER_GRANT_ARCHIVE)
    archive = tmp_path / "unchanged.a"
    shutil.copyfile(PAYER_GRANT_ARCHIVE, archive)
    assert archive.read_bytes() == PAYER_GRANT_ARCHIVE.read_bytes()
    assert_retired_payer_grant_symbols_absent(archive)


@pytest.mark.parametrize("symbol", RETIRED_PAYER_GRANT_SYMBOLS)
@pytest.mark.parametrize("binding", ("strong", "weak"))
def test_payer_grant_archive_rejects_unreferenced_definitions(tmp_path, symbol, binding):
    assert_retired_payer_grant_symbols_absent(PAYER_GRANT_ARCHIVE)
    original = PAYER_GRANT_ARCHIVE.read_bytes()
    archive = tmp_path / "contaminated.a"
    shutil.copyfile(PAYER_GRANT_ARCHIVE, archive)
    source = tmp_path / "unreferenced.c"
    member = tmp_path / "unreferenced.o"
    attribute = "__attribute__((weak)) " if binding == "weak" else ""
    source.write_text(f"{attribute}int {symbol}(void) {{ return 0; }}\n")
    subprocess.run(
        [*shlex.split(os.environ.get("CC", "cc")), "-c", str(source), "-o", str(member)],
        check=True,
        capture_output=True,
        text=True,
    )
    subprocess.run(
        ["ar", "q", str(archive), str(member)],
        check=True,
        capture_output=True,
        text=True,
    )
    members = subprocess.run(
        ["ar", "t", str(archive)], check=True, capture_output=True, text=True
    ).stdout.splitlines()
    assert members.count(member.name) == 1
    definitions = subprocess.run(
        ["nm", "--defined-only", "-A", "--", str(archive)],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.splitlines()
    expected_type = "W" if binding == "weak" else "T"
    assert any(
        member.name in line
        and line.split()[-2:] == [expected_type, symbol]
        for line in definitions
    ), (member, symbol, binding)
    with pytest.raises(AssertionError, match=rf"\b{symbol}\b"):
        assert_retired_payer_grant_symbols_absent(archive)
    assert PAYER_GRANT_ARCHIVE.read_bytes() == original


@pytest.mark.parametrize("problem", ("missing", "unreadable", "invalid", "corrupt"))
def test_payer_grant_archive_rejects_invalid_archives(tmp_path, problem):
    archive = tmp_path / "invalid.a"
    if problem == "unreadable":
        shutil.copyfile(PAYER_GRANT_ARCHIVE, archive)
        archive.chmod(0)
    elif problem == "invalid":
        archive.write_bytes(b"not an archive\n")
    elif problem == "corrupt":
        archive.write_bytes(b"!<arch>\nbroken member\n")
    with pytest.raises(AssertionError, match=re.escape(str(archive))):
        assert_retired_payer_grant_symbols_absent(archive)


def test_payer_grant_archive_requires_symbol_tool(tmp_path, monkeypatch):
    monkeypatch.setenv("PATH", str(tmp_path))
    with pytest.raises(AssertionError, match="with nm"):
        assert_retired_payer_grant_symbols_absent(PAYER_GRANT_ARCHIVE)


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
    assert "test-payer-grant-archive" in rules[".PHONY"]
    ports = set(rules["test-core-ports"])
    assert ports == {
        "test-payer-grant-archive",
        *(f"$(BUILD_DIR)/tests/kernel/{suite}" for suite in PORTED_SUITES),
    }
    assert rules["test-payer-grant-archive"] == ["$(LIBRARY)"]
    archive_commands = [
        shlex.split(recipe) for recipe in recipes["test-payer-grant-archive"]
    ]
    assert archive_commands == [
        [
            "LAYERX_TEST_LIBRARY=$(abspath $(LIBRARY))",
            "python3",
            "-m",
            "pytest",
            "-q",
            "tests/ci/test_core_test_all_wired.py",
            "-k",
            "payer_grant_archive",
        ]
    ]
    assert "$(LIBRARY)" in rules["test-tests-python"]
    python_command = shlex.split(recipes["test-tests-python"][0])
    assert python_command[:4] == [
        "LAYERX_TEST_LIBRARY=$(abspath $(LIBRARY))",
        "python3",
        "-m",
        "pytest",
    ]
    assert "tests/ci" in python_command
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
