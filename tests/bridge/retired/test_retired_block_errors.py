from kernel_retirement import assert_retired


def test_block_errors_is_retired():
    assert_retired(["contracts/manager/BlockErrors.sol"])
