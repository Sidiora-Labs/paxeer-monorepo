from kernel_retirement import assert_retired


def test_bytes_is_retired():
    assert_retired(["contracts/libraries/Bytes.sol"])
