from retired_tests import assert_symbols_unreferenced, build_and_run_c_suite


LXP_LOG_BATCH_HEADER = "LXP_LOG_BATCH_HEADER"


def test_log_batch_header_retired():
    assert_symbols_unreferenced(
        [LXP_LOG_BATCH_HEADER], ["tests/storage/lxp_test_recovery.c"]
    )
    assert_symbols_unreferenced(
        [LXP_LOG_BATCH_HEADER], ["tests/storage/lxp_test_log_durability.c"]
    )
    build_and_run_c_suite("build/tests/lxp_test_recovery")
    build_and_run_c_suite("build/tests/lxp_test_log_durability")
