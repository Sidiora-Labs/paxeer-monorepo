from kernel_retirement import assert_retired


def test_storage_is_retired():
    assert_retired(["contracts/libraries/Storage.sol"])
