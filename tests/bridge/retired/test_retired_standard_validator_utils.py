from kernel_retirement import assert_retired


def test_standard_validator_utils_is_retired():
    assert_retired(["contracts/manager/StandardValidatorUtils.sol"])
