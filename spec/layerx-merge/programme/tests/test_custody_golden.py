import shlex
import subprocess
from pathlib import Path

from programme.activation import GOLDEN_PATH, run_goldens
from programme.evidence import evidence_line
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
CASE_TESTS = {
    "golden.custody-release": (
        "./modules/layerxcustody/keeper",
        "TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority",
    ),
    "golden.anchor-retired": (
        "./precompiles/layerxanchor",
        "TestEveryAnchorSelectorRetired",
    ),
}


def test_custody_and_anchor_goldens_pass(tmp_path: Path) -> None:
    goldens = load(GOLDEN_PATH)
    selected = {section: goldens[section] for section in CASE_TESTS}
    for section, (package, test) in CASE_TESTS.items():
        fields = selected[section]
        assert fields["test"] == test, section
        command = fields["command"]
        assert isinstance(command, str), section
        assert shlex.split(command) == [
            "go",
            "test",
            package,
            "-run",
            f"^{test}$",
            "-count=1",
            "-v",
        ], section
        assert type(fields["expect_exit"]) is int, section
        assert fields["expect_exit"] == 0, section

    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    directory = tmp_path / "custody-and-anchor"
    results = run_goldens(selected, ROOT, directory)
    lines = (directory / "evidence.log").read_text(encoding="utf-8").splitlines()
    assert set(results) == set(CASE_TESTS)
    assert len(lines) == len(CASE_TESTS)
    for (section, fields), line in zip(selected.items(), lines, strict=True):
        log_path = directory / f"{section}.log"
        output = log_path.read_text(encoding="utf-8")
        assert results[section], f"{section}: {line}\n{output}"
        assert line == evidence_line(revision, fields["command"], 0, log_path)
        test = CASE_TESTS[section][1]
        assert f"=== RUN   {test}\n" in output, f"{section}: {log_path}"
        assert f"--- PASS: {test} (" in output, f"{section}: {log_path}"
        assert "[no tests to run]" not in output, f"{section}: {log_path}"
    assert results == {section: True for section in CASE_TESTS}
