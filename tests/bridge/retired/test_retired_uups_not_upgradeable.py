from kernel_retirement import assert_retired


def test_uups_not_upgradeable_is_retired():
    assert_retired(["contracts/security/UUPSNotUpgradeable.sol"])
