from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_web_retired():
    assert_paths_absent(
        [
            "tests/modules/test_web_intake.c",
            "tests/network/test_web_adapter.c",
            "tests/sequencer/test_web_root.c",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lx_web_activity_encode",
            "lx_web_adapter_run",
            "lx_web_root_from_availability",
            "lx_web_leaf_encode",
            "lx_web_availability_bundle_build",
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
