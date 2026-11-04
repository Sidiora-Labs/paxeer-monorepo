"""Offline source proofs for audit 22.30.59 findings 18 and 19.

The alternate image is retired. Inspect the real root generation wiring without
running Docker, Go, Buf, shell subprocesses or network requests.
"""

import ast
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
HELPER = ROOT / "wasm/contrib/prototools-docker"
RECORD = Path(__file__).with_name("repair_22_33_1232.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def docker_instructions():
    """Parse logical Dockerfile instructions, including continued RUN lines."""
    text = (HELPER / "Dockerfile").read_text(encoding="utf-8")
    logical = re.sub(r"\\\n", " ", text)
    instructions = []
    for line in logical.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        name, argument = line.split(None, 1)
        assert name in {"FROM", "ARG", "RUN", "WORKDIR", "ENV", "COPY", "LABEL"}
        instructions.append((name, shlex.split(argument)))
    return instructions


def script_commands(command):
    text = source("scripts/protoc.sh").replace("\\\n", " ")
    return [
        shlex.split(line, comments=True)
        for line in text.splitlines()
        if line.startswith(command + " ")
    ]


def test_finding_18_dockerfile_explicitly_retires_the_alternate_image():
    text = (HELPER / "Dockerfile").read_text(encoding="utf-8")
    assert text.startswith(
        "# Historical upstream image, retired from supported repository generation.\n"
        "# Run bash scripts/protoc.sh from the repository root; see README.md.\n"
    )
    instructions = docker_instructions()
    stages = [i for i, (name, _) in enumerate(instructions) if name == "FROM"]
    assert len(stages) == 2
    final = instructions[stages[-1]:]
    labels = {}
    for name, arguments in final:
        if name == "LABEL":
            for argument in arguments:
                key, value = argument.split("=", 1)
                assert key not in labels
                labels[key] = value
    assert labels["org.opencontainers.image.description"] == (
        "Historical upstream protobuf tools; retired from supported generation. "
        "Use bash scripts/protoc.sh from the repository root."
    )
    # Historical provenance remains explicit, not silently presented as v2 tooling.
    assert instructions[stages[0]] == ("FROM", ["golang:1.16.8-alpine3.13", "AS", "build"])
    assert final[0] == ("FROM", ["golang:1.17.3-alpine"])
    arguments = dict(
        argument.split("=", 1)
        for name, values in instructions if name == "ARG"
        for argument in values
    )
    assert arguments["BUF_VERSION"] == "0.30.0"
    assert arguments["REGEN_GOGOPROTO_VERSION"] == "0.3.0"


def test_finding_19_readme_has_one_supported_root_generation_recipe():
    readme = (HELPER / "README.md").read_text(encoding="utf-8")
    assert "retired from supported repository generation" in readme
    assert "retained only as a historical upstream reference" in readme
    assert "From the repository root" in readme
    assert "There are no supported build or run instructions for this retired image." in readme
    examples = re.findall(r"```([^\n]*)\n(.*?)\n```", readme, re.S)
    assert len(examples) == 1
    language, command = examples[0]
    assert language == "sh"
    assert shlex.split(command, comments=True) == ["bash", "scripts/protoc.sh"]
    assert (ROOT / shlex.split(command)[1]).is_file()
    for stale in (
        "docker build", "docker run", "cosmwasm/prototool-docker",
        "cosmwasm/prototools-docker", "/workspace/cosmos_sdk_dir", "--env",
    ):
        assert stale not in readme, stale
    assert "no external\nCosmos SDK checkout or `COSMOS_SDK_DIR` mount is needed" in readme
    local_links = {
        link for link in re.findall(r"\[[^\]]+\]\(([^)]+)\)", readme)
        if not link.startswith("https://")
    }
    assert {"Dockerfile", "../../../scripts/protoc.sh", "../../../buf.yaml",
            "../../../buf.gen.yaml", "../../../go.mod", "../../../sdk/proto",
            "../../../sdk/third_party/proto"} <= local_links
    for link in local_links:
        resolved = (HELPER / link).resolve()
        assert resolved.is_relative_to(ROOT)
        assert resolved.exists(), link


def test_findings_18_19_root_toolchain_produces_the_documented_generator():
    readme = (HELPER / "README.md").read_text(encoding="utf-8")
    go_version = re.findall(r"^go (\S+)$", source("go.mod"), re.M)
    assert go_version == ["1.25.8"]
    assert f"Go `{go_version[0]}`" in readme
    assert "v0.3.1" in readme and "Buf v1.58.0" in readme
    script = source("scripts/protoc.sh")
    assert "\nset -euo pipefail\n" in script
    assert script_commands("go") == [
        ["go", "get", "github.com/regen-network/cosmos-proto/protoc-gen-gocosmos@v0.3.1"],
        ["go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0", "generate"],
        ["go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0", "generate",
         "--template", "consensus/internal/buf.gen.yaml"],
        ["go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0", "generate",
         "--template", "consensus/internal/wireguard.buf.gen.yaml"],
    ]
    ordered = [
        "go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos@v0.3.1",
        "mkdir -p ./build/proto/gocosmos",
        'build_out="${PWD}/build/proto/gocosmos"',
        'pushd "$(go env GOMODCACHE)/github.com/regen-network/cosmos-proto@v0.3.1" &&',
        'go build -o "${build_out}/protoc-gen-gocosmos" ./protoc-gen-gocosmos &&',
        "  popd",
        "go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate\n",
    ]
    positions = [script.index(fact) for fact in ordered]
    assert positions == sorted(positions)
    for template in ("consensus/internal/buf.gen.yaml",
                     "consensus/internal/wireguard.buf.gen.yaml"):
        assert yaml.safe_load(source(template))["version"] == "v2"


def test_findings_18_19_root_config_routes_local_sdk_types_and_generated_output():
    workspace = yaml.safe_load(source("buf.yaml"))
    generation = yaml.safe_load(source("buf.gen.yaml"))
    assert workspace["version"] == generation["version"] == "v2"
    modules = {module["path"] for module in workspace["modules"]}
    assert {"api", "sdk/proto", "sdk/third_party/proto", "interchain/proto",
            "consensus/proto", "consensus/internal", "wasm/proto"} <= modules
    for module in modules:
        assert (ROOT / module).is_dir()
    assert {entry["directory"] for entry in generation["inputs"]} == {
        "api", "sdk/proto", "interchain/proto", "consensus/proto", "wasm/proto",
    }
    plugins = generation["plugins"]
    gocosmos = [p for p in plugins if p.get("local") == [
        "./build/proto/gocosmos/protoc-gen-gocosmos",
    ]]
    assert len(gocosmos) == 1
    assert gocosmos[0]["out"] == "./build/proto/gocosmos"
    assert set(gocosmos[0]["opt"]) == {
        "plugins=interfacetype+grpc",
        "Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types",
        "Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types",
        "Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types",
        "Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types",
        "Mgoogle/protobuf/any.proto=github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types",
    }
    readme = (HELPER / "README.md").read_text(encoding="utf-8")
    assert "`build/proto/gocosmos/protoc-gen-gocosmos`" in readme
    assert "`github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types`" in readme
    assert (ROOT / "sdk/codec/types").is_dir()
    prefix = "./build/proto/gocosmos/github.com/Sidiora-Labs/Paxeer-X-Network"
    assert script_commands("cp") == [
        ["cp", "-rf", prefix + "/*", "./"],
        ["cp", "-rf", prefix + "/sdk/*", "./sdk"],
        ["cp", "-rf", prefix + "/wasm/*", "./wasm"],
        ["cp", "-rf", "./build/proto/gogofaster/github.com/Sidiora-Labs/Paxeer-X-Network/consensus/*",
         "./consensus"],
    ]
    script = source("scripts/protoc.sh")
    assert script.rindex("cp -rf ") < script.rindex("rm -rf ./build/proto")
    assert script_commands("rm") == [["rm", "-rf", "./build/proto"]] * 2
    assert "overwrites\ngenerated files" in readme
    assert "update module metadata" in readme


def test_repair_record_maps_both_findings_to_declared_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["meta"]["task"] == "22.33.1232"
    assert record["meta"]["acceptance"] == "22998.1"
    assert record["meta"]["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert set(record["finding"]) == {"18", "19"}
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    mapped = set()
    for finding_id, filename, kind in (
        ("18", "Dockerfile", "config-drift"), ("19", "README.md", "doc-drift"),
    ):
        finding = record["finding"][finding_id]
        assert finding["file"] == f"wasm/contrib/prototools-docker/{filename}"
        assert finding["class"] == kind and finding["severity"] == "medium"
        assert finding["owner_domain"] == 16
        assert finding["tests"] and set(finding["tests"]) <= declared
        mapped.update(finding["tests"])
    assert mapped == declared - {
        "test_repair_record_maps_both_findings_to_declared_proofs_and_evidence",
    }
    assert record["proof"]["wiring_requests"] == []
    assert "not executed" in record["proof"]["review_only"]
    assert "No pre-change verification was run" in record["baseline"]["verification"]
    assert record["qualification"]["build_cmd"] == "python3 --version"
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1232.py"
    )
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in record["qualification"]["evidence"]
