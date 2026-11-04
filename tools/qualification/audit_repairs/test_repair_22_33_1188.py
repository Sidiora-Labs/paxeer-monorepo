"""Offline source qualification for audit 22.30.55/e2e-docker-context."""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DOCKER = ROOT / "consensus/test/e2e/docker"
RECORD = Path(__file__).with_name("repair_22_33_1188.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def instructions():
    text = (DOCKER / "Dockerfile").read_text(encoding="utf-8")
    result = []
    for line in text.replace("\\\n", " ").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        name, value = line.split(None, 1)
        result.append((name.upper(), value.strip()))
    return result


def values(name):
    return [value for instruction, value in instructions() if instruction == name]


def test_e2e_docker_repository_module_and_toolchain():
    go_version = re.search(r"^go (\S+)$", source("go.mod"), re.M).group(1)
    assert values("FROM") == [f"golang:{go_version}-bookworm"]
    assert values("WORKDIR") == ["/src/paxeer", "/tendermint"]
    copies = [shlex.split(value) for value in values("COPY")]
    assert ["go.mod", "go.sum", "./"] in copies
    assert [".", "."] in copies
    for filename in ("go.mod", "go.sum"):
        assert (ROOT / filename).is_file()
    replacements = re.findall(r"=> (\./[^\s]+)", source("go.mod"))
    assert replacements
    for replacement in replacements:
        directory = replacement.removeprefix("./")
        assert [f"{directory}/go.mod", f"{directory}/go.sum", f"./{directory}/"] in copies
        for filename in ("go.mod", "go.sum"):
            assert (ROOT / directory / filename).is_file()
    steps = instructions()
    module_copy = steps.index(("COPY", "go.mod go.sum ./"))
    download = steps.index(("RUN", "go mod download"))
    source_copy = steps.index(("COPY", ". ."))
    build = next(i for i, (op, arg) in enumerate(steps) if op == "RUN" and "go build" in arg)
    assert module_copy < download < source_copy < build
    for replacement in replacements:
        directory = replacement.removeprefix("./")
        assert steps.index(("COPY", f"{directory}/go.mod {directory}/go.sum ./{directory}/")) < download


def test_e2e_docker_builds_actual_builtin_command_and_backends():
    commands = [shlex.split(value) for value in values("RUN")]
    build = [argv for argv in commands if "build" in argv]
    assert len(build) == 1
    argv = build[0]
    assert argv[:3] == ["CGO_ENABLED=1", "go", "build"]
    assert "-mod=readonly" in argv
    assert set(argv[argv.index("-tags") + 1].split(",")) == {
        "badgerdb", "boltdb", "cleveldb", "rocksdb"
    }
    assert argv[argv.index("-o") + 1] == "/usr/bin/app"
    assert argv[-1] == "./consensus/test/e2e/node"
    main = source(argv[-1][2:] + "/main.go")
    assert "package main" in main and "func main()" in main
    assert 'noderuntime.Run(ctx, configFile, os.Getenv("TMHOME"))' in main
    install = next(argv for argv in commands if "apt-get" in argv)
    assert "libleveldb-dev" in install and "librocksdb-dev" in install
    assert "--no-install-recommends" in install
    assert "make" not in {arg for argv in commands for arg in argv}
    assert "build/tendermint" not in (DOCKER / "Dockerfile").read_text()


def test_e2e_docker_installs_runtime_contract():
    copies = [shlex.split(value) for value in values("COPY")]
    for name in ("entrypoint", "entrypoint-builtin"):
        assert ["--chmod=0755", f"consensus/test/e2e/docker/{name}", f"/usr/bin/{name}"] in copies
        assert (DOCKER / name).is_file()
        assert (DOCKER / name).stat().st_mode & 0o111 == 0o111
    assert [json.loads(value) for value in values("ENTRYPOINT")] == [["/usr/bin/entrypoint"]]
    assert [json.loads(value) for value in values("CMD")] == [["start"]]
    assert values("ENV") == ["TMHOME=/tendermint"]
    assert [json.loads(value) for value in values("VOLUME")] == [["/tendermint"]]
    assert values("STOPSIGNAL") == ["SIGTERM"]
    assert set(values("EXPOSE")[0].split()) == {"26656", "26657", "26660", "6060"}


@pytest.mark.parametrize("name", ["entrypoint", "entrypoint-builtin"])
def test_e2e_docker_entrypoints_are_parsed_and_guarded(name):
    path = DOCKER / name
    parsed = subprocess.run(["bash", "-n", str(path)], capture_output=True, text=True, check=False)
    assert parsed.returncode == 0, parsed.stderr
    text = path.read_text(encoding="utf-8")
    lines = [line.strip() for line in text.splitlines() if line.strip() and not line.startswith("#")]
    assert text.startswith("#!/usr/bin/env bash\n")
    assert lines[0] == "set -euo pipefail"
    assert not re.search(r"\s&\s*$", text, re.M)
    assert not any(shlex.split(line)[0] in {"rm", "sleep"} for line in lines)
    if name == "entrypoint":
        assert lines == ["set -euo pipefail", 'exec /usr/bin/entrypoint-builtin "$@"']
    else:
        assert 'if [[ $# -gt 1 || ( $# -eq 1 && $1 != start ) ]]; then' in lines
        assert 'exit 64' in lines
        home_guard = ': "${TMHOME:?TMHOME must name the generated testnet node directory}"'
        config_path = 'config="${TMHOME}/config/app.toml"'
        readable = 'if [[ ! -r "$config" ]]; then'
        execute = 'exec /usr/bin/app "$config"'
        assert lines.index(home_guard) < lines.index(config_path) < lines.index(readable)
        guard = lines[lines.index(readable):lines.index(execute)]
        assert 'exit 1' in guard and guard[-1] == "fi"
        assert lines[-1] == execute
        assert shlex.split(lines[-1]) == ["exec", "/usr/bin/app", "$config"]


def test_e2e_docker_generated_config_reaches_builtin_consumer():
    setup = source("consensus/test/e2e/runner/setup.go")
    assert 'image: tendermint/e2e-node' in setup
    assert 'entrypoint: /usr/bin/entrypoint-builtin' in setup
    assert 'return mode == e2e.ModeLight || protocol == e2e.ProtocolBuiltin' in setup
    assert 'filepath.Join(nodeDir, "config", "app.toml")' in setup
    assert '"protocol":                  node.Testnet.ABCIProtocol' in setup
    assert 'if node.Testnet.ABCIProtocol != e2e.ProtocolBuiltin {' in setup
    main = source("consensus/test/e2e/node/main.go")
    assert 'run(ctx, os.Args[1])' in main
    assert 'signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)' in main
    config = source("consensus/test/e2e/node/runtime/config.go")
    assert 'toml.DecodeFile(file, &cfg)' in config
    assert 'case cfg.Protocol != builtinProtocol:' in config
    runtime = source("consensus/test/e2e/node/runtime/node.go")
    assert 'const builtinProtocol = "builtin"' in runtime
    assert 'cfg, err := LoadConfig(configFile)' in runtime
    assert 'err = startNode(ctx, cfg, home)' in runtime
    assert 'app.NewApplication(cfg.App())' in runtime
    assert 'n, err := node.New(' in runtime
    assert 'if err := n.Start(ctx); err != nil {' in runtime


def test_e2e_docker_context_request_resolves_real_inputs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    wiring = record["wiring"]["makefile"]
    assert wiring["path"] == "consensus/test/e2e/Makefile"
    assert wiring["owner_domain"] == "15 (fm-cosmos-fork)"
    directory = ROOT / wiring["working_directory"]
    argv = shlex.split(wiring["command"])
    assert argv[:2] == ["docker", "build"]
    assert argv[argv.index("--tag") + 1] == "tendermint/e2e-node"
    assert (directory / argv[-1]).resolve() == ROOT
    assert (directory / argv[argv.index("-f") + 1]).resolve() == DOCKER / "Dockerfile"
    makefile = source(wiring["path"])
    recipe = re.search(r"^docker:\n\t([^\n]+)", makefile, re.M).group(1)
    actual = shlex.split(recipe)
    assert actual[:2] == ["docker", "build"]
    assert actual[actual.index("--tag") + 1] == "tendermint/e2e-node"
    if (directory / actual[-1]).resolve() != ROOT:
        assert "context from ../.. to ../../.." in wiring["request"]
    qualification = record["wiring"]["qualification"]
    assert qualification["path"] == "consensus/test/e2e/runner"
    assert qualification["owner_domain"] == "15 (fm-cosmos-fork)"
    assert "real builtin testnet" in qualification["request"]
    assert "committed blocks" in qualification["request"]


def test_e2e_docker_record_maps_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["e2e-docker-context"]
    assert finding["audit"] == "22.30.55"
    assert finding["file"] == "consensus/test/e2e/docker/Dockerfile"
    assert finding["class"] == "wiring-gap" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {"test_e2e_docker_record_maps_proofs_and_limits"}
    repair = record["repair"]
    assert repair["task"] == "22.33.1188"
    assert repair["requirement"] == "22998.1"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1188.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "No baseline tests were run" in repair["baseline"]
    assert "source-only gate" in repair["review_only"]
    assert "does not claim Docker-stage coverage" in finding["resolution"]
