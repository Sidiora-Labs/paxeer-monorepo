from retired_tests import assert_symbols_unreferenced, build_and_run_c_suite


def test_qual_faults_retired():
    assert_symbols_unreferenced(
        [
            "lxp_replica",
            "lxp_replica_chain_link",
            "lxp_replica_halt",
            "lxp_sequencer_liveness",
            "lxp_sequencer_loss",
            "lxp_sequencer_can_seal",
            "lxp_sequencer_handover_authorize",
            "lxp_batch_range_check",
        ],
        ["tests/qualification/lxp_qual_faults.c"],
    )
    assert_symbols_unreferenced(
        ["lxp_partition_sim", "lxp_sequencer_loss_sim"],
        ["tests/test_qual_recovery.c"],
    )
    build_and_run_c_suite("build/tests/test_qual_recovery")
