from kernel_retirement import assert_retired


def test_guarantor_bond_is_retired():
    assert_retired(["contracts/GuarantorBond.sol"])
