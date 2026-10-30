from kernel_retirement import assert_retired


def test_ed25519_is_retired():
    assert_retired(
        ["contracts/crypto/Ed25519.sol", "contracts/config/ed25519-vectors.json"]
    )
