import os
import pathlib
import re
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]


def run_make(*arguments):
    return subprocess.run(
        ["make", *arguments],
        cwd=ROOT,
        env={**os.environ, "LC_ALL": "C"},
        capture_output=True,
        text=True,
        check=False,
    )


def test_relay_archive_targets_absent():
    for target in ("relay-archive-build", "relay-archive-e2e"):
        result = run_make("-n", target)
        assert result.returncode == 2, result.stdout + result.stderr
        assert f"No rule to make target '{target}'" in result.stderr
    assert "tests/relay-archive" not in (ROOT / "Makefile").read_text()


def test_parallel_differential_rule_omits_batch_wal():
    makefile = (ROOT / "Makefile").read_text()
    rule = re.search(
        r"^\$\(BUILD_DIR\)/tests/programs_parallel_differential:.*?(?=\n\n)",
        makefile,
        re.M | re.S,
    )
    assert rule is not None, "The parallel differential build rule is missing"
    assert "lxp_daemon_batch_wal" not in rule.group()
    target = "build/tests/programs_parallel_differential"
    dry_run = run_make("-n", target)
    assert dry_run.returncode == 0, dry_run.stdout + dry_run.stderr
    assert "lxp_daemon_batch_wal" not in dry_run.stdout + dry_run.stderr


def test_parallel_differential_links_without_batch_wal():
    test_parallel_differential_rule_omits_batch_wal()


def test_state_diff_rules_absent():
    retired = run_make("-n", "test-state-diff")
    assert retired.returncode == 2, retired.stdout + retired.stderr
    assert "No rule to make target 'test-state-diff'" in retired.stderr
    assert "lxp_test_state_diff" not in (ROOT / "Makefile").read_text()
    aggregate = run_make("-n", "test")
    assert aggregate.returncode == 0, aggregate.stdout + aggregate.stderr
    assert "lxp_test_state_diff" not in aggregate.stdout
