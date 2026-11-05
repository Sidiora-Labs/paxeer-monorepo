from retired_tests import REPO_ROOT, assert_paths_absent


def test_deployment_and_governance_suites_retired():
    assert_paths_absent([
        "tests/solidity/ContractDeployment.t.sol",
        "tests/solidity/ContractManager.t.sol",
        "tests/solidity/ContractInvariants.t.sol",
        "tests/solidity/DepositAuthorityTimelock.t.sol",
        "tests/solidity/LayerXTimelock.t.sol",
        "tests/solidity/PaxeerBetaDeploy.t.sol",
        "tests/solidity/PaxeerBetaDeploymentValidator.t.sol",
        "tests/solidity/SidioraProxyGovernance.t.sol",
    ])
    remaining = sorted((REPO_ROOT / "tests/solidity").glob("*.sol"))
    assert not remaining, "Solidity suites remain:\n" + "\n".join(
        str(path.relative_to(REPO_ROOT)) for path in remaining
    )
