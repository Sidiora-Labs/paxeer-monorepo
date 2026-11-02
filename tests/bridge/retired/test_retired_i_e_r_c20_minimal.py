from kernel_retirement import assert_retired


def test_i_e_r_c20_minimal_is_retired():
    assert_retired(["contracts/interfaces/IERC20Minimal.sol"])
