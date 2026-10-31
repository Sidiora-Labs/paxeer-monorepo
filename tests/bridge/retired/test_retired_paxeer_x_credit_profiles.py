from kernel_retirement import assert_retired


def test_paxeer_x_credit_profiles_is_retired():
    assert_retired(
        [
            "tests/bridge/paxeer_x_advanced_trust_credit.py",
            "tests/bridge/paxeer_x_multiasset_credit.py",
        ]
    )
