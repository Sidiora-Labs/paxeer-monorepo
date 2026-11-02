from kernel_retirement import assert_retired


def test_staticconfig_is_retired():
    assert_retired(["contracts/config/StaticConfig.sol"])
