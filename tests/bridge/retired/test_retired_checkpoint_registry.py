from kernel_retirement import assert_retired


def test_checkpoint_registry_is_retired():
    assert_retired(["contracts/CheckpointRegistry.sol"])
