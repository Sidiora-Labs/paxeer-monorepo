from kernel_retirement import assert_retired


def test_transient_context_is_retired():
    assert_retired(["contracts/libraries/TransientContext.sol"])
