from kernel_retirement import assert_retired


def test_i_layer_x_asset_registry_is_retired():
    assert_retired(["contracts/interfaces/ILayerXAssetRegistry.sol"])
