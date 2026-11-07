from kernel_retirement import assert_retired


def test_native_state_proof_is_retired():
    assert_retired(["contracts/libraries/NativeStateProof.sol"])
