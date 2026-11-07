from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]


def test_routes_table_lists_archive_not_live() -> None:
    document = (ROOT / "docs/system/routes.md").read_text(encoding="utf-8")
    rows = [
        [cell.strip().strip("`") for cell in line.strip("|").split("|")]
        for line in document.splitlines()
        if line.startswith("| ")
    ]
    prefix = "/explorer/backend/api/v2/paxeer-x"
    for suffix in (
        "archive/anchors",
        "archive/receipts",
        "archive/receipts/{id}",
        "capabilities",
    ):
        path = f"{prefix}/{suffix}"
        matches = [row for row in rows if row[1] == path]
        assert matches == [
            ["GET", path, "http", "public-read", "8", "0", "explorer"]
        ], f"Expected exactly one GET row with the existing columns for {path}"

    for row in rows:
        path = row[1]
        assert not any(
            path == f"{prefix}/{retired}"
            or path.startswith(f"{prefix}/{retired}/")
            for retired in ("anchors", "receipts")
        ), f"Retired live API route is still documented: {path}"

    explorer = document.split("\n### explorer\n", 1)[1].split("\n### ", 1)[0]
    note = explorer.rsplit("|", 1)[1].strip()
    assert "Other `/api/v2/paxeer-x` paths answer 404." in note
