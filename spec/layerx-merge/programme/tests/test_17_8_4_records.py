from pathlib import Path

from programme.decisions import Decision, load_index
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
FEATURE = ROOT / "spec/layerx-merge"
SPEC = load(FEATURE / "spec.kvx")
HEAD_BASIS = "7fd177486f7544342d1b9a7a97f6fc8053f99396"


def render_record(entry: Decision) -> str:
    identifier = Path(entry["record"]).stem.removeprefix("D")
    decision = SPEC[f"decision.{identifier}"]
    affects = "\n".join(f"- {item}" for item in decision["affects"])
    binds = "\n".join(f"- `task.{task}`" for task in entry["binds"])
    return (
        f"# D{identifier}: {decision['title']}\n\n"
        f"Carry status: **{entry['carry']}**.\n\n"
        f"Source: `[decision.{identifier}]` in `../../spec.kvx`; "
        "carry status and bound tasks from "
        "`../../programme/data/decisions.kvx`.\n\n"
        f"## Value\n\n{decision['value']}\n\n"
        "## HEAD basis\n\n"
        f"Task input HEAD: `{HEAD_BASIS}`.\n\n"
        f"{decision['basis']}\n\n"
        f"## Affects\n\n{affects}\n\n"
        f"## Bound tasks\n\n{binds}\n\n"
        "[Decision index](INDEX.md)\n"
    )


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
        assert record.read_bytes() == render_record(entry).encode("utf-8"), (
            f"D{identifier} differs from its carry-forward index rendering"
        )
