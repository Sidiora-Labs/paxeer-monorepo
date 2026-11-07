from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_batch_publication_retired():
    assert_paths_absent([
        "tests/test_batch_publish.c",
        "tests/test_batch_time.c",
        "tests/test_history_query.c",
    ])
    assert_symbols_unreferenced(
        [
            "lxp_batch_replica_target",
            "lxp_batch_eligibility_state",
            "lxp_batch_body_decode",
            "lxp_batch_publish",
            "lxp_batch_eligibility_init",
            "lxp_batch_eligibility",
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
