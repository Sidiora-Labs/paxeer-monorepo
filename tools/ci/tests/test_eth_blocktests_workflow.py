from pathlib import Path

import yaml


def test_blocktest_job_builds_paxd_before_running():
    root = Path(__file__).resolve().parents[3]
    with (root / ".github/workflows/paxeer-eth-blocktests.yml").open(
        encoding="utf-8"
    ) as handle:
        workflow = yaml.safe_load(handle)

    steps = workflow["jobs"]["eth-blocktests"]["steps"]
    names = [step.get("name") for step in steps]
    blocktest_index = names.index("Run ETH Blocktest")
    previous_index = names.index("Set up Go")
    expected_steps = (
        (
            "Install native dependencies",
            "sudo apt-get update && sudo apt-get install --yes build-essential libssl-dev",
        ),
        (
            "Install pinned Rust toolchain",
            "rustup toolchain install 1.91.1 --profile minimal",
        ),
        ("Build native libraries", "make paxd-native-libs"),
        ("Install paxd", "make -f chain.mk install"),
    )
    for name, command in expected_steps:
        index = names.index(name)
        assert steps[index]["run"] == command
        assert previous_index < index < blocktest_index, name
        previous_index = index
