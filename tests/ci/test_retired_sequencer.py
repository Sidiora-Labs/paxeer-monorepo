from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_sequencer_retired():
    assert_paths_absent(
        [
            "tests/test_sequencer_recovery.c",
            "tests/test_sequencer_sequence.c",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lxp_sequencer_recovery_ops",
            "lxp_sequencer_recovery_result",
            "lxp_sequencer_recover",
            "lxp_sequencer_equivocation_evidence",
            "lxp_sequencer_header_registry",
            "lxp_sequencer_header_registry_init",
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
