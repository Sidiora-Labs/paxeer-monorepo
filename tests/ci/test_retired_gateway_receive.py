from retired_tests import REPO_ROOT, assert_paths_absent, assert_symbols_unreferenced


def test_gateway_receive_retired():
    assert_paths_absent(["tests/test_gateway_receive.c"])
    excluded_roots = [
        REPO_ROOT / path
        for path in (
            "tests/arith",
            "tests/codec",
            "tests/crypto",
            "tests/fuzz",
            "tests/chain",
            "tests/daemon",
            "tests/bridge",
            "tests/explorer",
        )
    ]
    sources = [
        path.relative_to(REPO_ROOT)
        for path in (REPO_ROOT / "tests").rglob("*")
        if path.is_file()
        and not any(path.is_relative_to(root) for root in excluded_roots)
    ]
    assert_symbols_unreferenced(
        ["lxp_gateway_receive_claim", "lxp_gateway_grant_present_test_locked"],
        sources,
    )
