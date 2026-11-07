from kernel_retirement import assert_retired


def test_withdrawal_claims_is_retired():
    assert_retired(["contracts/WithdrawalClaims.sol"])
