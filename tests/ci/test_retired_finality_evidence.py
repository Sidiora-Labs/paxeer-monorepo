from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_finality_evidence_retired():
    assert_paths_absent(["tests/storage/lxp_test_finality_evidence.c"])
    assert_symbols_unreferenced(
        ["lxp_da_log_store_body"],
        ["tests"],
        exclude=[
            "tests/arith",
            "tests/codec",
            "tests/crypto",
            "tests/fuzz",
            "tests/chain",
            "tests/daemon",
            "tests/bridge",
            "tests/explorer",
        ],
    )
