from pathlib import Path

import yaml


def test_wallet_go_workflow_triggers_on_every_test_input():
    root = Path(__file__).resolve().parents[3]
    with (root / ".github/workflows/wallet-go-test.yml").open(
        encoding="utf-8"
    ) as handle:
        workflow = yaml.safe_load(handle)

    triggers = workflow[True]
    pull_request_paths = triggers["pull_request"]["paths"]
    push_paths = triggers["push"]["paths"]
    assert pull_request_paths == push_paths

    expected_filters = {
        "services/wallet/attestor/**",
        "services/wallet/ceremony/**",
        "services/wallet/schema/attestor-api/**",
        "services/wallet/deploy/attestor-policy.json",
        "services/wallet/deploy/attestor-kernel-policy.json",
        "services/agent/crates/layerx-wire/tests/support/wallet_vectors/**",
        "layerxproof/**",
        "go.mod",
        "go.sum",
        "precompiles/**/abi.json",
        "precompiles/perps/abi/abi.json",
        "precompiles/common/erc20_abi.json",
        ".github/workflows/wallet-go-test.yml",
    }
    for paths in (pull_request_paths, push_paths):
        assert expected_filters <= set(paths)
