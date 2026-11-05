from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_paxeer_anchor_retired():
    assert_paths_absent(
        ["tests/paxeer/test_anchor_submit.c", "tests/paxeer/lxp_da_withhold.sh"]
    )
    assert_symbols_unreferenced(
        [
            "lxp_paxeer_abi_selector",
            "lxp_paxeer_checkpoint_certificate_encode",
            "lxp_checkpoint_submit_calldata",
            "lxp_paxeer_abi_encode_bytes",
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
