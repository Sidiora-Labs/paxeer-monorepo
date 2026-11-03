from kernel_retirement import assert_retired


def test_checkpoint_challenge_manager_is_retired():
    assert_retired(["contracts/challenge/CheckpointChallengeManager.sol"])
