from kernel_retirement import assert_retired


def test_cryptography_primitives_is_retired():
    assert_retired(["contracts/libraries/CryptographyPrimitives.sol"])
