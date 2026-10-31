from kernel_retirement import assert_retired


def test_reserve_reconciler_is_retired():
    assert_retired(["contracts/custody/ReserveReconciler.sol"])
