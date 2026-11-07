from kernel_retirement import assert_retired


def test_asset_registry_is_retired():
    assert_retired(["contracts/custody/AssetRegistry.sol"])
