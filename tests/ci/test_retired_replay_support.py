from retired_tests import (
    assert_paths_absent,
    assert_symbols_unreferenced,
)


def test_replay_support_retired():
    assert_paths_absent(
        [
            "tests/support/lxp_pay1_replay.h",
            "tests/support/lxp_real_replay.h",
            "tests/test_tools.c",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lxp_guarantor_recompute_roots",
            "lxp_replay_checkpoint",
            "lxp_replay_checkpoint_create",
            "lxp_replay_checkpoint_restore",
            "lxp_replay_checkpoint_destroy",
            "lxp_replay_engine_bind_transaction",
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
    assert_symbols_unreferenced(
        [
            "lxp_real_replay.h",
            "lxp_batch_body",
            "lxp_da_bundle",
            "lxp_checkpoint_certificate",
            "lxp_guarantor_ctx",
            "lxp_guarantor_attestation",
            "lxp_guarantor_key_record",
            "lxp_guarantor_cert",
        ],
        ["tests/test_tools.c"],
    )
