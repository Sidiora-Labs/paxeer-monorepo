from kernel_retirement import assert_retired


def test_caller_checker_is_retired():
    assert_retired(["contracts/libraries/CallerChecker.sol"])
