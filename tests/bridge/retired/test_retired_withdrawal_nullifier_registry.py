from kernel_retirement import assert_retired


def test_withdrawal_nullifier_registry_is_retired():
    assert_retired(["contracts/storage/WithdrawalNullifierRegistry.sol"])
