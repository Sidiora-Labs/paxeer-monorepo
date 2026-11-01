from typing import Final

from retired_tests import (
    REPO_ROOT,
    assert_paths_absent,
    assert_symbols_unreferenced as _assert_symbols_unreferenced,
)


lxp_da_failure_evidence: Final[str] = "lxp_da_failure_evidence"
lxp_da_challenge: Final[str] = "lxp_da_challenge"
lxp_da_challenge_response: Final[str] = "lxp_da_challenge_response"
lxp_da_challenge_registry: Final[str] = "lxp_da_challenge_registry"
lxp_da_challenge_indices: Final[str] = "lxp_da_challenge_indices"
lxp_da_challenge_issue: Final[str] = "lxp_da_challenge_issue"


def assert_symbols_unreferenced(symbols, roots, *, exclude):
    excluded = [REPO_ROOT / directory for directory in exclude]
    paths = [
        path
        for search_root in roots
        for path in (REPO_ROOT / search_root).rglob("*")
        if path.is_file()
        and not any(path.is_relative_to(directory) for directory in excluded)
    ]
    _assert_symbols_unreferenced(symbols, paths)


def test_da_challenge_retired():
    assert_paths_absent([
        "tests/test_da_challenge.c",
        "tests/test_da_unavailable.c",
    ])
    assert_symbols_unreferenced(
        [
            lxp_da_failure_evidence,
            lxp_da_challenge,
            lxp_da_challenge_response,
            lxp_da_challenge_registry,
            lxp_da_challenge_indices,
            lxp_da_challenge_issue,
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
