from kernel_retirement import assert_retired


def test_i_guarantor_eligibility_is_retired():
    assert_retired(["contracts/interfaces/IGuarantorEligibility.sol"])
