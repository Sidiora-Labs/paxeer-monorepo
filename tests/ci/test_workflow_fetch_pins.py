from pathlib import Path
import re
import shlex

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
FORGE_STD = (
    "https://github.com/foundry-rs/forge-std",
    "v1.9.6",
    "3b20d60d14b343ee4f908cb8079495c07f5e8981",
)
OPENZEPPELIN = (
    "https://github.com/OpenZeppelin/openzeppelin-contracts",
    "v5.3.0",
    "e4f70216d759d8e6a64144a9e1f7bbeed78e7079",
)
ROCKSDB = (
    "https://github.com/facebook/rocksdb.git",
    "v8.9.1",
    "49ce8a1064dd1ad89117899839bf136365e49e79",
)
ETH_TESTS = "c67e485ff8b5be9abc8ad15345ec21aa22e290d9"


def _commands(source):
    return [
        shlex.split(line, comments=True)
        for line in source.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


def _workflow_steps(filename):
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows" / filename).read_text(encoding="utf-8")
    )
    return [
        step
        for job in workflow["jobs"].values()
        for step in job["steps"]
    ]


def _assert_checkout(commands, directory, url, sha):
    assert re.fullmatch(r"[0-9a-f]{40}", sha), sha
    _assert_checkout_commands(commands, directory, url, sha)


def _assert_checkout_commands(commands, directory, url, sha):
    git = ["git", "-C", directory]
    expected = [
        ["git", "init", directory],
        git + ["remote", "add", "origin", url],
        git + ["fetch", "--depth", "1", "origin", sha],
        git + ["checkout", "--detach", sha],
    ]
    for command in expected:
        assert commands.count(command) == 1, command
    positions = [commands.index(command) for command in expected]
    assert positions == sorted(positions), expected


@pytest.mark.parametrize(
    "filename, dependencies",
    [
        (
            "bridge-test.yml",
            [
                ("contracts/bridge/evm/lib/forge-std", FORGE_STD),
                ("contracts/bridge/evm/lib/openzeppelin-contracts", OPENZEPPELIN),
            ],
        ),
        ("paxeer-rocksdb-unit-tests.yml", [("rocksdb", ROCKSDB)]),
    ],
)
def test_workflow_clones_pin_commit_sha(filename, dependencies):
    steps = _workflow_steps(filename)
    scripts = [step["run"] for step in steps if "run" in step]
    commands = [command for source in scripts for command in _commands(source)]
    git_commands = [command for command in commands if command[0] == "git"]
    assert len(git_commands) == 4 * len(dependencies), git_commands
    assert all("clone" not in command for command in git_commands)
    for directory, (url, tag, sha) in dependencies:
        matching = [source for source in scripts if url in source]
        assert len(matching) == 1, (filename, url)
        source = matching[0]
        _assert_checkout(_commands(source), directory, url, sha)
        assert f"fetch --depth 1 origin {sha} # {tag}" in source
    if filename == "paxeer-rocksdb-unit-tests.yml":
        cache = [step for step in steps if step.get("id") == "cache-rocksdb"]
        assert len(cache) == 1
        assert cache[0]["with"]["key"] == (
            f"rocksdb-{ROCKSDB[1]}-{ROCKSDB[2]}-${{{{ runner.os }}}}"
        )


@pytest.mark.parametrize(
    "filename, helper",
    [
        ("contracts/bootstrap-libs.sh", "fetch_library"),
        ("scripts/ci/xweb-forge-libs.sh", "fetch_missing_library"),
    ],
)
def test_bootstrap_scripts_pin_commit_sha(filename, helper):
    source = (ROOT / filename).read_text(encoding="utf-8")
    commands = _commands(source)
    variables = dict(re.findall(r"^([A-Z_]+)='([^']+)'", source, re.MULTILINE))
    calls = [command for command in commands if command[0] == helper]
    resolved = [
        [
            variables[argument[1:]] if argument.startswith("$") else argument
            for argument in call[1:]
        ]
        for call in calls
    ]
    assert resolved == [
        ["forge-std", *FORGE_STD],
        ["openzeppelin-contracts", *OPENZEPPELIN],
    ]
    for _, _, _, sha in resolved:
        assert re.fullmatch(r"[0-9a-f]{40}", sha), sha
    for _, tag, sha in (FORGE_STD, OPENZEPPELIN):
        assert re.search(
            rf"^.*'{sha}'.* # {re.escape(tag)}$", source, re.MULTILINE
        ), (filename, tag, sha)
    function = re.search(
        rf"^{helper}\(\) \{{\n(.*?)^\}}$", source, re.MULTILINE | re.DOTALL
    )
    assert function is not None, filename
    body = function[1]
    assert 'local name="$1" url="$2" tag="$3" commit="$4"' in body
    git_commands = [command for command in commands if command[0] == "git"]
    assert all("clone" not in command for command in git_commands)
    _assert_checkout_commands(_commands(body), "$dest", "$url", "$commit")
    assert ["git", "-C", "$dest", "tag", "$tag", "$commit"] in _commands(body)
    if filename == "contracts/bootstrap-libs.sh":
        assert 'current="$(checked_out_commit "$dest")"' in body
        assert '[ "$current" = "$commit" ] && checkout_is_pristine "$dest"' in body
        assert 'git -C "$dir" rev-parse --verify HEAD' in source
        assert 'git -C "$dir" status --porcelain' in source
    else:
        assert 'if [ -e "$dest" ] || [ -L "$dest" ]; then\n        return 0' in body
        verifier = ["exec", "python3", "$root/scripts/ci/xweb_dependency_identity.py"]
        assert commands[-1] == verifier
        assert all(commands.index(call) < commands.index(verifier) for call in calls)


def test_eth_tests_fetch_is_shallow():
    steps = _workflow_steps("paxeer-eth-blocktests.yml")
    scripts = [step["run"] for step in steps if "run" in step]
    matching = [
        source for source in scripts
        if "https://github.com/ethereum/tests.git" in source
    ]
    assert len(matching) == 1
    commands = _commands(matching[0])
    _assert_checkout(
        commands, "ethtests", "https://github.com/ethereum/tests.git", ETH_TESTS
    )
    assert len(commands) == 4, commands
    all_commands = [command for source in scripts for command in _commands(source)]
    git_commands = [command for command in all_commands if command[0] == "git"]
    assert git_commands == commands
    assert all(
        "--unshallow" not in command and "clone" not in command
        for command in git_commands
    )
