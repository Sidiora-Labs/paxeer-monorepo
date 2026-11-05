from kernel_retirement import assert_retired


def test_predeploys_is_retired():
    assert_retired(["contracts/deployment/Predeploys.sol"])
