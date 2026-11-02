from kernel_retirement import assert_retired


def test_layer_x_timelock_is_retired():
    assert_retired(["contracts/governance/LayerXTimelock.sol"])
