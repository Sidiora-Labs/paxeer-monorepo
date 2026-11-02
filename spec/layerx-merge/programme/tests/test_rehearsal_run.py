from pathlib import Path
from tempfile import mkdtemp

from programme import rehearsal
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
PLAN = ROOT / "spec/layerx-merge/programme/data/rehearsal.kvx"
QUALIFICATION = ROOT / "spec/layerx-merge/qualification.kvx"
EXPECTED_TESTS = (
    "node/merge_rehearsal_test.go::TestMergeRehearsalFirstSequencedBlock",
    "node/upgrade_export_test.go::TestExportAfterMergeKeepsSequencerState",
    "node/upgrades_merge_activation_test.go::TestMergePlanActivatesGatedPrecompiles",
    "node/store_delete_test.go::TestAnchorStoreDeletedAtStep",
)


def test_rehearsal_passes_on_merged_revision() -> None:
    sections = load(PLAN)
    names = [name for name in sections if name.startswith("step.")]
    assert names == ["step.1", "step.2", "step.3", "step.4"]
    steps = [sections[name] for name in names]
    for step, expected in zip(steps, EXPECTED_TESTS, strict=True):
        assert step["test"] == expected
        test_name = expected.split("::")[1]
        assert step["command"] == f"go test -count=1 ./node/ -run '^({test_name})$'"
        assert step["stop_on_failure"] is True

    logs_root = ROOT / ".logs"
    logs_root.mkdir(mode=0o700, exist_ok=True)
    log_dir = Path(mkdtemp(prefix="layerx-merge-rehearsal-", dir=logs_root))
    result = rehearsal.run_steps(steps, ROOT, log_dir)
    finding = rehearsal.to_finding(result, "17.3.4")
    if finding:
        with QUALIFICATION.open("a", encoding="utf-8") as ledger:
            ledger.write("\n" + finding)

    assert result.passed is True, "\n".join(result.evidence)
    assert result.failed_step is None
    assert result.exit_code == 0
    assert len(result.evidence) == 4
    assert (
        (log_dir / "evidence.log").read_text(encoding="utf-8").splitlines()
        == result.evidence
    )
