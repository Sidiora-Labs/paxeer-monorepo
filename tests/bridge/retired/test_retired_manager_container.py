from kernel_retirement import assert_retired


def test_manager_container_is_retired():
    assert_retired(["contracts/manager/ManagerContainer.sol"])
