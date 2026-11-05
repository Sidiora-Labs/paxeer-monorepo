from kernel_retirement import assert_retired


def test_paxeer_beta_deployment_validator_is_retired():
    assert_retired(["contracts/deployment/PaxeerBetaDeploymentValidator.sol"])
