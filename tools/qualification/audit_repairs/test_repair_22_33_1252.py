"""Offline source proof for the two wasm generator configuration findings.

Only bash syntax parsing invokes a subprocess; generation is never executed.
The committed protos, Go gateway and Swagger are the comparison inputs.
"""

import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[3]
MODULE = "github.com/Sidiora-Labs/Paxeer-X-Network"
QUERY = "wasm/proto/cosmwasm/wasm/v1/query.proto"
RECORD = "tools/qualification/audit_repairs/repair_22_33_1252.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def active_shell(path):
    return "\n".join(
        line for line in source(path).splitlines() if not line.lstrip().startswith("#")
    ).replace("\\\n", "")


def yaml_block(text, key, indent=0):
    """Read an exact block mapping entry from the committed YAML subset.

    Indentation bounds the value; quoted route/status keys are decoded with
    shlex. This does not attempt to interpret arbitrary YAML or execute tags.
    """
    lines = text.splitlines()
    matches = []
    for index, line in enumerate(lines):
        if len(line) - len(line.lstrip()) != indent or not line.endswith(":"):
            continue
        tokens = shlex.split(line.strip()[:-1])
        if tokens == [key]:
            matches.append(index)
    assert len(matches) == 1, (key, matches)
    start = matches[0] + 1
    end = start
    while end < len(lines):
        line = lines[end]
        if line.strip() and len(line) - len(line.lstrip()) <= indent:
            break
        end += 1
    return "\n".join(lines[start:end])


def query_methods():
    methods = re.findall(
        r"rpc (\w+)\((\w+)\) returns \((\w+)\) \{\s*"
        r'option \(google.api.http\).get = "([^"]+)";\s*}',
        source(QUERY),
    )
    assert len(methods) == 9
    assert len({method[0] for method in methods}) == 9
    return methods


def query_messages():
    proto = re.sub(r"//[^\n]*", "", source(QUERY))
    messages = {}
    for name, body in re.findall(r"message (\w+) \{(.*?)\n}", proto, re.S):
        fields = re.findall(
            r"^  (?:(repeated) )?([\w.]+) (\w+) = (\d+)"
            r"(?:\s*\[(.*?)\])?;",
            body,
            re.M | re.S,
        )
        assert fields, name
        messages[name] = fields
    assert len(messages) == 19
    return messages


def test_wasm_generator_shell_syntax_and_delegation():
    paths = [
        "scripts/protoc.sh",
        "wasm/scripts/protocgen.sh",
        "wasm/scripts/protoc-swagger-gen.sh",
    ]
    # bash -n accepts one script; parse every script separately, including root.
    for path in paths:
        result = subprocess.run(
            ["bash", "-n", str(ROOT / path)], capture_output=True, text=True
        )
        assert result.returncode == 0, (path, result.stderr)
    for name, options in (
        ("protocgen.sh", []),
        ("protoc-swagger-gen.sh", ["--wasm-swagger"]),
    ):
        shell = active_shell("wasm/scripts/" + name)
        lines = [line for line in shell.splitlines() if line]
        assert lines == [
            "set -euo pipefail",
            'repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"',
            'exec bash "$repo_root/scripts/protoc.sh" '
            + " ".join(options + ['"$@"']),
        ]
        assert shlex.split(lines[-1]) == [
            "exec", "bash", "$repo_root/scripts/protoc.sh", *options, "$@"
        ]
        assert "@latest" not in shell
        assert "buf protoc" not in shell


def test_wasm_swagger_root_mode_uses_pinned_buf_workspace():
    shell = active_shell("scripts/protoc.sh")
    assert shell.startswith(
        '\nset -euo pipefail\n\nrepo_root="$(cd -- '
        '"$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"\ncd -- "$repo_root"'
    )
    mode = shell.split("  --wasm-swagger)\n", 1)[1].split("    ;;", 1)[0]
    assert mode.index("if [[ $# -ne 1 ]]") < mode.index("exec go run")
    assert "exit 2" in mode
    template = json.loads(re.search(r"swagger_template='(.*?)'", mode, re.S)[1])
    assert template == {
        "version": "v2",
        "inputs": [{"directory": "wasm/proto"}],
        "plugins": [{
            "local": ["go", "run", "github.com/grpc-ecosystem/grpc-gateway/"
                      "protoc-gen-swagger@v1.16.0"],
            "out": "./build/swagger/wasm",
            "opt": ["logtostderr=true", "fqn_for_swagger_name=true",
                    "simple_operation_ids=true"],
        }],
    }
    command = mode[mode.index("exec go run"):].strip()
    assert shlex.split(command) == [
        "exec", "go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0",
        "generate", "--template", "$swagger_template", "--path", QUERY,
    ]
    dispatch = shell.split("esac", 1)[0]
    assert 'if [[ $# -ne 0 ]]' in dispatch
    assert re.search(r"\*\)\s*echo .*?\n\s*exit 2", dispatch)
    assert shell.index("esac") < shell.index("rm -rf ./build/proto")
    assert not any(token in mode for token in ("buf protoc", "-I ", "go get", "rm -rf"))


def test_wasm_imports_resolve_from_canonical_module_roots():
    workspace = source("buf.yaml")
    assert re.search(r"^version: v2$", workspace, re.M)
    roots = re.findall(r"^  - path: (\S+)$", yaml_block(workspace, "modules"), re.M)
    assert {"wasm/proto", "sdk/proto", "sdk/third_party/proto"} <= set(roots)
    assert all((ROOT / root).is_dir() for root in roots)
    imports = re.findall(r'^import "([^"]+)";', source(QUERY), re.M)
    assert set(imports) == {
        "cosmos/base/query/v1beta1/pagination.proto",
        "cosmwasm/wasm/v1/types.proto",
        "gogoproto/gogo.proto",
        "google/api/annotations.proto",
    }
    for imported in imports:
        assert any((ROOT / root / imported).is_file() for root in roots), imported
    for imported in (
        "google/api/http.proto", "cosmos_proto/cosmos.proto", "google/protobuf/any.proto"
    ):
        assert any((ROOT / root / imported).is_file() for root in roots), imported
    assert not (ROOT / "wasm/third_party/proto").exists()


def test_wasm_codegen_preserves_root_pins_any_mapping_and_output_owner():
    shell = active_shell("scripts/protoc.sh")
    assert "go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos@v0.3.1" in shell
    assert 'go build -o "${build_out}/protoc-gen-gocosmos" ./protoc-gen-gocosmos' in shell
    assert "@latest" not in shell
    config = source("buf.gen.yaml")
    inputs = yaml_block(config, "inputs")
    assert "wasm/proto" in re.findall(r"^  - directory: (\S+)$", inputs, re.M)
    plugins = yaml_block(config, "plugins")
    blocks = re.split(r"^  - local:\s*$", plugins, flags=re.M)[1:]
    assert len(blocks) == 3
    gocosmos, gateway, _ = blocks
    assert re.search(r"^      - ./build/proto/gocosmos/protoc-gen-gocosmos$", gocosmos, re.M)
    options = re.findall(r"^      - (\S+)$", yaml_block(gocosmos, "opt", 4), re.M)
    assert "plugins=interfacetype+grpc" in options
    assert "Mgoogle/protobuf/any.proto=" + MODULE + "/sdk/codec/types" in options
    assert re.search(r"^    out: ./build/proto/gocosmos$", gocosmos, re.M)
    assert "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.16.0" in gateway
    assert re.search(r"^    out: ./build/proto/gocosmos$", gateway, re.M)
    assert "go run github.com/bufbuild/buf/cmd/buf@v1.58.0 generate\n" in shell
    assert "cp -rf ./build/proto/gocosmos/" + MODULE + "/wasm/* ./wasm" in shell
    types = source("wasm/x/wasm/types/types.pb.go")
    assert 'types "' + MODULE + '/sdk/codec/types"' in types
    assert re.search(r"Extension\s+\*types.Any `protobuf:", types)


def test_wasm_query_service_and_gateway_retain_all_declared_routes():
    generated = source("wasm/x/wasm/types/query.pb.go")
    gateway = source("wasm/x/wasm/types/query.pb.gw.go")
    module = source("wasm/x/wasm/module.go")
    assert "types.RegisterQueryServer(cfg.QueryServer(), NewQuerier(am.keeper))" in module
    assert "types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(clientCtx))" in module
    assert "wasm.AppModuleBasic{}" in source("node/app.go")
    assert 'ServiceName: "cosmwasm.wasm.v1.Query"' in generated
    assert re.findall(r'MethodName: "(\w+)"', generated) == [m[0] for m in query_methods()]
    client = gateway.split("func RegisterQueryHandlerClient(", 1)[1].split("\nvar (", 1)[0]
    # Match both the opcodes and their string pool, so variable captures and
    # literal segments cannot trade places while retaining the same words.
    for method, request, response, route in query_methods():
        assert f"{method}(context.Context, *{request}) (*{response}, error)" in generated
        assert f'c.cc.Invoke(ctx, "/cosmwasm.wasm.v1.Query/{method}", in, out, opts...)' in generated
        assert f'mux.Handle("GET", pattern_Query_{method}_0,' in client
        assert f"request_Query_{method}_0(rctx, inboundMarshaler, client, req, pathParams)" in client
        pattern = re.search(
            rf"pattern_Query_{method}_0 = runtime.MustPattern\(runtime.NewPattern\(1, "
            r"\[\]int\{([^}]+)}, \[\]string\{([^}]+)}", gateway
        )
        assert pattern, method
        pool, opcodes = [], []
        for segment in route.strip("/").split("/"):
            variable = segment.startswith("{")
            pool.append(segment.strip("{}"))
            index = len(pool) - 1
            opcodes.extend([1, 0, 4, 1, 5, index] if variable else [2, index])
        assert [int(value) for value in pattern[1].split(",")] == opcodes
        assert json.loads("[" + pattern[2] + "]") == pool


def test_wasm_query_message_wire_fields_match_committed_generated_types():
    generated = source("wasm/x/wasm/types/query.pb.go")
    messages = query_messages()
    for _, request, response, _ in query_methods():
        assert request in messages and response in messages
    for name, fields in messages.items():
        struct = re.search(rf"type {name} struct \{{(.*?)\n}}", generated, re.S)
        assert struct, name
        tags = re.findall(r'protobuf:"([^"]+)"', struct[1])
        assert len(tags) == len(fields), name
        actual = {}
        for tag in tags:
            parts = tag.split(",")
            field_name = next(part[5:] for part in parts if part.startswith("name="))
            actual[field_name] = (parts[0], parts[1], parts[2])
        expected = {
            field: ("varint" if kind == "uint64" else "bytes", number,
                    "rep" if repeated else "opt")
            for repeated, kind, field, number, _ in fields
        }
        assert actual == expected, name
    assert "Data RawContractMessage" in generated
    assert 'query "' + MODULE + '/sdk/types/query"' in generated


def test_wasm_committed_swagger_routes_and_response_shapes_match_service():
    swagger = source("docs/swagger-ui/swagger.yml")
    assert re.search(r"^swagger: ['\"]2.0['\"]$", swagger, re.M)
    paths = yaml_block(swagger, "paths")
    definitions = yaml_block(swagger, "definitions")
    messages = query_messages()
    documented = set(re.findall(r"^  ['\"]?(/cosmwasm/wasm/v1/[^\n]*?)['\"]?:$", paths, re.M))
    assert documented == {method[3] for method in query_methods()}
    for _, _, response, route in query_methods():
        operation = yaml_block(yaml_block(paths, route, 2), "get", 4)
        success = yaml_block(yaml_block(operation, "responses", 6), "200", 8)
        schema = yaml_block(success, "schema", 10)
        props = yaml_block(schema, "properties", 12)
        definition = yaml_block(definitions, "cosmwasm.wasm.v1." + response, 2)
        defined = yaml_block(definition, "properties", 4)
        fields = messages[response]
        assert set(re.findall(r"^              (\w+):$", props, re.M)) == {f[2] for f in fields}
        assert set(re.findall(r"^      (\w+):$", defined, re.M)) == {f[2] for f in fields}
        for repeated, kind, field, _, _ in fields:
            for block, indent in ((props, 14), (defined, 6)):
                value = yaml_block(block, field, indent)
                expected = "array" if repeated else "string" if kind in ("string", "bytes", "uint64") else "object"
                assert re.search(rf"^{' ' * (indent + 2)}type: {expected}$", value, re.M)
                if repeated:
                    value = yaml_block(value, "items", indent + 2)
                    indent += 2
                if kind in ("bytes", "uint64"):
                    fmt = "byte" if kind == "bytes" else "uint64"
                    assert re.search(rf"^{' ' * (indent + 2)}format: {fmt}$", value, re.M)


def test_wasm_repair_record_maps_findings_limits_wiring_and_evidence():
    record = tomllib.loads(source(RECORD))
    expected = {
        "wasm-swagger-import-roots": "test_wasm_swagger_root_mode_uses_pinned_buf_workspace",
        "wasm-protoc-generation-drift": "test_wasm_codegen_preserves_root_pins_any_mapping_and_output_owner",
    }
    assert set(record["finding"]) == set(expected)
    test_source = source("tools/qualification/audit_repairs/test_repair_22_33_1252.py")
    for finding, test in expected.items():
        entry = record["finding"][finding]
        assert test in entry["tests"]
        for name in entry["tests"]:
            assert f"def {name}():" in test_source
        assert entry["audit"] == "22.30.61"
    task = record["task"]["22"]["33"]["1252"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "no baseline" in task["baseline"].lower()
    assert "not executed" in task["review_only"]
    for path in ("wasm/Makefile", "docs/swagger-ui/swagger.yml", "wasm/x/wasm/types"):
        assert any(path in request and "domain" in request for request in task["wiring_requests"])
    readme = source("wasm/scripts/README.md")
    for command in ("bash scripts/protoc.sh", "bash scripts/protoc.sh --wasm-swagger"):
        assert command in readme
    assert "query.swagger.json" in readme
    assert "wasm/Makefile" in readme
