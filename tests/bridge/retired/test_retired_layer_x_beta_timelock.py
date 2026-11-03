from kernel_retirement import assert_retired


def test_layer_x_beta_timelock_is_retired():
    assert_retired(["contracts/governance/LayerXBetaTimelock.sol"])
