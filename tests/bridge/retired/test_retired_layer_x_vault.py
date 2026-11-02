from kernel_retirement import assert_retired


def test_layer_x_vault_is_retired():
    assert_retired(["contracts/custody/LayerXVault.sol"])
