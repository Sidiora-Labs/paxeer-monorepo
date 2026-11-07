from hashlib import sha256
from pathlib import Path

from programme.kvx import load
from programme.parity import PARITY_PATH, render_doc

DOC_PATH = Path(__file__).resolve().parents[2] / "docs/parity-inventory.md"
ORACLE_SHA256 = "cd0ac6bd9cb39d6c5d93d423f0a5058508d3119933fee86955f8569efaed9dc0"


def test_parity_doc_matches_render() -> None:
    rows = load(PARITY_PATH)
    rendered = render_doc(rows)
    assert DOC_PATH.read_bytes() == rendered.encode("utf-8")

    oracle = rendered.split("## 1 Parity oracle\n", 1)[1].split("## 2 ", 1)[0]
    assert sha256(("## 1 Parity oracle\n" + oracle).encode()).hexdigest() == ORACLE_SHA256

    sections = {
        number: rendered.split(f"## {number} ", 1)[1].split("\n## ", 1)[0]
        for number in (2, 3, 4, 5)
    }
    for name, row in rows.items():
        if not name.startswith(("construct.", "dropped.")):
            continue
        section = sections[row["source_section"]]
        line = next(
            line
            for line in section.splitlines()
            if line.startswith(f"| {row['title']} |")
        )
        cells = line.strip("| ").split(" | ")
        assert cells[1] == "; ".join(f"`{path}`" for path in row["source_paths"])
        assert cells[3:5] == [str(row["owner_domain"]), str(row["wave"])]
        if "selectors" in row:
            assert cells[2] == "; ".join(f"`{test}`" for test in row["selectors"])
            planned = "; ".join(f"`{task}`" for task in row["planned_by"]) or "none"
            assert cells[5] == planned
        else:
            assert cells[2] == "Dropped; see section 4"
        if "dropped" in row:
            dropped_line = next(
                line
                for line in sections[4].splitlines()
                if line.startswith(f"| {row['title']} |")
            )
            dropped_cells = dropped_line.strip("| ").split(" | ")
            assert dropped_cells[1] == "; ".join(
                f"`{test}`" for test in row["dropped"]
            )
            assert dropped_cells[2:5] == [
                str(row["source_section"]),
                str(row["owner_domain"]),
                str(row["wave"]),
            ]
            if "reason" in row:
                assert row["reason"] in dropped_line
