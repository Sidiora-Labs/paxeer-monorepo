from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_equivocation_bond_retired():
    assert_paths_absent(
        ["tests/test_equivocation.c", "tests/test_paxeer_bond.c"]
    )
    assert_symbols_unreferenced(
        [
            "lxp_sealed_header_record",
            "lxp_equivocation_kind",
            "lxp_slashing_submit",
            "lxp_paxeer_slash_submit",
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
