import shlex
import subprocess
from pathlib import Path

from programme.activation import run_goldens
from programme.evidence import evidence_line
from programme.kvx import check_format

ROOT = Path(__file__).resolve().parents[4]
GOLDEN_PATH = Path(__file__).resolve().parents[1] / "data/goldens/receipt-order.kvx"
ORDER_TESTS = (
    "TestOrderCheckRefusesSeqGap",
    "TestOrderCheckRefusesPrevSeqHashMismatch",
)


def test_receipt_order_golden_passes(tmp_path: Path) -> None:
    sections = check_format(GOLDEN_PATH)
    assert set(sections) == {"meta", "golden.receipt-order"}
    fields = sections["golden.receipt-order"]
    assert type(fields["domain"]) is int
    assert fields["domain"] == 5
    assert fields["tests"] == [
        f"node/process_proposal_order_test.go::{test}" for test in ORDER_TESTS
    ]
    assert fields["rule"] == (
        "Receipt order equals block order: sequence numbers are contiguous and "
        "prevSeqHash chains to the last accepted receipt."
    )
    assert fields["leader_source"] == "node/leader_source.go::App.LeaderSchedule"
    assert fields["leader_switch"] == (
        "Below H_act use voting-power leader election; from H_act use the "
        "committed pinned leaders L[round mod len(L)] supplied by App.LeaderSchedule."
    )
    command = fields["command"]
    assert isinstance(command, str)
    assert shlex.split(command) == [
        "go",
        "test",
        "./node",
        "-run",
        f"^({'|'.join(ORDER_TESTS)})$",
        "-count=1",
        "-v",
    ]
    assert type(fields["expect_exit"]) is int
    assert fields["expect_exit"] == 0

    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    directory = tmp_path / "receipt-order"
    results = run_goldens({"golden.receipt-order": fields}, ROOT, directory)
    log_path = directory / "golden.receipt-order.log"
    output = log_path.read_text(encoding="utf-8")
    lines = (directory / "evidence.log").read_text(encoding="utf-8").splitlines()
    assert results == {"golden.receipt-order": True}, f"{lines}\n{output}"
    assert lines == [evidence_line(revision, command, 0, log_path)]
    for test in ORDER_TESTS:
        assert f"=== RUN   {test}\n" in output, f"{test}: {log_path}\n{output}"
        assert f"--- PASS: {test} (" in output, f"{test}: {log_path}\n{output}"
    assert "[no tests to run]" not in output, str(log_path)
