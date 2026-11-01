import json
from pathlib import Path

from programme.retired import load_components, stale_rows

ROOT = Path(__file__).resolve().parents[4]


def test_no_stale_rows_after_retirement(tmp_path: Path) -> None:
    assert stale_rows(ROOT) == []

    inventory = load_components()
    source = json.loads(
        (ROOT / "tools/paxeer-x/retired_callers.json").read_text(encoding="utf-8")
    )
    scan = tmp_path / "tools/paxeer-x/retired_callers.json"
    scan.parent.mkdir(parents=True)
    scan.write_text(json.dumps(source), encoding="utf-8")
    assert stale_rows(tmp_path) == []

    deleted_name = next(
        name
        for name in source["components"]
        if len(inventory[name]["deleted_trees"]) > 1
    )
    deleted_component = source["components"].pop(deleted_name)
    scan.write_text(json.dumps(source), encoding="utf-8")
    trees = [tmp_path / tree for tree in inventory[deleted_name]["deleted_trees"]]
    for tree in trees:
        tree.parent.mkdir(parents=True, exist_ok=True)
        tree.write_text("retirement not yet applied\n", encoding="utf-8")
    assert stale_rows(tmp_path) == [deleted_name]

    for tree in trees[:-1]:
        tree.unlink()
    assert stale_rows(tmp_path) == [deleted_name]
    trees[-1].unlink()
    assert stale_rows(tmp_path) == []

    trees[-1].symlink_to("missing-retired-target")
    assert stale_rows(tmp_path) == [deleted_name]
    trees[-1].unlink()

    refusal_name = next(
        name for name in source["components"] if not inventory[name]["deleted_trees"]
    )
    refusal_component = source["components"].pop(refusal_name)
    scan.write_text(json.dumps(source), encoding="utf-8")
    assert stale_rows(tmp_path) == []

    added_name = f"{deleted_name}-new"
    assert added_name not in inventory
    source["components"][added_name] = deleted_component
    scan.write_text(json.dumps(source), encoding="utf-8")
    assert stale_rows(tmp_path) == [added_name]

    source["components"].pop(added_name)
    source["components"][deleted_name] = deleted_component
    source["components"][refusal_name] = refusal_component
    scan.write_text(json.dumps(source), encoding="utf-8")
    assert stale_rows(tmp_path) == []
