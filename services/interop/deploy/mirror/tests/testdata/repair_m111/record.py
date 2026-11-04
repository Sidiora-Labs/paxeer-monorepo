"""Record production renderer outputs; not invoked by the Go qualification gate."""

import ast
import hashlib
import importlib.util
import json
import re
import tempfile
from pathlib import Path

TESTS = Path(__file__).resolve().parents[2]
RENDERER = TESTS.parent / "render-config.py"


def record():
    spec = importlib.util.spec_from_file_location("mirror_renderer", RENDERER)
    renderer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(renderer)
    shell = (TESTS / "render-config-check.sh").read_text()
    python = (TESTS / "test_render_check_arguments.py").read_text()
    shell_endpoints = {}
    for chain in ("paxeer", "ethereum", "solana"):
        # The first pair belongs to render_ethereum_only / solana_target;
        # subsequent Paxeer occurrences belong to the refusal scenario.
        shell_endpoints[chain] = re.findall(
            r'--' + chain + r'-endpoint\s+"([^"]+)"', shell
        )[:2]
        shell_endpoints[chain] = [
            value.replace("$WORK", "/run/secrets")
            for value in shell_endpoints[chain]
        ]
    function = next(
        node for node in ast.parse(python).body
        if isinstance(node, ast.FunctionDef)
        and node.name == "test_rendered_keys_match_the_example"
    )
    endpoint_loop = next(
        node for node in function.body
        if isinstance(node, ast.For) and ast.unparse(node.target) == "chain"
    )
    # Execute the actual endpoint producer loop, rather than copying its template.
    namespace = {"command": []}
    exec(compile(ast.Module(body=[endpoint_loop], type_ignores=[]),
                 "test_render_check_arguments.py", "exec"), namespace)
    python_endpoints = {"paxeer": [], "ethereum": [], "solana": []}
    command = namespace["command"]
    for index in range(0, len(command), 2):
        python_endpoints[command[index].removeprefix("--").removesuffix("-endpoint")].append(
            command[index + 1]
        )
    solana_loop = next(
        node for node in ast.walk(function)
        if isinstance(node, ast.For)
        and ast.unparse(node.target) == "backend"
        and "--solana-endpoint" in ast.unparse(node)
    )
    namespace = {"arguments": []}
    exec(compile(ast.Module(body=[solana_loop], type_ignores=[]),
                 "test_render_check_arguments.py", "exec"), namespace)
    python_endpoints["solana"] = namespace["arguments"][1::2]
    records = {
        "renderer_sha256": hashlib.sha256(RENDERER.read_bytes()).hexdigest(),
        "sources": {},
    }
    with tempfile.TemporaryDirectory() as directory:
        directory = Path(directory)
        keys = directory / "receipt-keys.json"
        public_key = "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e"
        keys.write_text(json.dumps([{
            "keyId": "0x" + hashlib.sha256(bytes.fromhex(public_key)).hexdigest(),
            "publicKey": "0x" + public_key,
            "firstHeight": 1,
            "lastHeight": 18446744073709551615,
        }]))
        for source, endpoints in (
            ("render-config-check.sh", shell_endpoints),
            ("test_render_check_arguments.py", python_endpoints),
        ):
            entry = {
                "sha256": hashlib.sha256((TESTS / source).read_bytes()).hexdigest(),
                "endpoint_arguments": endpoints,
                "renderings": [],
            }
            for with_solana in (False, True):
                output = directory / "rendered.json"
                arguments = [
                    "--output", str(output), "--state-directory", "/run/mirror/state",
                    "--first-height", "1", "--status-listen", "127.0.0.1:9091",
                    "--paxeer-chain-id", "125",
                    "--paxeer-genesis-hash", hashlib.sha256(b"paxeer-genesis").hexdigest(),
                    "--receipt-keys-file", str(keys),
                    "--ethereum-chain-id", "11155111",
                    "--ethereum-genesis-hash", "11" * 32,
                    "--ethereum-archive-contract", "22" * 20,
                    "--ethereum-archive-code-hash", "33" * 32,
                    "--ethereum-signer-public-key",
                    "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
                ]
                for chain in ("paxeer", "ethereum", "solana"):
                    if chain == "solana" and not with_solana:
                        continue
                    for endpoint in endpoints[chain]:
                        arguments.extend(["--" + chain + "-endpoint", endpoint])
                if with_solana:
                    for field in ("genesis-hash", "archive-program", "upgradeable-loader",
                                  "program-data-account", "signer-public-key"):
                        arguments.extend(["--solana-" + field,
                                          "11111111111111111111111111111112"])
                    arguments.extend(["--solana-program-code-hash", "44" * 32])
                if renderer.main(arguments) != 0:
                    raise RuntimeError("production renderer refused fixture inputs")
                entry["renderings"].append(json.loads(output.read_text()))
            records["sources"][source] = entry
    (Path(__file__).parent / "rendered.json").write_text(
        json.dumps(records, indent=2) + "\n"
    )


if __name__ == "__main__":
    record()
