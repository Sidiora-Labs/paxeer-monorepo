import re
from pathlib import Path

from programme.decisions import load_index, unbound
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]


def test_every_decision_binds_existing_tasks(tmp_path: Path) -> None:
    spec_path = ROOT / "spec/layerx-merge/spec.kvx"
    assert unbound(spec_path) == []

    sections = load(spec_path)
    index = load_index()
    source = spec_path.read_text(encoding="utf-8")
    for missing in ("task.17.8.1", "decision.1"):
        assert missing in sections
        reduced, count = re.subn(
            rf"^\[{re.escape(missing)}\]\n.*?(?=^\[|\Z)",
            "",
            source,
            flags=re.MULTILINE | re.DOTALL,
        )
        assert count == 1
        candidate = tmp_path / f"without-{missing}.kvx"
        candidate.write_text(reduced, encoding="utf-8")
        assert missing not in load(candidate)

        expected_decisions = sorted(
            identifier
            for identifier, entry in index.items()
            if any(f"task.{task}" == missing for task in entry["binds"])
        )
        expected_resolutions = sorted(
            name
            for name, section in sections.items()
            if name.startswith("resolution.17.")
            and missing in section["applies_to"]
        )
        assert expected_resolutions
        if missing.startswith("task."):
            assert expected_decisions
        assert unbound(candidate) == [*expected_decisions, *expected_resolutions]
