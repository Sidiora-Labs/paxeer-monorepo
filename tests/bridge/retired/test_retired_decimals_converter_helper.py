from kernel_retirement import assert_retired


def test_decimals_converter_helper_is_retired():
    assert_retired(["contracts/libraries/DecimalsConverterHelper.sol"])
