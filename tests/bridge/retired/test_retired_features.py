from kernel_retirement import assert_retired


def test_features_is_retired():
    assert_retired(["contracts/config/Features.sol"])
