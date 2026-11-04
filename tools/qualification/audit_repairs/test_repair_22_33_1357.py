"""Offline source proof of the explicit historical-fixture withdrawal.

Parse the retained production manifest and image recipe, follow their original
node/API/check consumers, and prove those recipes have no active entry point.
No Docker, shell, Go build, environment variable or live endpoint is required.
"""

import hashlib
import json
from pathlib import Path
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
ROSETTA = ROOT / "sdk/contrib/rosetta"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_1357.kvx"


def historical_source(relative, digest):
    path = ROSETTA / relative
    assert path.is_file() and not path.is_symlink()
    assert path.stat().st_mode & 0o111 == 0
    lines = path.read_bytes().splitlines(keepends=True)
    assert lines[0].startswith(b"# Historical example only:")
    assert b"withdrawn (22.33.1357)" in lines[0]
    assert lines[1].startswith(b"# Do not ")
    original = b"".join(lines[2:])
    assert hashlib.sha256(original).hexdigest() == digest
    return original.decode("utf-8")


def compose_example():
    return yaml.safe_load(historical_source(
        "docker-compose.yaml.historical",
        "40a4ab1c4bafdf2c272488e4434834096d05eff9f9ce510e1dd32e84cfda0f88",
    ))


def docker_instructions():
    original = historical_source(
        "node/Dockerfile.historical",
        "c9a95b7ffe543572a427bb313e52646f80518b2679a4bffad8731731ff2c0a43",
    )
    instructions = []
    for line in original.splitlines():
        if line.strip() and not line.lstrip().startswith("#"):
            opcode, argument = line.split(maxsplit=1)
            assert opcode in {"FROM", "RUN", "WORKDIR", "COPY", "ENV"}
            instructions.append((opcode, argument))
    return instructions


def test_rosetta_compose_node_wiring_withdraws_both_active_entry_points():
    # Check explicit Makefile paths and every standard Compose discovery name.
    for name in (
        "compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml",
        "compose.override.yaml", "compose.override.yml",
        "docker-compose.override.yaml", "docker-compose.override.yml",
        "node/Dockerfile", "node/dockerfile", "node/Containerfile",
    ):
        path = ROSETTA / name
        assert not path.exists() and not path.is_symlink(), name
    assert (ROSETTA / "docker-compose.yaml.historical").is_file()
    assert (ROSETTA / "node/Dockerfile.historical").is_file()
    readme = (ROSETTA / "README.md").read_text(encoding="utf-8")
    for statement in (
        "orchestration is **withdrawn**",
        "Docker Compose discovery nor the former explicit paths can launch this fixture",
        "unsupported and fail at the missing Dockerfile",
        "Do not run those targets or pass the historical examples to Docker",
        "not as current operating instructions",
        "does not certify its integration with a maintained node",
    ):
        assert statement in readme, statement
    for relative in ("docker-compose.yaml.historical", "node/Dockerfile.historical"):
        assert f"]({relative})" in readme


def test_rosetta_compose_node_wiring_preserves_historical_transport_and_checks():
    manifest = compose_example()
    assert set(manifest) == {"version", "services"}
    assert manifest["version"] == "3"
    services = manifest["services"]
    assert set(services) == {"cosmos", "rosetta", "faucet", "test_rosetta"}
    node, api, faucet, consumer = (
        services[name] for name in ("cosmos", "rosetta", "faucet", "test_rosetta")
    )
    assert node["image"] == api["image"] == faucet["image"] == "rosetta-ci:latest"
    assert node["command"] == [
        "simd", "start", "--pruning", "nothing", "--grpc-web.enable", "true",
        "--grpc-web.address", "0.0.0.0:9091",
    ]
    assert node["ports"] == ["9090:9090", "26657:26657"]
    assert node["logging"] == {"driver": "none"}
    assert api["command"][:2] == [node["command"][0], "rosetta"]
    assert api["command"][2:] == [
        "--blockchain", "app", "--network", "network",
        "--tendermint", "cosmos:26657", "--grpc", "cosmos:9090",
        "--addr", ":8080",
    ]
    assert api["ports"] == ["8080:8080"]
    assert faucet == {
        "image": "rosetta-ci:latest", "working_dir": "/rosetta",
        "command": ["python3", "faucet.py"], "expose": [8080],
    }
    assert consumer == {
        "image": "tendermintdev/rosetta-cli:v0.6.7",
        "volumes": ["./configuration:/rosetta/config:z"],
        "command": ["./config/run_tests.sh"], "working_dir": "/rosetta",
    }
    # Trace the archived consumer to the retained real check commands/config.
    runner = (ROSETTA / "configuration/run_tests.sh").read_text(encoding="utf-8")
    commands = [
        shlex.split(line) for line in runner.splitlines()
        if line.startswith("rosetta-cli ")
    ]
    assert commands == [
        ["rosetta-cli", "check:data", "--configuration-file", "./config/rosetta.json"],
        ["rosetta-cli", "check:construction", "--configuration-file", "./config/rosetta.json"],
    ]
    config = json.loads((ROSETTA / "configuration/rosetta.json").read_text())
    assert config["network"] == {"blockchain": "app", "network": "network"}
    assert config["online_url"] == "http://rosetta:8080"


def test_rosetta_compose_node_wiring_preserves_historical_image_producer():
    instructions = docker_instructions()
    assert [args for op, args in instructions if op == "FROM"] == [
        "golang:1.18-alpine as build", "alpine",
    ]
    assert ("WORKDIR", "/simd") in instructions
    assert ("RUN", "go build -o simd ./simapp/simd/") in instructions
    assert ("COPY", "--from=build /simd/simd /bin/simd") in instructions
    assert ("COPY", "./contrib/rosetta/node/data.tar.gz data.tar.gz") in instructions
    assert ("COPY", "./contrib/rosetta/configuration ./") in instructions
    assert ("COPY", "--from=build /node/root /root/") in instructions
    assert instructions[-2:] == [
        ("WORKDIR", "/root/.simapp"), ("RUN", "chmod -R 0777 ./"),
    ]
    # Producer and both archived consumers agree on simd; this remains an
    # unavailable historical source, rather than an unproven rename to paxd.
    services = compose_example()["services"]
    for name in ("cosmos", "rosetta"):
        binary = services[name]["command"][0]
        assert ("COPY", f"--from=build /{binary}/{binary} /bin/{binary}") in instructions
    assert not (ROOT / "sdk/simapp/simd").exists()
    assert (ROOT / "daemon/paxd").is_dir()


def test_rosetta_compose_node_wiring_record_maps_proof_and_external_callers():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["id"] == "22.33.1357"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1357.py"
    )
    assert set(record["finding"]) == {"rosetta-compose-node-wiring"}
    finding = record["finding"]["rosetta-compose-node-wiring"]
    assert finding["resolution"] == "withdraw executable target; retain historical example"
    assert finding["tests"] == [
        "test_rosetta_compose_node_wiring_withdraws_both_active_entry_points",
        "test_rosetta_compose_node_wiring_preserves_historical_transport_and_checks",
        "test_rosetta_compose_node_wiring_preserves_historical_image_producer",
        "test_rosetta_compose_node_wiring_record_maps_proof_and_external_callers",
    ]
    for name in finding["tests"]:
        assert callable(globals()[name]), name
    wiring = record["wiring"]["sdk_makefile"]
    assert wiring["path"] == "sdk/Makefile"
    assert wiring["owner_domain"] == 15
    assert wiring["targets"] == ["test-rosetta", "rosetta-data"]
    makefile = (ROOT / wiring["path"]).read_text(encoding="utf-8")
    for target in wiring["targets"]:
        recipe = makefile.split(f"\n{target}:\n", 1)[1].split("\n\n", 1)[0]
        commands = [line.strip() for line in recipe.splitlines()]
        build = "docker build -t rosetta-ci:latest -f contrib/rosetta/node/Dockerfile ."
        assert commands.count(build) == 1
        assert not (ROOT / "sdk/contrib/rosetta/node/Dockerfile").exists()
        assert all(".historical" not in command for command in commands)
    assert "no baseline failure is claimed" in repair["baseline"]
    assert "not runtime certification" in repair["review_only"]
