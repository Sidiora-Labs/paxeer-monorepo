from kernel_retirement import assert_retired


def test_burn_is_retired():
    assert_retired(["contracts/libraries/Burn.sol"])
