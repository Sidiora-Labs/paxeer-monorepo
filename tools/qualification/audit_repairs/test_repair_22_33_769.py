"""Offline source-fact proof for rpc-root-permissions; no image build or node run."""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[3]
RPC = ROOT / "docker/rpcnode"
RECORD = Path(__file__).with_name("repair_22_33_769.kvx")


def docker_instructions():
    source = (RPC / "Dockerfile").read_text(encoding="utf-8")
    logical = re.sub(r"\\\n\s*", " ", source)
    instructions = []
    for line in logical.splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            key, value = line.split(None, 1)
            instructions.append((key.upper(), value))
    return instructions


def run_commands():
    return [
        shlex.split(command.strip())
        for key, value in docker_instructions()
        if key == "RUN"
        for command in value.split("&&")
    ]


def script(name):
    return (RPC / "scripts" / name).read_text(encoding="utf-8")


def test_rpc_root_permissions_dedicated_nonroot_identity():
    instructions = docker_instructions()
    assert [value for key, value in instructions if key == "FROM"] == [
        "golang:1.25.8-bookworm AS go-dist", "ubuntu:latest"
    ]
    assert instructions[-1] == ("USER", "pax:pax")
    assert [value for key, value in instructions if key == "USER"] == ["pax:pax"]
    commands = run_commands()
    assert ["groupadd", "--gid", "10001", "pax"] in commands
    assert [
        "useradd", "--uid", "10001", "--gid", "pax", "--home-dir",
        "/home/pax", "--shell", "/usr/sbin/nologin", "--no-create-home", "pax"
    ] in commands
    env = dict(
        token.split("=", 1)
        for key, value in instructions if key == "ENV"
        for token in shlex.split(value)
    )
    assert env == {
        "HOME": "/home/pax",
        "GOPATH": "/home/pax/go",
        "GOCACHE": "/home/pax/.cache/go-build",
        "PATH": "/home/pax/go/bin:/usr/local/go/bin:$PATH",
    }
    # No other RUN command can reintroduce broad ownership or access grants.
    assert {command[0] for command in commands} == {
        "apt-get", "groupadd", "useradd", "install"
    }
    assert "/root" not in (RPC / "Dockerfile").read_text(encoding="utf-8")


def test_rpc_root_permissions_narrow_owned_state_and_build_directories():
    directories = {}
    for command in run_commands():
        if command[0] != "install":
            continue
        assert command[:2] == ["install", "-d"]
        assert command[2:8:2] == ["-o", "-g", "-m"]
        owner, group, mode = command[3:8:2]
        assert int(mode, 8) & 0o022 == 0
        for directory in command[8:]:
            assert directory not in directories
            directories[directory] = (owner, group, mode)
    private = {
        "/home/pax", "/home/pax/.pax", "/home/pax/.pax/config",
        "/home/pax/.pax/data", "/home/pax/go", "/home/pax/go/bin",
        "/home/pax/go/pkg", "/home/pax/go/pkg/mod", "/home/pax/.cache",
        "/home/pax/.cache/go-build", "/pax-protocol/pax-chain/build",
        "/pax-protocol/pax-chain/build/generated",
        "/pax-protocol/pax-chain/build/generated/logs",
    }
    expected = {path: ("pax", "pax", "0700") for path in private}
    expected.update({
        "/pax-protocol": ("root", "root", "0755"),
        "/pax-protocol/pax-chain": ("root", "root", "0755"),
    })
    assert directories == expected
    # UID 10001 can traverse its private parents; unrelated users cannot.
    for path in private:
        for parent in Path(path).parents:
            if str(parent) in directories:
                owner, group, mode = directories[str(parent)]
                assert int(mode, 8) & (0o100 if owner == "pax" else 0o001)
    assert directories["/home/pax/.pax/config"] == ("pax", "pax", "0700")


def test_rpc_root_permissions_root_owned_executable_launch_scripts():
    instructions = docker_instructions()
    copies = [shlex.split(value) for key, value in instructions if key == "COPY"]
    assert copies == [
        ["--from=go-dist", "/usr/local/go", "/usr/local/go"],
        ["--chmod=0555", "docker/rpcnode/scripts/deploy.sh", "/usr/bin/deploy.sh"],
        ["--chmod=0555", "docker/rpcnode/scripts/step0_build.sh", "/usr/bin/build.sh"],
        ["--chmod=0555", "docker/rpcnode/scripts/step1_configure_init.sh", "/usr/bin/configure_init.sh"],
        ["--chmod=0555", "docker/rpcnode/scripts/step2_start_pax.sh", "/usr/bin/start_pax.sh"],
    ]
    # COPY defaults to root ownership and occurs before the final USER.
    for _, source, _ in copies[1:]:
        path = ROOT / source
        assert path.is_file()
        result = subprocess.run(
            ["/bin/bash", "-n", str(path)], capture_output=True, text=True,
            check=False, timeout=10,
        )
        assert result.returncode == 0, result.stderr
    assert [json.loads(value) for key, value in instructions if key == "CMD"] == [
        ["/usr/bin/deploy.sh"]
    ]
    assert [value for key, value in instructions if key == "WORKDIR"] == [
        "/pax-protocol/pax-chain"
    ]
    assert [value for key, value in instructions if key == "STOPSIGNAL"] == ["SIGTERM"]


def test_rpc_root_permissions_launcher_build_init_start_path():
    deploy = script("deploy.sh")
    lines = [line.strip() for line in deploy.splitlines()
             if line.strip() and not line.startswith("#")]
    assert lines[0] == "umask 077"
    assert lines.count("umask 077") == 1
    assert 'export GOPATH="$HOME/go"' in lines
    assert 'export GOBIN="$GOPATH/bin"' in lines
    assert "export BUILD_PATH=/pax-protocol/pax-chain/build" in lines
    assert 'export PATH="$GOBIN:$PATH:/usr/local/go/bin:$BUILD_PATH"' in lines
    assert 'mkdir -p "$GOBIN"' in lines
    assert re.search(
        r'if \[ -z "\$SKIP_BUILD" \]\s+then\s+/usr/bin/build\.sh\s+fi', deploy
    )
    stages = [
        '/usr/bin/build.sh', 'cp build/paxd "$GOBIN"/',
        '/usr/bin/configure_init.sh', '/usr/bin/start_pax.sh',
    ]
    offsets = [deploy.index(stage) for stage in stages]
    assert offsets == sorted(offsets)
    for path in sorted((RPC / "scripts").glob("*.sh")):
        content = path.read_text(encoding="utf-8")
        assert "/root" not in content
        assert ".bashrc" not in content
        assert not re.search(r"\b(?:sudo|su|chmod|chown|umask)\b", content) or (
            path.name == "deploy.sh" and re.findall(
                r"\b(?:sudo|su|chmod|chown|umask)\b", content
            ) == ["umask"]
        )
    build = script("step0_build.sh")
    assert "make -f chain.mk install" in build
    assert "mkdir -p build/generated" in build
    assert 'echo "DONE" > build/generated/build.complete' in build
    init = script("step1_configure_init.sh")
    assert 'paxd init --chain-id pax "$MONIKER"' in init
    for source, dest in [
        ("docker/rpcnode/config/app.toml", "app.toml"),
        ("docker/rpcnode/config/config.toml", "config.toml"),
        ("build/generated/genesis.json", "genesis.json"),
    ]:
        assert f"cp {source} ~/.pax/config/{dest}" in init
    start = script("step2_start_pax.sh")
    assert 'LOG_DIR="build/generated/logs"' in start
    assert 'paxd start --chain-id pax > "$LOG_DIR/rpc-node.log" 2>&1 &' in start
    assert 'echo "Done" >> build/generated/rpc-launch.complete' in start


def test_rpc_root_permissions_record_maps_proof_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["id"] == "22.33.769"
    assert repair["requirement"] == "22998.1"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_769.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert "No pre-change verification was run" in repair["baseline"]
    assert repair["wiring_requests"] == []
    finding = record["finding"]["rpc-root-permissions"]
    assert finding["audit"] == "22.30.174"
    assert finding["file"] == "docker/rpcnode/Dockerfile"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
