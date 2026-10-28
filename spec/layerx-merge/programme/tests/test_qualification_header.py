from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]


def test_header_names_v2_ids_and_keys() -> None:
    entry_keys = load(ROOT / "spec/workflow.kvx")["qualification_log"]["entry_keys"]
    assert isinstance(entry_keys, list) and entry_keys

    with (ROOT / "spec/layerx-merge/qualification.kvx").open(
        encoding="utf-8"
    ) as ledger:
        id_header = ledger.readline()
        keys_header = ledger.readline()

    assert "<N>.<k>.<m>" in id_header, (
        "The qualification ledger's first line must name v2 task ids <N>.<k>.<m>"
    )
    _, separator, key_names = keys_header.partition(":")
    assert separator, "The qualification ledger's second line must list entry keys"
    header_keys = [
        key.strip() for key in key_names.strip().removesuffix(".").split(",")
    ]
    assert header_keys == entry_keys, (
        "The qualification ledger's second line must list workflow entry_keys "
        f"in order: expected {entry_keys}, found {header_keys}"
    )
