from retired_tests import assert_paths_absent


def test_batch_wal_retired():
    assert_paths_absent([
        "tests/storage/lxp_test_batch_wal_recovery.c",
        "tests/storage/lxp_test_maintenance_publication.c",
        "tests/storage/check_daemon_recovery_order.sh",
    ])
