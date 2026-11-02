from kernel_retirement import assert_retired


def test_blueprint_is_retired():
    assert_retired(["contracts/deployment/Blueprint.sol"])
