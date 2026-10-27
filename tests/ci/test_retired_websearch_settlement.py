from retired_tests import assert_symbols_unreferenced


def test_websearch_settlement_retired():
    assert_symbols_unreferenced(
        ["checkpoint-settlement.json"], ["tests/ci/websearch-context.py"]
    )
