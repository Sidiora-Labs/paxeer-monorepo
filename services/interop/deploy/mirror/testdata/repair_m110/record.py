"""Record production configuration outputs; never invoked by the Go gate."""

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile


HERE = Path(__file__).resolve().parent
MIRROR = HERE.parents[1]
ROOT = MIRROR.parents[3]
GATEWAY = MIRROR.parent / "gateway"


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def record():
    mirror = load("mirror_renderer", MIRROR / "render-config.py")
    gateway = load("gateway_renderer", GATEWAY / "render.py")
    example = json.loads((MIRROR / "config.example.json").read_text())
    canonical = HERE / "receipt-keys.json"
    keys = json.loads(canonical.read_text())
    gateway_example = json.loads((GATEWAY / "config.example.json").read_text())
    environ = {
        "LAYERX_BETA_INTEROP_" + name.upper(): json.dumps(gateway_example[name])
        for name in ("ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers")
    }
    paths = [
        MIRROR / "render-config.py", MIRROR / "config.example.json",
        canonical, Path(__file__).resolve(), GATEWAY / "render.py",
        ROOT / "services/interop/crates/layerx-mirror/tests/fixtures/committed_blocks.json",
        ROOT / "services/agent/crates/layerx-proof/src/receipt.rs",
        ROOT / "services/agent/crates/layerx-proof/src/inclusion.rs",
    ]
    transcript = {
        "source_hashes": {
            str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in paths
        },
        "renderings": [], "policies": [], "endpoint_refusals": [],
    }
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        output = directory / "mirror.json"
        arguments = [
            "--output", str(output), "--state-directory", "/var/lib/layerx-mirror",
            "--first-height", "1", "--status-listen", "127.0.0.1:9091",
            "--paxeer-chain-id", "125", "--paxeer-genesis-hash", "11" * 32,
            "--receipt-keys-file", str(canonical), "--ethereum-chain-id", "11155111",
            "--ethereum-genesis-hash", "11" * 32,
            "--ethereum-archive-contract", "22" * 20,
            "--ethereum-archive-code-hash", "33" * 32,
            "--ethereum-signer-public-key",
            "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
        ]
        for chain in ("paxeer", "ethereum"):
            for endpoint in example[chain]["rpc"]["endpoints"]:
                arguments.extend(["--" + chain + "-endpoint", ",".join([
                    endpoint["url"], endpoint["independent_backend"],
                    endpoint["ca_certificate_der"], endpoint["bearer_token_file"],
                ])])
        for with_solana in (False, True):
            argv = list(arguments)
            if with_solana:
                for endpoint in example["solana"]["rpc"]["endpoints"]:
                    argv.extend(["--solana-endpoint", ",".join([
                        endpoint["url"], endpoint["independent_backend"],
                        endpoint["ca_certificate_der"], endpoint["bearer_token_file"],
                    ])])
                for name in ("genesis-hash", "archive-program", "upgradeable-loader",
                             "program-data-account", "signer-public-key"):
                    argv.extend(["--solana-" + name, "11111111111111111111111111111112"])
                argv.extend(["--solana-program-code-hash", "44" * 32])
            mirror.main(argv)
            transcript["renderings"].append(json.loads(output.read_text()))
        accepted = gateway.receipt_keys(canonical)
        transcript["gateway_keys"] = accepted
        transcript["gateway"] = gateway.render(
            ROOT, environ, "layerx-beta",
            sorted({key["publicKey"][2:] for key in accepted}),
        )

        key = keys[1]
        cases = {
            "canonical": keys,
            "zero_only": [{**key, "lastHeight": 0}],
            "full_uint64": [{**key, "lastHeight": 2**64 - 1}],
            "max_only": [{**key, "firstHeight": 2**64 - 1, "lastHeight": 2**64 - 1}],
            "duplicate_disjoint": [key, {**keys[0], "keyId": key["keyId"]}],
            "duplicate_normalized": [key, {**keys[0], "keyId": key["keyId"][2:].upper()}],
            "overlap": [key, {**keys[0], "firstHeight": 1000}],
            "negative_first": [{**key, "firstHeight": -1}],
            "negative_last": [{**key, "lastHeight": -1}],
            "overflow_first": [{**key, "firstHeight": 2**64}],
            "overflow_last": [{**key, "lastHeight": 2**64}],
            "reversed": [{**key, "firstHeight": 1001}],
            "boolean_first": [{**key, "firstHeight": False}],
            "boolean_last": [{**key, "lastHeight": True}],
            "string_first": [{**key, "firstHeight": "0"}],
            "float_last": [{**key, "lastHeight": 1000.0}],
            "zero_key": [{**key, "publicKey": "0x" + "00" * 32}],
            "zero_id": [{**key, "keyId": "0x" + "00" * 32}],
            "short_key": [{**key, "publicKey": "0x" + "11" * 31}],
            "malformed_id": [{**key, "keyId": "0x" + "zz" * 32}],
            "empty": [],
            "unknown_field": [{**key, "unexpected": True}],
        }
        policy_file = directory / "policy.json"
        for name, entries in cases.items():
            policy_file.write_text(json.dumps(entries))
            result = {"name": name, "input": entries}
            for component, producer, refusal in (
                ("mirror", mirror.checkpoint_policy, SystemExit),
                ("gateway", gateway.receipt_keys, gateway.Refused),
            ):
                try:
                    result[component] = {"keys": producer(policy_file)}
                except refusal as error:
                    # Preserve the real refusal; never replace it with an expected verdict.
                    result[component] = {"error": str(error).replace(str(policy_file), "<keys>")}
            transcript["policies"].append(result)
        for url in (
            "https://paxeer-a.example/rpc", "https://paxeer-a.example/evm",
            "https://paxeer-a.example/%2f", "https://paxeer-a.example/?rpc=1",
            "https://paxeer-a.example/#rpc", "https://user@paxeer-a.example/",
        ):
            endpoint_args = [
                url + ",a,/run/secrets/a.der,/run/secrets/a.token",
                "https://paxeer-b.example/,b,/run/secrets/b.der,/run/secrets/b.token",
            ]
            entry = {"url": url}
            try:
                entry["rpc"] = mirror.endpoints(endpoint_args, "--paxeer-endpoint")
            except SystemExit as error:
                entry["error"] = str(error)
            transcript["endpoint_refusals"].append(entry)
    (HERE / "rendered.json").write_text(json.dumps(transcript, indent=2) + "\n")


if __name__ == "__main__":
    record()
