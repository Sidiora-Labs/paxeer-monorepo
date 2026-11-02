import shlex
import subprocess
from pathlib import Path

from programme.activation import GOLDEN_PATH, run_goldens
from programme.evidence import evidence_line
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
REHEARSAL = "TestMergeRehearsalFirstSequencedBlock"


def test_bringup_goldens_pass_on_merged_revision(tmp_path: Path) -> None:
    goldens = load(GOLDEN_PATH)
    assert set(goldens) == {
        "golden.accepted",
        "golden.no-leaders",
        "golden.lastseq",
        "golden.no-key",
        "golden.bad-genesis",
    }
    selected_tests: dict[str, list[str]] = {}
    for section, fields in goldens.items():
        test = fields["test"]
        assert isinstance(test, str)
        command = fields["command"]
        assert isinstance(command, str)
        arguments = shlex.split(command)
        assert arguments == ["go", "test", "./node", "-run", f"^{test}$", "-count=1"]
        selected_tests[section] = [test]
        if section == "golden.accepted":
            selected_tests[section].append(REHEARSAL)
            arguments[4] = f"^({test}|{REHEARSAL})$"
        arguments.append("-v")
        fields["command"] = shlex.join(arguments)

    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    directory = tmp_path / "bringup"
    results = run_goldens(goldens, ROOT, directory)
    lines = (directory / "evidence.log").read_text(encoding="utf-8").splitlines()
    assert set(results) == set(goldens)
    assert len(lines) == len(goldens)
    for (section, fields), line in zip(goldens.items(), lines, strict=True):
        log_path = directory / f"{section}.log"
        output = log_path.read_text(encoding="utf-8")
        assert results[section], f"{section}: {line}\n{output}"
        assert line == evidence_line(
            revision, fields["command"], fields["expect_exit"], log_path
        )
        for test in selected_tests[section]:
            assert f"=== RUN   {test}\n" in output, f"{section}: {log_path}"
            assert f"--- PASS: {test} (" in output, f"{section}: {log_path}"
        assert "[no tests to run]" not in output, f"{section}: {log_path}"
