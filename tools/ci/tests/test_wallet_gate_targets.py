import os
import subprocess
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ("gate-test.sh", "gate-lint.sh")


@pytest.mark.parametrize("script", SCRIPTS)
def test_builtin_targets_exist(script):
    env = os.environ.copy()
    env.pop("WALLET_GATE_TARGETS_FILE", None)
    env["WALLET_GATE_BUDGET_SECONDS"] = "30"
    result = subprocess.run(
        ["bash", str(ROOT / "scripts/wallet" / script), "--check"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
    )

    assert "syntax: ok\n" in result.stdout, result.stdout + result.stderr
    assert "targets:\n" in result.stdout, result.stdout + result.stderr
    rows = [
        line.strip().split("\t", 3)
        for line in result.stdout.split("targets:\n", 1)[1].splitlines()
    ]
    assert len(rows) == (6 if script == "gate-test.sh" else 12)
    for label, status, directory, command in rows:
        assert status == "exists", f"{script}: {label}: {directory}"
        assert (ROOT / directory).is_dir(), f"{script}: {label}: {directory}"
        assert command, f"{script}: {label}: missing command"
        if label.startswith("rust-human"):
            assert directory == "services/human"
        elif label.startswith("rust-platform"):
            assert directory == "services/platform"


@pytest.mark.parametrize("script", SCRIPTS)
@pytest.mark.parametrize("missing", ("directory", "marker"))
def test_absent_target_fails_gate(tmp_path, script, missing):
    directory = tmp_path / "missing-directory" if missing == "directory" else tmp_path
    marker = "Cargo.toml"
    targets = tmp_path / "targets.tsv"
    targets.write_text(f"missing-target\t{directory}\tpwd\t{marker}\n", encoding="utf-8")
    env = os.environ.copy()
    env.update(
        WALLET_GATE_TARGETS_FILE=str(targets),
        WALLET_GATE_LOG_DIR=str(tmp_path / "logs"),
        WALLET_GATE_BUDGET_SECONDS="30",
    )
    result = subprocess.run(
        ["bash", str(ROOT / "scripts/wallet" / script)],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
    )

    assert result.returncode == 1, result.stdout + result.stderr
    missing_path = directory if missing == "directory" else directory / marker
    assert f"absent: missing-target ({missing_path})" in result.stdout
    assert "summary: ran=0 passed=0 failed=0 absent=1 result=INCOMPLETE" in result.stdout
