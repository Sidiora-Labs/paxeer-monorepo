from kernel_retirement import assert_retired


def test_safe_transfer_is_retired():
    assert_retired(["contracts/libraries/SafeTransfer.sol"])
