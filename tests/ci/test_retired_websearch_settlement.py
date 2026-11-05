from pathlib import Path
import runpy

from retired_tests import assert_symbols_unreferenced


def test_websearch_settlement_retired():
    assert_symbols_unreferenced(
        ["checkpoint-settlement.json"], ["tests/ci/websearch-context.py"]
    )


def test_checkpoint_settlement_schema_is_retained():
    root = Path(__file__).resolve().parents[2]
    reader = runpy.run_path(str(root / "tests/bridge/retired/kernel_retirement.py"))
    document = reader["checkpoint_settlement"](root)
    assert next(iter(document)) == "schema"
    assert document["protocol_version"] == 2
