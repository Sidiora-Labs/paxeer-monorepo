from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_replica_retired():
    assert_paths_absent(
        [
            "tests/test_replica_ingest.c",
            "tests/test_replica_divergence.c",
            "tests/test_replica_replay.c",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lxp_replica_init",
            "lxp_replica_ingest_batch",
            "lxp_replica_bind_execution",
            "lxp_replica_bind_eligibility",
            "lxp_replica_batch_eligible",
            "lxp_divergence_component",
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
