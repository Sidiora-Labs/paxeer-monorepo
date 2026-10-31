from kernel_retirement import assert_retired


def test_native_withdrawal_proof_is_retired():
    assert_retired(["contracts/config/native-withdrawal-proof.json"])
