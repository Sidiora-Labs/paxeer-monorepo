from kernel_retirement import assert_retired


def test_layer_x_custody_is_retired():
    assert_retired(["contracts/LayerXCustody.sol"])
