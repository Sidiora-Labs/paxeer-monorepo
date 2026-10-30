from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_guarantor_bond_custody_retired():
    assert_paths_absent(
        ["tests/test_guarantor_bond.c", "tests/test_paxeer_custody.c"]
    )
    assert_symbols_unreferenced(
        [
            "lxp_paxeer_custody_abi",
            "lxp_checkpoint_registry_state",
            "lxp_checkpoint_registration",
            "lxp_paxeer_guarantor_attestation",
            "lxp_paxeer_custody_abi_init",
            "LXP_PAXEER_CUSTODY_INPUT_COUNT",
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
