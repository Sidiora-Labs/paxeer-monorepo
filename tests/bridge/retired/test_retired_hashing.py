from kernel_retirement import assert_retired


def test_hashing_is_retired():
    assert_retired(["contracts/libraries/Hashing.sol"])
