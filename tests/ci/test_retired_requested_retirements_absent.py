import re

from retired_tests import REPO_ROOT, assert_paths_absent


SOLIDITY_IMPORT = re.compile(r"\bimport\b[^;]*;")
RETIRED_CONTRACT_PATH = re.compile(r"[\"'][^\"']*contracts/storage/")


def test_relay_archive_image_absent():
    assert_paths_absent(["docker/relay-archive"])


def test_relay_archive_tests_absent():
    assert_paths_absent(["tests/relay-archive"])


def test_security_storage_suites_absent():
    assert_paths_absent(
        [
            "tests/solidity/ContractIntegration.t.sol",
            "tests/solidity/ContractExecutionSafety.t.sol",
            "tests/solidity/CheckpointChallengeManager.t.sol",
            "tests/solidity/ContractInvariants.t.sol",
            "tests/solidity/PaxeerBetaDeploy.t.sol",
        ]
    )
    references = [
        f"{path.relative_to(REPO_ROOT)}: {statement.group().strip()}"
        for path in sorted((REPO_ROOT / "tests/solidity").rglob("*.sol"))
        for statement in SOLIDITY_IMPORT.finditer(path.read_text(encoding="utf-8"))
        if RETIRED_CONTRACT_PATH.search(statement.group())
    ]
    assert not references, "Retired Solidity imports remain:\n" + "\n".join(references)


def test_nullifier_registry_suites_absent():
    assert_paths_absent(
        [
            "tests/solidity/ContractInvariants.t.sol",
            "tests/solidity/PaxeerBetaDeploy.t.sol",
        ]
    )


def test_beta_deploy_script_absent():
    assert_paths_absent(["scripts/PaxeerBetaDeploy.s.sol"])
    references = [
        f"{path.relative_to(REPO_ROOT)}: {statement.group().strip()}"
        for path in sorted((REPO_ROOT / "scripts").rglob("*.sol"))
        for statement in SOLIDITY_IMPORT.finditer(path.read_text(encoding="utf-8"))
        if re.search(r"\bWithdrawalNullifierRegistry\b", statement.group())
    ]
    assert not references, "Nullifier registry script imports remain:\n" + "\n".join(
        references
    )


def test_state_surface_names_no_nullifier_registry():
    path = REPO_ROOT / "scripts/ci/solidity-state-surface.sh"
    assert "WithdrawalNullifierRegistry" not in path.read_text(encoding="utf-8"), (
        "scripts/ci/solidity-state-surface.sh still names WithdrawalNullifierRegistry"
    )
