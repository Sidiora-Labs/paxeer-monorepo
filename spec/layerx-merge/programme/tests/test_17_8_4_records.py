from pathlib import Path

from programme.decisions import load_index, render_record
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
FEATURE = ROOT / "spec/layerx-merge"
SPEC = load(FEATURE / "spec.kvx")


def test_records_23_to_33_match_index() -> None:
    index = load_index(FEATURE / "programme/data/decisions.kvx")
    expected_ids = set(range(23, 34))
    bound_ids = {
        identifier
        for identifier, entry in index.items()
        if "17.8.4" in entry["binds"]
    }
    assert bound_ids == expected_ids

    for identifier in sorted(expected_ids):
        entry = index[identifier]
        assert entry["record"] == (
            f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        )
        decision_name = f"decision.{identifier}"
        assert entry["carry"] == SPEC[decision_name]["carry"]
        assert all(f"task.{task}" in SPEC for task in entry["binds"])
        resolution = SPEC[f"resolution.17.{identifier}"]
        assert set(resolution["applies_to"]) == {decision_name} | {
            f"task.{task}" for task in entry["binds"]
        }
        record = ROOT / entry["record"]
        assert record.read_bytes() == render_record(entry, SPEC).encode("utf-8"), (
            f"D{identifier} differs from its carry-forward index rendering"
        )
