from functools import cache
from pathlib import Path

from programme.decisions import load_index, render_record
from programme.kvx import KvxValue, load

ROOT = Path(__file__).resolve().parents[4]
SPEC_PATH = ROOT / "spec/layerx-merge/spec.kvx"


@cache
def _spec() -> dict[str, dict[str, KvxValue]]:
    return load(SPEC_PATH)


def test_records_1_to_11_match_index() -> None:
    index = load_index()
    spec = _spec()
    for identifier in range(1, 12):
        entry = index[identifier]
        name = f"decision.{identifier}"
        decision = spec[name]
        assert entry["record"] == f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        assert entry["carry"] == decision["carry"]
        assert entry["binds"]
        assert len(entry["binds"]) == len(set(entry["binds"]))
        resolutions = [
            section
            for section_name, section in spec.items()
            if section_name.startswith("resolution.")
            and name in section.get("applies_to", [])
        ]
        assert resolutions, name
        bound_tasks = {
            target.removeprefix("task.")
            for section in resolutions
            for target in section["applies_to"]
            if target.startswith("task.")
        }
        assert set(entry["binds"]) == bound_tasks
        assert all(f"task.{task}" in spec for task in entry["binds"])
        record = (ROOT / entry["record"]).read_bytes()
        assert record == render_record(entry, spec).encode("utf-8"), entry["record"]
        text = record.decode("utf-8")
        assert f"Carry status: `{entry['carry']}`." in text
        assert f"## Value\n\n{decision['value']}\n\n" in text
        assert (
            f"## Code basis\n\nBasis revision: `{entry['basis_revision']}`.\n\n"
            f"{entry['basis']}\n\n"
        ) in text
        assert f"Basis revision: `{entry['basis_revision']}`." in text
        assert all(f"- `task.{task}`\n" in text for task in entry["binds"])
