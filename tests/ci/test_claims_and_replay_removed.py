import pathlib
import re


ROOT = pathlib.Path(__file__).resolve().parents[2]
C_NON_CODE = re.compile(r'/\*.*?\*/|//[^\n]*|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'', re.S)


def kernel_test_sources():
    paths = sorted(
        path for path in (ROOT / "tests").rglob("*")
        if path.suffix in {".c", ".h"} and path.is_file()
    )
    assert paths, "No kernel test sources found"
    return {
        path.relative_to(ROOT).as_posix(): C_NON_CODE.sub(" ", path.read_text())
        for path in paths
    }


def test_no_claim_or_deposit_root_callers():
    retired_calls = re.compile(
        r"\b(?:lxp_exit_claim_build|lxp_exit_verify_balance_proof|"
        r"lxp_bridge_withdraw_finalize|"
        r"lx_checkpoint_registry_register_deposit_root)\s*\("
    )
    callers = [
        f"{path}: {match.group(0)}"
        for path, source in kernel_test_sources().items()
        for match in retired_calls.finditer(source)
    ]
    assert not callers, "Retired claim callers remain:\n" + "\n".join(callers)


def test_replay_expects_sequence_refusal():
    idempotency = C_NON_CODE.sub(
        " ", (ROOT / "tests/state/lxp_test_idempotency.c").read_text()
    )
    assert not re.search(r"\bLXP_ERR_IDEMPOTENT_REPLAY\b", idempotency), (
        "Retired replay expectations remain in tests/state/lxp_test_idempotency.c"
    )
    transition = C_NON_CODE.sub(
        " ", (ROOT / "tests/test_state_commitment_transition.c").read_text()
    )
    assert re.search(
        r"REQUIRE\s*\(\s*lxp_kernel_execute_activity\s*\(\s*"
        r"&f->kernel\s*,\s*&f->activity\s*,\s*&f->execution\s*,\s*&replay\s*\)"
        r"\s*==\s*LXP_ERR_SEQUENCE_REUSED\s*\)\s*;",
        transition,
    ), "The state commitment replay must assert a sequence refusal"
