from kernel_retirement import assert_retired


def test_merkle_lib_is_retired():
    assert_retired(["contracts/libraries/MerkleLib.sol"])
