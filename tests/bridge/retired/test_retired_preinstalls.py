from kernel_retirement import assert_retired


def test_preinstalls_is_retired():
    assert_retired(["contracts/deployment/Preinstalls.sol"])
