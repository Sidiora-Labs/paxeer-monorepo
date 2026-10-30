from kernel_retirement import assert_retired


def test_error_is_retired():
    assert_retired(["contracts/libraries/Error.sol"])
