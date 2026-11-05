from kernel_retirement import assert_retired


def test_canonical_checkpoint_is_retired():
    assert_retired(["contracts/libraries/CanonicalCheckpoint.sol"])
