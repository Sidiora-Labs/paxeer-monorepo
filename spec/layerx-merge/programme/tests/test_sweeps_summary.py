import subprocess
import sys
from pathlib import Path

import pytest

from programme.kvx import load
from programme.sweeps_summary import ROOT, _load_report, sweeps_summary


CHECKER = Path(__file__).resolve().parents[1] / "sweeps_summary.py"


def _revision() -> str:
    return subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def _report(directory: Path, name: str, targets: str, counts: tuple[int, ...]) -> Path:
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / f"{name}.kvx"
    total, passed, failed, not_run = counts
    path.write_text(
        f"[sweep.{name}]\n"
        f'revision = "{_revision()}"\n'
        'command = "python3 -m pytest TARGET"\n'
        f"targets_total = {total}\n"
        f"targets_passed = {passed}\n"
        f"targets_failed = {failed}\n"
        f"targets_not_run = {not_run}\n\n"
        + targets,
        encoding="utf-8",
    )
    return path


def test_sweeps_summary_counts_and_copies_failures(tmp_path: Path) -> None:
    spec_dir = tmp_path / "spec with spaces"
    directory = spec_dir / "programme" / "sweeps"
    _report(
        directory,
        "z-last",
        '[target.z-last.1]\ntarget = "z-failing"\nresult = "failed"\n'
        'exit_code = 1\nobserved = "tests/a.py:7: \\"bad # value\\" at C:\\temp."\n'
        'assumption = "The expected value likely predates the interface."\n'
        '[target.z-last.2]\ntarget = "a-passing"\nresult = "passed"\nexit_code = 0\n',
        (2, 1, 1, 0),
    )
    _report(
        directory,
        "a-first",
        '[target.a-first.1]\ntarget = "z-deferred"\nresult = "not_run"\n'
        'exit_code = -1\nobserved = "The time budget expired."\n'
        'assumption = "The target requires a separate bounded run."\n'
        '[target.a-first.2]\ntarget = "a-deferred"\nresult = "not_run"\n'
        'exit_code = -1\nreason = "No allocation remained."\n'
        '[target.a-first.3]\ntarget = "passing"\nresult = "passed"\nexit_code = 0\n',
        (3, 1, 0, 2),
    )
    (directory / "interfaces.kvx").write_text('[interfaces]\nrows = 0\n')
    destination = directory / "summary.kvx"
    destination.write_text('[summary]\nreports = 999\n')

    assert sweeps_summary(spec_dir) == destination
    sections = load(destination)
    assert sections["summary"] == {
        "reports": 2,
        "targets_total": 5,
        "targets_passed": 2,
        "targets_failed": 1,
        "targets_not_run": 2,
    }
    expected = [
        {
            "sweep": "a-first",
            "target": "a-deferred",
            "result": "not_run",
            "observed": "",
            "assumption": "",
        },
        {
            "sweep": "a-first",
            "target": "z-deferred",
            "result": "not_run",
            "observed": "The time budget expired.",
            "assumption": "The target requires a separate bounded run.",
        },
        {
            "sweep": "z-last",
            "target": "z-failing",
            "result": "failed",
            "observed": 'tests/a.py:7: "bad # value" at C:\\temp.',
            "assumption": "The expected value likely predates the interface.",
        },
    ]
    assert list(sections) == ["summary", "failing.1", "failing.2", "failing.3"]
    assert [sections[f"failing.{index}"] for index in range(1, 4)] == expected
    original = destination.read_bytes()
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_dir)],
        cwd=tmp_path,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert destination.read_bytes() == original

    for report in directory.glob("*.kvx"):
        if report.name != "summary.kvx":
            report.unlink()
    _report(directory, "empty", "", (0, 0, 0, 0))
    sweeps_summary(spec_dir)
    assert load(destination) == {
        "summary": {
            "reports": 1,
            "targets_total": 0,
            "targets_passed": 0,
            "targets_failed": 0,
            "targets_not_run": 0,
        }
    }


@pytest.mark.parametrize("elapsed", ["780.788", "+780.788", "-780.788", "0.0"])
def test_sweeps_summary_accepts_decimal_values(tmp_path: Path, elapsed: str) -> None:
    directory = tmp_path / "programme" / "sweeps"
    report = _report(
        directory,
        "decimal",
        '[target.decimal.1]\ntarget = "package"\nresult = "failed"\n'
        f"elapsed_seconds = {elapsed} # decimal measurement\n"
        'exit_code = 1\nobserved = "The assertion failed."\n'
        'assumption = "The expected value is stale."\n',
        (1, 0, 1, 0),
    )
    parsed = _load_report(report)
    measurement = parsed["target.decimal.1"]["elapsed_seconds"]
    assert type(measurement) is float
    assert measurement == float(elapsed)
    assert type(parsed["sweep.decimal"]["targets_total"]) is int

    sections = load(sweeps_summary(tmp_path))
    assert sections["summary"] == {
        "reports": 1,
        "targets_total": 1,
        "targets_passed": 0,
        "targets_failed": 1,
        "targets_not_run": 0,
    }
    assert sections["failing.1"] == {
        "sweep": "decimal",
        "target": "package",
        "result": "failed",
        "observed": "The assertion failed.",
        "assumption": "The expected value is stale.",
    }


@pytest.mark.parametrize("name", ["go-modules", "go-launchpad-modules"])
def test_sweeps_summary_second_pass_replaces_not_run(tmp_path: Path, name: str) -> None:
    directory = tmp_path / "programme" / "sweeps"
    _report(
        directory,
        name,
        "".join(
            f"[target.{name}.{index}]\n"
            f'target = "{target}"\nresult = "not_run"\n'
            'reason = "The first-pass time budget expired."\n'
            for index, target in enumerate(
                ("now-passed", "now-failed", "still-deferred", "untouched"), start=1
            )
        )
        + f"[target.{name}.5]\n"
        'target = "previous-failure"\nresult = "failed"\n'
        'observed = "The original assertion failed."\n'
        'assumption = "The original expectation may be stale."\n',
        (5, 0, 1, 4),
    )
    second_pass = f"{name}-2"
    _report(
        directory,
        second_pass,
        f"[target.{second_pass}.1]\n"
        'target = "now-passed"\nresult = "passed"\n'
        f"[target.{second_pass}.2]\n"
        'target = "now-failed"\nresult = "failed"\n'
        'observed = "The second-pass assertion failed."\n'
        'assumption = "The second-pass expectation may be stale."\n'
        f"[target.{second_pass}.3]\n"
        'target = "still-deferred"\nresult = "not_run"\n'
        'observed = "The second-pass time budget expired."\n'
        'assumption = "This target requires another bounded run."\n',
        (3, 1, 1, 1),
    )

    destination = sweeps_summary(tmp_path)
    sections = load(destination)
    assert sections["summary"] == {
        "reports": 2,
        "targets_total": 5,
        "targets_passed": 1,
        "targets_failed": 2,
        "targets_not_run": 2,
    }
    failures = [row for key, row in sections.items() if key.startswith("failing.")]
    assert len(failures) == 4
    by_target = {row["target"]: row for row in failures}
    assert set(by_target) == {
        "now-failed",
        "still-deferred",
        "untouched",
        "previous-failure",
    }
    assert by_target["now-failed"] == {
        "sweep": second_pass,
        "target": "now-failed",
        "result": "failed",
        "observed": "The second-pass assertion failed.",
        "assumption": "The second-pass expectation may be stale.",
    }
    assert by_target["still-deferred"] == {
        "sweep": second_pass,
        "target": "still-deferred",
        "result": "not_run",
        "observed": "The second-pass time budget expired.",
        "assumption": "This target requires another bounded run.",
    }
    assert by_target["untouched"]["sweep"] == name
    assert by_target["untouched"]["result"] == "not_run"
    assert by_target["previous-failure"]["sweep"] == name
    assert by_target["previous-failure"]["result"] == "failed"
    original = destination.read_bytes()
    sweeps_summary(tmp_path)
    assert destination.read_bytes() == original


@pytest.mark.parametrize(
    ("old", "new", "field"),
    [
        ("targets_total = 1", "targets_total = 2", "targets_total"),
        ("targets_passed = 0", "targets_passed = 1", "targets_passed"),
        ("targets_failed = 1", "targets_failed = 0", "targets_failed"),
        ("targets_not_run = 0", "targets_not_run = 1", "targets_not_run"),
        ("targets_total = 1", 'targets_total = "1"', "targets_total"),
        ("targets_total = 1", "targets_total = 1.0", "targets_total"),
        ('observed = "The assertion failed."\n', "", "observed"),
        ('assumption = "The expected value is stale."\n', "", "assumption"),
        ('observed = "The assertion failed."', 'observed = " "', "observed"),
        ('result = "failed"', 'result = "unknown"', "result"),
        ("REVISION", "not-a-commit", "revision"),
        ("REVISION", "0" * 40, "revision"),
    ],
)
def test_sweeps_summary_names_a_report_whose_counts_disagree(
    tmp_path: Path, old: str, new: str, field: str
) -> None:
    directory = tmp_path / "programme" / "sweeps"
    report = _report(
        directory,
        "bad-counts",
        '[target.bad-counts.1]\ntarget = "package"\nresult = "failed"\n'
        'exit_code = 1\nobserved = "The assertion failed."\n'
        'assumption = "The expected value is stale."\n',
        (1, 0, 1, 0),
    )
    if old == "REVISION":
        old = _revision()
    report.write_text(report.read_text(encoding="utf-8").replace(old, new))
    with pytest.raises(ValueError) as caught:
        sweeps_summary(tmp_path)
    assert str(report) in str(caught.value)
    assert field in str(caught.value)
    assert not (directory / "summary.kvx").exists()

    result = subprocess.run(
        [sys.executable, str(CHECKER), str(tmp_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1
    assert str(report) in result.stderr
    assert field in result.stderr
