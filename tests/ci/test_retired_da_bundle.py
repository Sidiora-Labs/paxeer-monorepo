from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_da_bundle_retired():
    assert_paths_absent([
        "tests/test_da_bundle.c",
        "tests/test_da_possession.c",
    ])
    assert_symbols_unreferenced(
        [
            "LXP_DA_CLASS_COUNT",
            "lxp_da_chunk",
            "lxp_da_module_root",
            "lxp_da_recovery_input",
            "lxp_da_recovery_metadata_encode",
            "lxp_da_possession_verify",
        ],
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
