from typing import Final

from retired_tests import (
    REPO_ROOT,
    assert_paths_absent,
    assert_symbols_unreferenced,
)


lxp_guarantor_authority_verdict: Final = "lxp_guarantor_authority_verdict"
lxp_guarantor_process_batch: Final = "lxp_guarantor_process_batch"
lxp_guarantor_dissent_record: Final = "lxp_guarantor_dissent_record"
lxp_guarantor_divergence: Final = "lxp_guarantor_divergence"
lxp_guarantor_first_divergence: Final = "lxp_guarantor_first_divergence"
lxp_guarantor_withhold: Final = "lxp_guarantor_withhold"


def _included_sources(roots, *, exclude):
    excluded = [REPO_ROOT / path for path in exclude]
    return [
        path
        for root in roots
        for path in (REPO_ROOT / root).rglob("*")
        if path.is_file()
        and not any(path == tree or tree in path.parents for tree in excluded)
    ]


def test_guarantor_duties_retired():
    assert_paths_absent(
        ["tests/test_guarantor_duties.c", "tests/test_guarantor_disagreement.c"]
    )
    assert_symbols_unreferenced(
        [
            lxp_guarantor_authority_verdict,
            lxp_guarantor_process_batch,
            lxp_guarantor_dissent_record,
            lxp_guarantor_divergence,
            lxp_guarantor_first_divergence,
            lxp_guarantor_withhold,
        ],
        _included_sources(
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
        ),
    )
