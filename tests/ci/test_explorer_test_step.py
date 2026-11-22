import os
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "services/explorer/backend"
BUILDER = ROOT / "services/explorer/deploy/tools/mix-in-builder.sh"


def test_compiler_inventory_covers_compiler_cases_and_their_fixtures():
    inventory = ROOT / "docker/explorer-elixir-builder/compilers.tsv"
    entries = [line.split("\t") for line in inventory.read_text().splitlines()]
    listed = set()
    for compiler, version, url, digest in entries:
        assert compiler in {"solc", "vyper"}
        assert url.startswith("https://")
        assert re.fullmatch(r"[0-9a-f]{64}", digest)
        assert (compiler, version) not in listed
        listed.add((compiler, version))

    tests = BACKEND / "apps/explorer/test"
    contracts = tests / "explorer/smart_contract"
    factory = (tests / "support/factory.ex").read_text()
    version_pattern = r'"(v\d+\.\d+\.\d+(?:-nightly\.[\d.]+)?(?:\+commit\.[0-9a-f]+)?)"'
    requested = set()
    for case in contracts.rglob("*.exs"):
        source = case.read_text()
        if not re.search(r"@(?:module|describe)?tag\s+:compiler|tags:\s*\[compiler:\s*true", source):
            continue
        sources = [source]
        for name in set(re.findall(r"Factory\.(contract_code_info\w*)\(", source)):
            body = re.search(rf"^  def {name} do\n(.*?)(?=^  def |\Z)", factory, re.M | re.S)
            assert body, name
            sources.append(body[1])
        for fixture in set(re.findall(r"/test/(support/fixture/[^\"\s]+\.json)", source)):
            sources.append((tests / fixture).read_text())
        for text in sources:
            for version in re.findall(version_pattern, text):
                compiler = "solc" if "+commit." in version else "vyper"
                requested.add((compiler, version))

    assert requested
    assert {compiler for compiler, _ in requested} == {"solc", "vyper"}
    assert requested <= listed, sorted(requested - listed)


def test_compiler_inventory_excludes_downloader_cases():
    inventory = ROOT / "docker/explorer-elixir-builder/compilers.tsv"
    listed = {tuple(line.split("\t")[:2]) for line in inventory.read_text().splitlines()}
    source = (
        BACKEND / "apps/explorer/test/repair/contract_reader_and_compiler_download_test.exs"
    ).read_text()
    version_pattern = r'"(v\d+\.\d+\.\d+(?:-nightly\.[\d.]+)?(?:\+commit\.[0-9a-f]+)?)"'
    requested = {
        ("solc" if "+commit." in version else "vyper", version)
        for version in re.findall(version_pattern, source)
    }
    assert {compiler for compiler, _ in requested} == {"solc", "vyper"}
    assert not requested & listed, sorted(requested & listed)


def test_compiler_inventory_covers_vyper_verification_requests():
    inventory = ROOT / "docker/explorer-elixir-builder/compilers.tsv"
    listed = {
        version
        for compiler, version, *_ in (
            line.split("\t") for line in inventory.read_text().splitlines()
        )
        if compiler == "vyper"
    }
    source = (
        BACKEND
        / "apps/block_scout_web/test/block_scout_web/controllers/api/v2/verification_controller_test.exs"
    ).read_text()
    requested = set()
    for section in re.split(r"^\s*describe\s+", source, flags=re.M)[1:]:
        if not section.startswith('"/api/v2/smart-contracts/{address_hash}/verification/via/vyper-'):
            continue
        requested.update(re.findall(r'"compiler_version"\s*=>\s*"(v[^"]+)"', section))
    assert requested
    assert requested <= listed, sorted(requested - listed)


def test_mix_test_requires_compiler_tools(tmp_path):
    source = BUILDER.read_text(encoding="utf-8")
    match = re.search(
        r"cat <<'CONTAINER_SCRIPT_END'\n(.*?)\nCONTAINER_SCRIPT_END", source, re.S
    )
    assert match, "The builder must select compiler tests inside its container"
    script = match[1]
    selection, setup = script[script.index("compiler_binaries_available() {") :].split(
        'if [ "${MIX_IN_BUILDER_BROWSER_DRIVER:-0}"', 1
    )
    assert "command -v node" in selection
    assert "require.resolve('solc'" in selection
    assert "command -v vyper" in selection
    assert setup.strip().endswith('run_mix_with_retry "$@"')
    assert 'sh -c "$CONTAINER_SCRIPT" mix-in-builder "$@"' in source

    # Exercise the real compiler check with no executable tools on PATH.
    # Stop before database setup and mix execution; no command is substituted.
    for arguments, expected in [
        (["test"], []),
        (
            ["test", "apps/explorer/test", "--only", "line:42", "a path with spaces"],
            [],
        ),
        (["compile", "--warnings-as-errors"], ["compile", "--warnings-as-errors"]),
    ]:
        result = subprocess.run(
            ["/bin/sh", "-c", selection + '\nprintf "%s\\0" "$@"',
             "mix-in-builder", *arguments],
            env={**os.environ, "PATH": str(tmp_path)},
            cwd=BACKEND,
            capture_output=True,
            check=False,
        )
        assert result.stdout.decode().split("\0")[:-1] == expected
        if arguments[0] == "test":
            assert result.returncode == 1
            assert result.stdout == b""
            assert result.stderr == (
                b"mix-in-builder: required compiler tools are unavailable\n"
            )
        else:
            assert result.returncode == 0
            assert result.stderr == b""

    contracts = BACKEND / "apps/explorer/test/explorer/smart_contract"
    compiler = (contracts / "solidity/code_compiler_test.exs").read_text()
    assert (
        "doctest Explorer.SmartContract.Solidity.CodeCompiler, "
        "tags: [compiler: true, compiler_doctest: true]"
    ) in compiler
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

    runtime_steps = [
        item for item in job["steps"]
        if item.get("name") == "Ensure the container runtime is available"
    ]
    assert len(runtime_steps) == 1
    runtime = runtime_steps[0]
    assert job["steps"].index(runtime) < job["steps"].index(step)
    assert "docker version" in runtime["run"]
    assert not runtime.get("continue-on-error", False)
    assert "if" not in runtime

    umbrella = (BACKEND / "mix.exs").read_text(encoding="utf-8")
    assert 'apps_path: "apps"' in umbrella
    for app in ["explorer", "ethereum_jsonrpc"]:
        manifest = (BACKEND / "apps" / app / "mix.exs").read_text(encoding="utf-8")
        assert f"app: :{app}" in manifest
        assert (BACKEND / "apps" / app / "test/test_helper.exs").is_file()
