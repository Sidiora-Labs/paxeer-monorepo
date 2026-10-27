import re
from pathlib import Path

from programme.kvx import load

PLAN = Path(__file__).resolve().parents[1] / "data/rehearsal.kvx"
EXPECTED_TESTS = (
    "node/merge_rehearsal_test.go::TestMergeRehearsalFirstSequencedBlock",
    "node/upgrade_export_test.go::TestExportAfterMergeKeepsSequencerState",
    "node/upgrades_merge_activation_test.go::TestMergePlanActivatesGatedPrecompiles",
    "node/store_delete_test.go::TestAnchorStoreDeletedAtStep",
)


def test_steps_are_ordered_and_stop_on_failure() -> None:
    sections = load(PLAN)
    steps = [name for name in sections if name.startswith("step.")]
    assert steps == ["step.1", "step.2", "step.3", "step.4"]
    for name in steps:
        step = sections[name]
        assert step["stop_on_failure"] is True, name
        assert isinstance(step["title"], str) and step["title"].strip(), name

    assert sections["meta"]["format_version"] == 1
    assert sections["meta"]["source_version"] == "v6.12.0"
    assert sections["meta"]["target_version"] == "v7.0"
    assert sections["step.1"]["expect_refusal"] == [
        "mergeUpgradePlan: invalid sequencer genesis (node/upgrades.go:436)",
        "mergeUpgradePlan: no leader list (node/upgrades.go:453)",
        "mergeUpgradePlan: lastSeq other than 0 (node/upgrades.go:456)",
        "mergeUpgradePlan: no receipt key covering H_act+1 (node/upgrades.go:463)",
    ]
    for name in steps[1:]:
        assert sections[name]["expect_refusal"] == [], name


def test_step_commands_run_one_exact_test() -> None:
    sections = load(PLAN)
    steps = [name for name in sections if name.startswith("step.")]
    assert steps == ["step.1", "step.2", "step.3", "step.4"]
    for name, expected_test in zip(steps, EXPECTED_TESTS, strict=True):
        step = sections[name]
        assert step["test"] == expected_test, name
        selector = re.fullmatch(
            r"(?P<package>node)/[A-Za-z0-9_]+_test\.go::"
            r"(?P<test>Test[A-Za-z0-9_]+)",
            step["test"],
        )
        assert selector is not None, name
        package, test = selector.group("package", "test")
        assert step["command"] == (
            f"go test -count=1 ./{package}/ -run '^({test})$'"
        ), name
