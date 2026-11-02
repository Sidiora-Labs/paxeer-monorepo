from kernel_retirement import assert_retired


def test_native_publication_verifier_is_retired():
    assert_retired(["contracts/libraries/NativePublicationVerifier.sol"])
