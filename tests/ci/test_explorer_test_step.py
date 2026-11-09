import os
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "services/explorer/backend"
BUILDER = ROOT / "services/explorer/deploy/tools/mix-in-builder.sh"


def test_mix_test_excludes_compiler_tag_without_binaries(tmp_path):
    source = BUILDER.read_text(encoding="utf-8")
    match = re.search(
        r"cat <<'CONTAINER_SCRIPT_END'\n(.*?)\nCONTAINER_SCRIPT_END", source, re.S
    )
    assert match, "The builder must select compiler tests inside its container"
    script = match[1]
    selection, setup = script.split(
        'if [ "${MIX_IN_BUILDER_BROWSER_DRIVER:-0}"', 1
    )
    assert "command -v node" in selection
    assert "require.resolve('solc'" in selection
    assert "command -v vyper" in selection
    assert 'exec mix "$@"' in setup
    assert 'sh -c "$CONTAINER_SCRIPT" mix-in-builder "$@"' in source

    # Exercise the real shell selection with no executable tools on PATH.
    # Stop before database setup and mix execution; no command is substituted.
    for arguments, expected in [
        (["test"], ["test", "--exclude", "compiler"]),
        (
            ["test", "apps/explorer/test", "--only", "line:42", "a path with spaces"],
            [
                "test", "apps/explorer/test", "--only", "line:42",
                "a path with spaces", "--exclude", "compiler",
            ],
        ),
        (["compile", "--warnings-as-errors"], ["compile", "--warnings-as-errors"]),
    ]:
        result = subprocess.run(
            ["/bin/sh", "-c", selection + '\nprintf "%s\\0" "$@"',
             "mix-in-builder", *arguments],
            env={**os.environ, "PATH": str(tmp_path)},
            cwd=BACKEND,
            capture_output=True,
            check=True,
        )
        assert result.stdout.decode().split("\0")[:-1] == expected
        if arguments[0] == "test":
            assert b"compiler tools unavailable" in result.stderr
        else:
            assert result.stderr == b""

    contracts = BACKEND / "apps/explorer/test/explorer/smart_contract"
    compiler = (contracts / "solidity/code_compiler_test.exs").read_text()
    assert "doctest Explorer.SmartContract.Solidity.CodeCompiler, tags: [compiler: true]" in compiler
    assert re.search(r'describe "run/2" do\s+@describetag :compiler', compiler)
    # Compiler-independent cases must continue to run in the same modules.
    for name in ["solidity/code_compiler_test.exs", "solidity/verifier_test.exs",
                 "solidity/publisher_test.exs"]:
        text = (contracts / name).read_text()
        assert "@moduletag :compiler" not in text
        assert 'describe "recorded chain ' in text
    vyper = (contracts / "vyper/publisher_test.exs").read_text()
    assert re.search(r'describe "publish/2" do\s+@describetag :compiler', vyper)


def test_explorer_test_step_runs_both_umbrella_apps():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/explorer-images.yml").read_text(encoding="utf-8")
    )
    job = workflow["jobs"]["explorer-backend-test"]
    commands = [
        step
        for step in job["steps"]
        if "run" in step
        and shlex.split(step["run"]) == [
            "services/explorer/deploy/tools/mix-in-builder.sh", "test"
        ]
    ]
    assert len(commands) == 1
    step = commands[0]
    assert shlex.split(step["run"]) == [
        "services/explorer/deploy/tools/mix-in-builder.sh", "test"
    ], "Run the entire umbrella, including explorer and ethereum_jsonrpc"
    assert step["env"]["MIX_ENV"] == "test"
    assert not step.get("continue-on-error", False)
    assert not job.get("continue-on-error", False)
    assert "if" not in step

    umbrella = (BACKEND / "mix.exs").read_text(encoding="utf-8")
    assert 'apps_path: "apps"' in umbrella
    for app in ["explorer", "ethereum_jsonrpc"]:
        manifest = (BACKEND / "apps" / app / "mix.exs").read_text(encoding="utf-8")
        assert f"app: :{app}" in manifest
        assert (BACKEND / "apps" / app / "test/test_helper.exs").is_file()
