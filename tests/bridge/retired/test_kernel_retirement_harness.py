import subprocess
from pathlib import Path

import pytest

from kernel_retirement import RETIRED, assert_retired


def _repository(root: Path, files: dict[str, str]) -> Path:
    subprocess.run(["git", "init", "-q", str(root)], check=True, capture_output=True)
    for name, text in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    subprocess.run(["git", "add", "--all"], cwd=root, check=True, capture_output=True)
    return root


def test_harness_reports_a_tracked_retired_file(tmp_path):
    paths = ["contracts/CheckpointRegistry.sol", "contracts/EmergencyExit.sol"]
    files = {path: "pragma solidity ^0.8.24;\n" for path in paths}
    files["scripts/check.py"] = "".join(f'path = "{path}"\n' for path in paths)
    root = _repository(tmp_path / "repo", files)
    (root / paths[0]).unlink()

    with pytest.raises(AssertionError) as error:
        assert_retired(paths, root=root)

    assert "Tracked retired files:" in str(error.value)
    for path in paths:
        assert path in str(error.value)
    assert "Live references:" in str(error.value)
    for line, path in enumerate(paths, 1):
        assert f"scripts/check.py:{line}: {path}" in str(error.value)


@pytest.mark.parametrize(
    ("target", "referrer", "content", "line"),
    [
        (
            "contracts/CheckpointRegistry.sol",
            "scripts/PaxeerBetaDeploy.s.sol",
            "// SPDX-License-Identifier: Apache-2.0\npragma solidity ^0.8.24;\n\n"
            'import {CheckpointRegistry} from "../contracts/CheckpointRegistry.sol";\n',
            4,
        ),
        (
            "contracts/config/checkpoint-settlement.json",
            "services/agent/crates/layerx-proof/src/settlement.rs",
            "const SETTLEMENT: &str =\n"
            '    include_str!("../../../../../contracts/config/checkpoint-settlement.json");\n',
            2,
        ),
        (
            "contracts/security/Governed.sol",
            "contracts/live/Live.sol",
            'import "../security/./Governed.sol";\n',
            1,
        ),
        (
            "contracts/storage/WithdrawalNullifierRegistry.sol",
            "contracts/live/check.c",
            '#include <../storage/WithdrawalNullifierRegistry.sol>\n',
            1,
        ),
        (
            "contracts/CheckpointRegistry.sol",
            "scripts/check.py",
            'path = "contracts/CheckpointRegistry.sol"\n',
            1,
        ),
        (
            "contracts/CheckpointRegistry.sol",
            "contracts/security/ReentrancyLock.sol",
            'import "../CheckpointRegistry.sol";\n',
            1,
        ),
    ],
)
def test_harness_reports_a_live_import_outside_the_retired_set(
    tmp_path, target, referrer, content, line
):
    other_referrer = "scripts/other_reference.py"
    root = _repository(
        tmp_path / "repo",
        {referrer: content, other_referrer: f'path = "{target}"\n'},
    )
    assert referrer not in RETIRED
    assert not (root / target).exists()

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    message = str(error.value)
    assert f"{referrer}:{line}: {target}" in message
    assert f"{other_referrer}:1: {target}" in message


def test_harness_ignores_documentation_references(tmp_path):
    target = "contracts/CheckpointRegistry.sol"
    documents = [
        "docs/nested/reference.txt",
        "spec/nested/retirement.kvx",
        "README.md",
        "contracts/README.md",
        "services/agent/notes.md",
        "GOTCHA.kvx",
    ]
    root = _repository(
        tmp_path / "repo",
        {name: f'Retired path: "{target}"\n' for name in documents},
    )
    untracked = root / "untracked.sol"
    untracked.write_text(f'import "{target}";\n', encoding="utf-8")

    assert_retired([target], root=root)

    for referrer in (
        "contracts/EmergencyExit.sol",
        "contracts/security/DelegateCallAware.sol",
        "contracts/security/Governed.sol",
        "contracts/security/LayerXComponent.sol",
        "contracts/security/UUPSNotUpgradeable.sol",
        "contracts/storage/WithdrawalNullifierRegistry.sol",
        "tests/bridge/custody_credit.py",
        "tests/bridge/retired/kernel_retirement.py",
        "tests/bridge/retired/test_retired_checkpoint_registry.py",
    ):
        assert referrer in RETIRED
        source = root / referrer
        source.parent.mkdir(parents=True, exist_ok=True)
        source.write_text(f'import "{target}";\n', encoding="utf-8")
    subprocess.run(
        ["git", "add", "contracts", "tests"],
        cwd=root,
        check=True,
        capture_output=True,
    )

    assert_retired([target], root=root)
