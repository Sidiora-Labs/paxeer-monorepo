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
        (
            "contracts/CheckpointRegistry.sol",
            "contracts/live/reference.py",
            'path = "../CheckpointRegistry.sol"\n',
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


def test_harness_reports_a_relative_reference_without_a_stem_in_the_content(tmp_path):
    target = "contracts/CheckpointRegistry.sol"
    referrer = f"{target}/reference.py"
    root = _repository(tmp_path / "repo", {referrer: 'path = "."\n'})
    assert (root / target).is_dir()
    assert referrer not in RETIRED

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    message = str(error.value)
    assert "Tracked retired files:" not in message
    assert f"{referrer}:1: {target}" in message


@pytest.mark.parametrize(
    "content",
    [
        "import tests.bridge.custody_credit\n",
        "from tests.bridge import custody_credit\n",
        "from tests.bridge import (\n    custody_credit,\n)\n",
        'importlib.import_module("tests.bridge.custody_credit")\n',
        '__import__("tests.bridge.custody_credit")\n',
    ],
)
@pytest.mark.parametrize("referrer", ["tests/bridge/live.py", "services/live.py"])
def test_harness_reports_a_live_import_by_module_name(tmp_path, referrer, content):
    target = "tests/bridge/custody_credit.py"
    root = _repository(tmp_path / "repo", {referrer: "# live consumer\n" + content})

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    line = 3 if content.startswith("from tests.bridge import (\n") else 2
    assert f"{referrer}:{line}: {target}" in str(error.value)


@pytest.mark.parametrize(
    ("referrer", "content"),
    [
        ("tools/check.sh", "python3 tests/bridge/custody_credit.py\n"),
        ("tools/check.sh", "python3 custody_credit.py\n"),
        ("tools/check.sh", "python3 custody_credit.py # live invocation\n"),
        ("tools/check.sh", 'python3 "custody_credit.py" # live invocation\n'),
        ("tools/check.sh", "./custody_credit.py --check\n"),
        ("tests/bridge/check.py", 'module = "custody_credit.py"\n'),
        ("cmd/check.mk", "check:\n\tpython3 custody_credit.py\n"),
        ("tools/Makefile", "check:\n\t./custody_credit.py --check\n"),
    ],
)
def test_harness_reports_a_stem_reference_in_a_script(tmp_path, referrer, content):
    target = "tests/bridge/custody_credit.py"
    root = _repository(tmp_path / "repo", {referrer: content})

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    line = 2 if content.startswith("check:\n") else 1
    assert f"{referrer}:{line}: {target}" in str(error.value)
    assert str(error.value).count(f": {target}") == 1


@pytest.mark.parametrize(
    ("target", "content"),
    [
        ("tests/bridge/sign_credit.py", "python3 sign_credit.py\n"),
        ("tests/bridge/sign_credit.c", "cc sign_credit.c\n"),
        ("tests/bridge/sign_credit.sh", "sh sign_credit.sh\n"),
        ("tests/bridge/sign_credit.c", "import tests.bridge.sign_credit\n"),
        ("tests/bridge/sign_credit.c", "from tests.bridge import sign_credit\n"),
        (
            "tests/bridge/sign_credit.c",
            "from tests.bridge import live, sign_credit as retired\n",
        ),
        (
            "tests/bridge/sign_credit.c",
            'importlib.import_module("tests.bridge.sign_credit")\n',
        ),
    ],
)
def test_harness_reports_common_stems_with_a_suffix_or_module(tmp_path, target, content):
    referrer = "tools/check.py"
    root = _repository(tmp_path / "repo", {referrer: content})

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    assert f"{referrer}:1: {target}" in str(error.value)


@pytest.mark.parametrize(
    ("content", "line"),
    [
        ("from tests.bridge import(sign_credit)\n", 1),
        ("from tests.bridge import (\n    # credit signer\n    sign_credit,\n)\n", 3),
        ("from tests.bridge import \\\n    sign_credit\n", 2),
        (
            "from tests.bridge import (\n"
            "    live as current,  # active module\n"
            "    sign_credit as retired,\n)\n",
            3,
        ),
    ],
)
def test_harness_reports_common_stems_in_continued_imports(tmp_path, content, line):
    target = "tests/bridge/sign_credit.c"
    referrer = "services/live.py"
    root = _repository(tmp_path / "repo", {referrer: content})

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    assert f"{referrer}:{line}: {target}" in str(error.value)


def test_harness_ignores_common_words_and_partial_stems(tmp_path):
    root = _repository(
        tmp_path / "repo",
        {
            "tools/check.py": (
                "# sign_credit signs a credit; Error describes a failure.\n"
                "sign_credit = 1\n"
                "custody_credit_backup = 2\n"
                'name = "prefix_custody_credit"\n'
                "from tests.bridge import live as sign_credit\n"
                "from tests.bridge import (\n"
                "    # sign_credit describes a credit signer.\n"
                "    live,\n)\n"
            ),
            "tools/notes.txt": "custody_credit\n",
            "services/live.py": "custody_credit\n",
        },
    )

    assert_retired(
        [
            "tests/bridge/custody_credit.py",
            "tests/bridge/sign_credit.c",
            "contracts/libraries/Error.sol",
        ],
        root=root,
    )


@pytest.mark.parametrize(
    "referrer",
    [
        "tests/bridge/retired/new_assertion.py",
        "tests/bridge/retired/nested/new_assertion.py",
        "tests/ci/test_retired_new_module.py",
        "tests/ci/test_requested_retirements_absent.py",
        "tests/ci/test_new_absent_modules.py",
        "cmd/retirement/test_new_module_retired.py",
        "cmd/retirement/nested/inventory.py",
    ],
)
def test_harness_ignores_retirement_assertions(tmp_path, referrer):
    target = "tests/bridge/custody_credit.py"
    root = _repository(
        tmp_path / "repo",
        {
            referrer: (
                f'path = "{target}"\n'
                'stem = "custody_credit.py"\n'
                "import tests.bridge.custody_credit\n"
            ),
            "tools/live.py": "import tests.bridge.custody_credit\n",
        },
    )

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    assert f"tools/live.py:1: {target}" in str(error.value)
    assert referrer not in str(error.value)
    (root / "tools/live.py").write_text("pass\n", encoding="utf-8")
    assert_retired([target], root=root)


@pytest.mark.parametrize(
    ("target", "other"),
    [
        ("tests/bridge/owner-custody.py", "owner-custody.json"),
        ("tests/bridge/custody_credit.py", "custody_credit.json"),
        ("tests/bridge/sign_credit.c", "sign_credit.py"),
        ("contracts/crypto/Ed25519.sol", "Ed25519.json"),
        ("contracts/libraries/EOA.sol", "EOA.json"),
    ],
)
def test_harness_ignores_a_stem_with_another_extension(tmp_path, target, other):
    root = _repository(
        tmp_path / "repo", {"tools/check.py": f'path = "{other}"\n'}
    )

    assert_retired([target], root=root)


@pytest.mark.parametrize(
    ("referrer", "content"),
    [
        (
            "tests/bridge/live.py",
            "# import tests.bridge.custody_credit\n"
            "# from tests.bridge import custody_credit\n"
            '# importlib.import_module("tests.bridge.custody_credit")\n'
            '# path = "tests/bridge/custody_credit.py"\n'
            'pass  # custody_credit.py and "contracts/crypto/Ed25519.sol"\n',
        ),
        (
            "tools/check.sh",
            "  # python3 tests/bridge/custody_credit.py\n"
            "true # python3 custody_credit.py\n",
        ),
        (
            "contracts/live/Live.sol",
            '// import "../crypto/Ed25519.sol";\n'
            '/* import "../crypto/Ed25519.sol";\n'
            'import "../crypto/Ed25519.sol"; */\n',
        ),
    ],
)
def test_harness_ignores_comment_references(tmp_path, referrer, content):
    root = _repository(tmp_path / "repo", {referrer: content})

    assert_retired(
        ["tests/bridge/custody_credit.py", "contracts/crypto/Ed25519.sol"],
        root=root,
    )


def test_harness_ignores_prose_and_partial_filenames(tmp_path):
    root = _repository(
        tmp_path / "repo",
        {
            "tools/check.py": (
                'message = "Ed25519 verifies signatures for an EOA"\n'
                'example = "import tests.bridge.custody_credit"\n'
                'example = "from tests.bridge import custody_credit"\n'
                'name = "prefix-custody_credit.py"\n'
                'name = "custody_credit.py.backup"\n'
                'name = "custody_credit.py_backup"\n'
            ),
        },
    )

    assert_retired(
        [
            "tests/bridge/custody_credit.py",
            "contracts/crypto/Ed25519.sol",
            "contracts/libraries/EOA.sol",
        ],
        root=root,
    )


@pytest.mark.parametrize(
    "target", ["contracts/crypto/Ed25519.sol", "contracts/libraries/EOA.sol"]
)
def test_harness_reports_a_stem_with_its_own_extension(tmp_path, target):
    root = _repository(
        tmp_path / "repo", {"tools/check.sh": f"check {Path(target).name}\n"}
    )

    with pytest.raises(AssertionError) as error:
        assert_retired([target], root=root)

    assert f"tools/check.sh:1: {target}" in str(error.value)


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

    module = "tests/bridge/custody_credit.py"
    module_documents = documents + [
        "tools/README.md",
        "tests/bridge/notes.md",
        "cmd/notes.md",
    ]
    for name in module_documents:
        source = root / name
        source.parent.mkdir(parents=True, exist_ok=True)
        source.write_text(
            f'Retired path: "{target}"\n'
            "import tests.bridge.custody_credit\ncustody_credit\n",
            encoding="utf-8",
        )
    retired = root / "tests/bridge/comet_credit.py"
    retired.parent.mkdir(parents=True, exist_ok=True)
    retired.write_text("import tests.bridge.custody_credit\n", encoding="utf-8")
    subprocess.run(
        ["git", "add", *module_documents, "tests/bridge/comet_credit.py"],
        cwd=root,
        check=True,
        capture_output=True,
    )

    assert_retired([module], root=root)

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
