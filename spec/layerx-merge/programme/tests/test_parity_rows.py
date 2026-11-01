import ast
import re
from pathlib import Path, PurePosixPath

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
FEATURE = ROOT / "spec/layerx-merge"
DATA = FEATURE / "programme/data/parity.kvx"


def _inventory_rows() -> dict[tuple[int, str], list[str]]:
    rows = {}
    section = 0
    for line in (FEATURE / "docs/parity-inventory.md").read_text().splitlines():
        heading = re.match(r"## (\d+) ", line)
        if heading:
            section = int(heading[1])
        if section not in (2, 3, 5) or not line.startswith("| "):
            continue
        cells = re.split(r"(?<!\\)\|", line)
        paths = re.findall(r"`([^`]+)`", cells[2])
        if paths:
            title = re.sub(r" \(\d+\)$", "", cells[1].strip())
            key = (section, title)
            assert key not in rows, key
            rows[key] = paths
    return rows


def _declares_test(selector: str) -> bool:
    path, name = selector.split("::")
    source = ROOT / path
    if not source.is_file():
        return False
    text = source.read_text(encoding="utf-8")
    escaped = re.escape(name)
    if source.suffix == ".go":
        return bool(
            name.startswith("Test")
            and re.search(rf"(?m)^func\s+{escaped}\s*\([^)]*\*testing\.T\b", text)
        )
    if source.suffix == ".rs":
        return bool(
            re.search(
                rf"#\[(?:tokio::)?test(?:\([^\]]*\))?\]\s*"
                rf"(?:async\s+)?fn\s+{escaped}\s*\(",
                text,
            )
        )
    if source.suffix == ".c":
        return name == "main" and bool(re.search(r"(?m)^int\s+main\s*\(", text))
    if source.suffix == ".py":
        return any(
            isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))
            and node.name == name
            and name.startswith("test_")
            for node in ast.parse(text).body
        )
    return False


def test_every_row_has_exact_selectors() -> None:
    sections = load(DATA)
    spec = load(FEATURE / "spec.kvx")
    planned = {
        selector
        for section, task in spec.items()
        if section.startswith("task.")
        for selector in task["tests"]
    }
    constructs = {
        name: row for name, row in sections.items() if name.startswith("construct.")
    }
    dropped = {
        name: row for name, row in sections.items() if name.startswith("dropped.")
    }
    assert constructs
    assert set(sections) == {"meta"} | constructs.keys() | dropped.keys()

    inventory = _inventory_rows()
    represented = set()
    active_selectors = set()
    dropped_selectors = set()
    for name, row in (constructs | dropped).items():
        key = (row["source_section"], row["title"])
        assert key in inventory, (name, key)
        assert key not in represented, key
        represented.add(key)
        assert row["source_paths"] == inventory[key], name
        assert type(row["owner_domain"]) is int, name
        assert f"feature.{row['owner_domain']}" in spec, name
        assert type(row["wave"]) is int and 0 <= row["wave"] <= 7, name

        if name in constructs:
            selectors = row["selectors"]
            assert isinstance(selectors, list) and selectors, name
            assert len(selectors) == len(set(selectors)), name
            for selector in selectors:
                assert re.fullmatch(
                    r"(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+::[A-Za-z_][A-Za-z0-9_]*",
                    selector,
                ), (name, selector)
                path = PurePosixPath(selector.split("::")[0])
                assert not {".", ".."}.intersection(path.parts), (name, selector)
                assert path.suffix in {
                    ".go",
                    ".rs",
                    ".c",
                    ".py",
                    ".ts",
                    ".tsx",
                    ".sol",
                    ".exs",
                }, (name, selector)
                assert selector in planned or _declares_test(selector), (
                    name,
                    selector,
                    "test neither exists nor is planned by a phase-2 task",
                )
                active_selectors.add(selector)
            for task_id in row["planned_by"]:
                task = spec[f"task.{task_id}"]
                assert set(task["tests"]).issubset(selectors), (name, task_id)
                owner = spec[f"feature.{row['owner_domain']}"]["name"]
                assert task["owner_role"] == f"fm-{owner}", name
        else:
            assert "selectors" not in row, name
            assert isinstance(row["reason"], str) and row["reason"].strip(), name
            assert isinstance(row["dropped"], list) and row["dropped"], name

        for selector in row.get("dropped", []):
            assert selector not in planned, (name, selector)
            if "::" in selector:
                assert not _declares_test(selector), (name, selector)
            dropped_selectors.add(selector)

    assert represented == set(inventory)
    assert active_selectors.isdisjoint(dropped_selectors)


def test_cutover_goldens_are_listed() -> None:
    sections = load(DATA)
    expected = {title for section, title in _inventory_rows() if section == 5}
    assert expected == {"Balance conservation", "Bond return", "Receipt-order equality"}
    goldens = {
        row["title"]: row
        for name, row in sections.items()
        if name.startswith("construct.") and row["source_section"] == 5
    }
    assert set(goldens) == expected
    required = {
        "Balance conservation": (
            "modules/layerxcustody/keeper/release_test.go"
            "::TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority"
        ),
        "Bond return": (
            "modules/layerxanchor/keeper/retire_merge_test.go"
            "::TestRetireAtMergeReturnsBondsAndEmits"
        ),
        "Receipt-order equality": (
            "node/sequencer/receipt_equality_test.go::TestReceiptEqualsCommittedReceipt"
        ),
    }
    for title, selector in required.items():
        assert goldens[title]["wave"] == 6, title
        assert selector in goldens[title]["selectors"], title
