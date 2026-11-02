from kernel_retirement import assert_retired


def test_e_o_a_is_retired():
    assert_retired(["contracts/libraries/EOA.sol"])
