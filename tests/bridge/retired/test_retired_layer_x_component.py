from kernel_retirement import assert_retired


def test_layer_x_component_is_retired():
    assert_retired(["contracts/security/LayerXComponent.sol"])
