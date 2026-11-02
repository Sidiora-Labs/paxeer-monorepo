from kernel_retirement import assert_retired


def test_encoding_is_retired():
    assert_retired(["contracts/libraries/Encoding.sol"])
