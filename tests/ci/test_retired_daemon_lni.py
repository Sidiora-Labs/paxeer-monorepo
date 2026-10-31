from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_daemon_lni_retired():
    assert_paths_absent(["tests/test_daemon_lni_admission.c"])
    assert_symbols_unreferenced(
        ["lxp_daemon_lni_internal.h"], ["tests/programs/test_blob_admission.c"]
    )
    assert_symbols_unreferenced(
        ["lxp_daemon_lni_internal.h"], ["tests/programs/test_paxai_native_gaps.c"]
    )
    assert_symbols_unreferenced(
        ["lxp_daemon_lni_internal.h"],
        ["tests/programs/test_paxai_storage_continuity.c"],
    )
