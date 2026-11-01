import re
from pathlib import Path

from programme import decisions
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
FEATURE = ROOT / "spec/layerx-merge"
RECORD_IDS = (34, *range(1701, 1707))


def records_missing() -> list[int]:
    """Read the installed index through the real programme record consumer."""
    return decisions.records_missing(decisions.load_index(), ROOT)


def test_index_rows_and_records_complete() -> None:
    index = decisions.load_index()
    spec = load(FEATURE / "spec.kvx")
    expected_ids = [*range(1, 95), *range(1701, 1721)]
    assert sorted(index) == expected_ids

    document = (FEATURE / "docs/decisions/INDEX.md").read_text(encoding="utf-8")
    table = [line for line in document.splitlines() if line.startswith("|")]
    assert table[:2] == [
        "| Id | Title | Carry | Affects | Resolutions |",
        "|---|---|---|---|---|",
    ]
    rows = [
        [cell.strip() for cell in line.strip("|").split("|")]
        for line in table[2:]
    ]
    assert len(rows) == len(expected_ids)
    seen_ids = []
    for row in rows:
        assert len(row) == 5
        match = re.fullmatch(r"\[D([1-9][0-9]*)\]\(D\1\.md\)", row[0])
        assert match is not None, row[0]
        identifier = int(match[1])
        seen_ids.append(identifier)
        entry = index[identifier]
        decision = spec[f"decision.{identifier}"]
        assert row[1] == decision["title"]
        assert row[2] == entry["carry"]
        assert row[3] == ", ".join(decision["affects"])
        resolutions = [
            (name, section)
            for name, section in spec.items()
            if name.startswith("resolution.")
            and f"decision.{identifier}" in section.get("applies_to", [])
        ]
        assert row[4] == str(len(resolutions))
        assert entry["record"] == f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        assert (FEATURE / "docs/decisions" / f"D{identifier}.md").is_file()

        if identifier in RECORD_IDS:
            record = (ROOT / entry["record"]).read_text(encoding="utf-8")
            assert record.splitlines()[0] == f"# D{identifier}: {decision['title']}"
            assert f"Programme carry status: **{entry['carry']}**" in record
            assert f"source carry: `{decision['carry']}`" in record
            assert f"## Value\n\n{decision['value']}\n" in record
            assert re.search(r"## Basis\n\n\S[^\n]*\n", record)
            assert re.search(r"## Carry-forward\n\n\S[^\n]*\n", record)
            for affected in decision["affects"]:
                assert f"\n- {affected}\n" in record
            for name, resolution in resolutions:
                assert (
                    f"| `{name}` | {resolution['title']} | "
                    f"{resolution['contract']} | "
                    f"{', '.join(resolution['applies_to'])} |"
                ) in record
            assert "[Decision index](INDEX.md)" in record

    assert seen_ids == expected_ids
    assert records_missing() == []
    assert decisions.records_missing(
        {identifier: index[identifier] for identifier in RECORD_IDS}, ROOT
    ) == []
