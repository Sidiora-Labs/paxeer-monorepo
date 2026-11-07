from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_paxeer_membership_retired():
    assert_paths_absent(
        [
            "tests/paxeer/membership_sync_driver.c",
            "tests/paxeer/test_membership_sync.c",
            "tests/paxeer/membership-sync-chain.py",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lxp_paxeer_bond_binding_encode",
            "lxp_paxeer_bond_binding",
            "lxp_paxeer_bond_binding_decode",
            "lxp_paxeer_bond_binding_adopt",
            "lxp_paxeer_membership_commitment",
            "lxp_paxeer_bond_deposit_record",
        ],
        ["tests"],
        exclude=[
            "tests/arith",
            "tests/codec",
            "tests/crypto",
            "tests/fuzz",
            "tests/chain",
            "tests/daemon",
            "tests/bridge",
            "tests/explorer",
        ],
    )
