from pathlib import Path

import pytest

from programme.decisions import load_index, records_missing
from programme.kvx import KvxError, load

ROOT = Path(__file__).resolve().parents[4]


def test_index_covers_every_carried_id(tmp_path: Path) -> None:
    index = load_index()
    expected = set(range(1, 95)) | set(range(1701, 1721))
    assert set(index) == expected
    assert all(
        entry["carry"] in {"kept", "rewritten", "superseded"}
        for entry in index.values()
    )

    spec = load(ROOT / "spec/layerx-merge/spec.kvx")
    for identifier, entry in index.items():
        name = f"decision.{identifier}"
        carry = spec[name]["carry"]
        assert entry["carry"] == ("kept" if carry == "new" else carry)
        assert entry["record"] == f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        assert entry["binds"]
        assert len(entry["binds"]) == len(set(entry["binds"]))
        assert all(f"task.{task}" in spec for task in entry["binds"])
        for section_name, section in spec.items():
            if section_name.startswith("resolution.") and name in section.get(
                "applies_to", []
            ):
                assert set(entry["binds"]) == {
                    target[5:]
                    for target in section["applies_to"]
                    if target.startswith("task.")
                }

    assert records_missing(index, ROOT) == []
    assert records_missing(index, tmp_path) == sorted(expected)
    copied = tmp_path / index[1]["record"]
    copied.parent.mkdir(parents=True)
    copied.write_bytes((ROOT / index[1]["record"]).read_bytes())
    directory = tmp_path / index[2]["record"]
    directory.mkdir()
    assert records_missing(index, tmp_path) == sorted(expected - {1})
    assert records_missing({94: index[94], 2: index[2]}, tmp_path) == [2, 94]
    assert records_missing({}, tmp_path) == []

    invalid = tmp_path / "invalid.kvx"
    invalid.write_text(
        '[meta]\nformat_version = 1\n\n[decision.1]\n'
        'carry = "new"\nrecord = "D1.md"\nbinds = ["17.8.1"]\n',
        encoding="utf-8",
    )
    with pytest.raises(KvxError, match="invalid carry status"):
        load_index(invalid)


def test_exchange_v2_decisions_are_rewritten() -> None:
    index = load_index()
    rewritten = {1, 2, 6, 8, 25, 30, 32, 39, 62, 73}
    assert {identifier: index[identifier]["carry"] for identifier in rewritten} == {
        identifier: "rewritten" for identifier in rewritten
    }
