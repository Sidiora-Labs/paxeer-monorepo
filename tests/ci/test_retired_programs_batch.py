from retired_tests import (
    assert_paths_absent,
    assert_symbols_unreferenced,
    build_and_run_c_suite,
)


def test_programs_batch_retired():
    assert_paths_absent(
        [
            "tests/programs/test_occupancy_batch.c",
            "tests/programs/test_balance_read.c",
            "tests/programs/test_interface_protocol.c",
        ]
    )
    assert_symbols_unreferenced(
        ["lxp_replay_batch_publication"],
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
    assert_symbols_unreferenced(
        ["lxp_batch_sign"], ["tests/programs/test_call_activity.c"]
    )
    build_and_run_c_suite("build/tests/programs_call_activity")
