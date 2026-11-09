"""Every wallet repair keeps a colocated suite in the public workflow's test scope."""

from pathlib import Path
import re
import shlex

import yaml

from test_pnpm_workspace_roots import expand_matrix, matrix_rows, run_directory


ROOT = Path(__file__).resolve().parents[2]
WALLET = ROOT / "apps/wallet"
SOURCE = WALLET / "src"
ACCOUNT_SUITE = SOURCE / "account/test/repair-22-33-162.test.ts"
ACCOUNT_RECORD = ACCOUNT_SUITE.with_name("repair-22-33-162.kvx")


def repair_suites():
    records = sorted(SOURCE.rglob("repair*.kvx"))
    assert records, "Wallet repair inventory must not be empty"
    inventory = {}
    for record in records:
        text = record.read_text()
        explicit = set()
        for name in re.findall(r"(?<![\w./-])[\w./\[\]-]+\.test\.tsx?", text):
            if name.startswith(("apps/wallet/src/", "src/")):
                suite = WALLET / name.removeprefix("apps/wallet/")
            elif "/" not in name:
                suite = record.parent / name
                if not suite.is_file():
                    matches = sorted(SOURCE.rglob(name))
                    assert len(matches) == 1, (
                        f"{record.relative_to(ROOT)} references {name}, which must "
                        f"resolve to exactly one wallet suite: {matches}"
                    )
                    suite = matches[0]
            else:
                suite = record.parent / name
            explicit.add(suite)
        canonical = {
            record.with_suffix(extension)
            for extension in (".test.ts", ".test.tsx")
            if record.with_suffix(extension).is_file()
        }
        suites = explicit | canonical
        if not suites:
            suites = {record.with_suffix(".test.ts")}
        assert any(suite.parent == record.parent for suite in suites), (
            f"{record.relative_to(ROOT)} must retain a colocated repair suite"
        )
        inventory[record] = suites
    return inventory


def test_wallet_repair_suites_exist():
    inventory = repair_suites()
    assert ACCOUNT_SUITE in inventory[ACCOUNT_RECORD]
    for record, suites in inventory.items():
        for suite in suites:
            assert suite.is_file(), (
                f"{record.relative_to(ROOT)} references missing {suite.relative_to(ROOT)}"
            )
            assert suite.suffix in {".ts", ".tsx"}
            assert suite.is_relative_to(SOURCE)


def test_wallet_repair_suites_named_in_workflow_scope():
    workflow = yaml.load(
        (ROOT / ".github/workflows/wallet-test.yml").read_text(),
        Loader=yaml.BaseLoader,
    )
    assert {"pull_request", "push", "workflow_dispatch"} <= workflow["on"].keys()
    job = workflow["jobs"]["vitest"]
    assert "apps/wallet" in job["strategy"]["matrix"]["directory"]
    assert "if" not in job
    assert "continue-on-error" not in job
    run = next(step for step in job["steps"] if step.get("name") == "Run Vitest suite")
    assert run["working-directory"] == "${{ matrix.directory }}"
    assert run["run"] == "${{ matrix.command }}"
    (wallet_row,) = [
        row for row in matrix_rows(job) if row["directory"] == "apps/wallet"
    ]
    assert run_directory(workflow, job, run, wallet_row) == WALLET
    assert shlex.split(expand_matrix(run["run"], wallet_row)) == [
        "pnpm", "exec", "vitest", "run"
    ]
    assert "if" not in run
    assert "continue-on-error" not in run

    config = (WALLET / "vitest.config.ts").read_text()
    include = re.search(r"\binclude:\s*\[([^]]+)\]", config)
    assert include, "Vitest must explicitly include the wallet repair suites"
    patterns = re.findall(r"['\"]([^'\"]+)['\"]", include[1])
    assert patterns, "Vitest include patterns must not be empty"
    assert "exclude:" not in config, "Repair suites must not be excluded from the public run"
    included = {pattern: set(WALLET.glob(pattern)) for pattern in patterns}
    inventory = repair_suites()
    assert any(ACCOUNT_SUITE in suites for suites in inventory.values())
    for record, suites in inventory.items():
        for suite in suites:
            relative = suite.relative_to(WALLET)
            matched_patterns = [
                pattern for pattern, paths in included.items() if suite in paths
            ]
            assert matched_patterns, (
                f"{record.relative_to(ROOT)} suite {relative} is outside Vitest workflow scope"
            )
