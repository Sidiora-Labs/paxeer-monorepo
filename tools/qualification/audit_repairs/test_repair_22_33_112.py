"""Bounded source qualification; native/Rust execution is explicitly review-only.

No compiler, node, credential, environment variable or external service is used.
The declared hf-build owns bash -n. Pure settings/pointer code is executed from
its actual source, and guarded native commands are checked against real ABIs,
producer fixtures, loader source and deployment files.
"""

import ast
import base64
import configparser
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import sys

import pytest

ROOT = Path(__file__).resolve().parents[3]
TESTS = ROOT / "services/interop/deploy/x-websearch/tests"
DRY = TESTS / "dry-run-check.sh"
PACKAGING = TESTS / "packaging-check.sh"
NATIVE = TESTS / "paxd-check.py"
EXAMPLE = ROOT / "services/interop/deploy/x-websearch/config.example.json"
VALID = ROOT / "services/interop/crates/x-websearch/tests/fixtures/config/valid.json"
GENESIS = ROOT / "services/platform/hosted/node/tests/fixtures/repair_22_31_355/genesis.json"
LOADER = ROOT / "services/interop/crates/x-websearch/src/config.rs"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_112.kvx"


def embedded_python(path):
    return re.findall(r"<<'([A-Z]+)'\n(.*?)\n\1(?:\n|$)", path.read_text(), re.S)


def functions(source, names, namespace=None):
    """Execute selected real pure helpers, with no imports or runtime entrypoint."""
    tree = ast.parse(source)
    selected = [node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name in names]
    assert {node.name for node in selected} == set(names)
    namespace = dict(namespace or {})
    exec(compile(ast.Module(body=selected, type_ignores=[]), "<source helpers>", "exec"), namespace)
    return namespace


def method(source, class_name, name):
    cls = next(node for node in ast.parse(source).body if isinstance(node, ast.ClassDef) and node.name == class_name)
    return next(node for node in cls.body if isinstance(node, ast.FunctionDef) and node.name == name)


def calls(node):
    return [ast.unparse(item.func) for item in ast.walk(node) if isinstance(item, ast.Call)]


def contains_call(node, name, literal):
    return any(isinstance(item, ast.Call) and ast.unparse(item.func) == name
               and any(isinstance(arg, ast.Constant) and arg.value == literal for arg in item.args)
               for item in ast.walk(node))


def test_shell_embedded_programs_are_parsed_and_native_entrypoint_is_guarded():
    for path in (DRY, PACKAGING):
        assert "set -euo pipefail" in path.read_text()
        programs = embedded_python(path)
        assert programs
        for _, source in programs:
            ast.parse(source)
    ast.parse(NATIVE.read_text())
    shell = DRY.read_text()
    native_branch = shell[shell.index('if [ "${1-}" = --paxd ];'):shell.index("MODE=live")]
    assert '[ "$#" -eq 2 ] ||' in native_branch
    assert 'exec python3 "$(dirname "$0")/paxd-check.py" "$2"' in native_branch
    # Focused callback coverage remains registered alongside the native scenario.
    assert 'rpc("anvil_setStorageAt"' in shell
    assert 'rpc("anvil_impersonateAccount", [PRECOMPILE])' in shell
    assert 'python3 "$HARNESS" run "$CONTEXT"' in shell


def test_xweb_synthetic_lifecycle_native_request_restart_and_callback_guards():
    source = NATIVE.read_text()
    lifecycle = method(source, "NativeScenario", "lifecycle")
    start = method(source, "NativeScenario", "start")
    sending = method(source, "NativeScenario", "send")
    # A signed consumer ask traverses the native precompile. No injected storage,
    # synthesized event or impersonation is reachable in the native scenario.
    assert contains_call(lifecycle, "encode_call", "ask")
    assert "self.send" in calls(lifecycle)
    assert contains_call(start, "self.send", "0x")  # contract creation transaction
    assert "self.rpc.send" in calls(sending)
    assert "key.sign_msg_hash" in calls(sending)
    assert not re.search(r"anvil_|impersonate|setStorageAt|class Front", source)
    assert 'PRECOMPILE = "0x" + "00" * 18 + "1019"' in source
    abi = json.loads((ROOT / "precompiles/xweb/abi.json").read_text())
    fulfil = next(entry for entry in abi if entry.get("name") == "fulfil")
    assert [field["type"] for field in fulfil["inputs"]] == ["uint64", "bytes", "bytes32", "uint32", "bytes[]"]
    assert contains_call(lifecycle, "encode_call", "fulfil")
    request_state = method(source, "NativeScenario", "request_state")
    assert contains_call(request_state, "self.view", "getRequest")
    body = ast.unparse(lifecycle)
    ordered = ['wait(observed', 'process.stop()', 'self.node.stop()', 'self.node.start()',
               'wait(recovered', 'process.start()', 'wait(attestations', 'restored == records',
               "['X_WEBSEARCH_SUBMITTER_KEY_FILE']", 'wait(lambda: self.callbacks']
    at = 0
    for fragment in ordered:
        at = body.index(fragment, at) + len(fragment)
    assert "entry['request']['request_id'] == request_id" in body
    assert "self.request_state(request_id) == before" in body
    assert "restored == records" in body
    assert 'watch/pending.json' in source
    assert "pending.json" in (ROOT / "services/interop/crates/x-websearch/src/watch.rs").read_text()
    assert 'config.data_dir.join("watch")' in (ROOT / "services/interop/crates/x-websearch/src/main.rs").read_text()
    assert "duplicate=True" in body
    assert "LXP_XWEB_ALREADY_FULFILLED" in ast.unparse(sending)
    assert "receipt['status'] == '0x0' and (not receipt['logs'])" in ast.unparse(sending)
    assert "len(self.callbacks(request_id, receipt['blockNumber'])) == 1" in body
    assert "consumer response differs from module result" in body
    assert "recovery changed consumer answer" in body
    assert "<= 400000" in body
    owned = ast.unparse(method(source, "OwnedProcess", "stop"))
    assert "self.process.terminate()" in owned and "self.process.wait(timeout=30)" in owned
    assert "shutil.copytree" in calls(method(source, "NativeScenario", "__init__"))


def test_xweb_dry_run_config_uses_native_genesis_windows_and_production_loader_fields(tmp_path):
    shell = DRY.read_text()
    harness = dict(embedded_python(DRY))["PY"]
    namespace = functions(harness, {"receipt_windows"}, {"os": os, "json": json})
    windows = namespace["receipt_windows"](str(ROOT))
    native = functions(NATIVE.read_text(), {"native_windows", "sidecar_settings"}, {"base64": base64, "copy": copy})
    assert windows == native["native_windows"](json.loads(GENESIS.read_text()))
    assert windows and all(0 < key["first_height"] <= key["last_height"] <= (1 << 64) - 1 for key in windows)
    template = json.loads(VALID.read_text())
    config = native["sidecar_settings"](template, windows, "http://127.0.0.1:8545", "http://127.0.0.1:8545/rpc",
                                        "127.0.0.1:8480", tmp_path, "http://127.0.0.1:8080/index.html", [], template["assets"])
    assert config["gateway"] == {"endpoint": "http://127.0.0.1:8545/rpc", "receipt_keys": windows}
    assert "kernel_network_id" not in config
    assert not {"sequencer_id", "sequencer_public_key"} & config["gateway"].keys()
    # Exercise the focused harness's actual configuration writer, too.
    ctx = {"sidecars": [{"listen": "127.0.0.1:8480", "data_dir": str(tmp_path), "url": "http://127.0.0.1:8480"}],
           "site_url": "http://127.0.0.1:8080", "front_url": "http://127.0.0.1:8545", "chain_id": 1337,
           "receipt_keys": windows}
    context_path, output = tmp_path / "context.json", tmp_path / "config.json"
    context_path.write_text(json.dumps(ctx))
    generator = functions(harness, {"context", "config"}, {"json": json, "hashlib": hashlib})
    generator["config"](context_path, 0, output)
    assert json.loads(output.read_text())["gateway"]["receipt_keys"] == windows
    assert '"sequencer_id"' not in harness and '"sequencer_public_key"' not in harness
    assert '"$BINARY" --config "$dir/config.json"' in shell
    loader = LOADER.read_text()
    gateway = loader[loader.index("fn gateway("):loader.index("fn payment(")]
    for field in ("key_id", "public_key", "first_height", "last_height"):
        assert f'"{field}"' in gateway
    assert 'object.take("receipt_keys")?' in gateway
    assert "last_height < first_height" in gateway
    assert "first_height <= key.last_batch_number()" in gateway
    # Drive both production codecs over the unchanged producer's wire shape.
    sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
    from layerx_sdk.x402_rpc import decode_receipt_keys
    keys = decode_receipt_keys({"keys": [{"keyId": "0x" + k["key_id"], "publicKey": "0x" + k["public_key"],
                                         "firstHeight": k["first_height"], "lastHeight": k["last_height"]} for k in windows]})
    assert [(k.key_id.hex(), k.public_key.hex(), k.first_height, k.last_height) for k in keys] == [
        (k["key_id"], k["public_key"], k["first_height"], k["last_height"]) for k in windows]
    with pytest.raises(ValueError):
        decode_receipt_keys({"keys": [{"keyId": "0x" + windows[0]["key_id"], "publicKey": "0x" + windows[0]["public_key"],
                                      "firstHeight": windows[0]["last_height"], "lastHeight": 0}]})


def test_xweb_dry_run_config_real_relay_and_paid_response_reopen_guards():
    source = NATIVE.read_text()
    payment = ast.unparse(method(source, "NativeScenario", "payment"))
    for fragment in ("self.send(LEDGER, data)", "self.rpc.receipt(transaction)", "verify_payment_receipt(signed, raw, self.pins",
                     "verify_sequencer_receipt(returned, raw, self.pins)", "returned == signed", "checked.height == verified.height",
                     "receipt_digest(signed)", "binding['responseDigest'] == hashlib.sha256(body).hexdigest()",
                     "process.stop()", "process.start()", "replay_body == body", "replay_headers['payment-response'] == response_headers['payment-response']"):
        assert fragment in payment
    assert "'verificationLevel': 'receipted'" in payment
    assert "'idempotencyKey': reference.hex()" in payment
    assert '"LAYERX-PAYER-ADDRESS"' in source
    production_payment = (ROOT / "services/interop/crates/x-websearch/src/payment.rs").read_text()
    assert 'pub const PAYER_ADDRESS: &str = "LAYERX-PAYER-ADDRESS";' in production_payment
    assert 'fields.get("verificationLevel").and_then(Value::as_str) != Some("receipted")' in production_payment
    assert 'self.gate.rpc' in production_payment or '.sequencer_receipt(&hash)' in production_payment
    start = ast.unparse(method(source, "NativeScenario", "start"))
    assert "observed == self.windows" in start
    assert "self.rpc.receipt_keys()" in start
    # The receipt digest helper consumes the producer's exact bytes unchanged.
    vectors = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    case = next(case for case in vectors["cases"] if case["id"] == "V1-layerxledger-pay-exact")
    helpers = functions(source, {"receipt_digest", "require"}, {"hashlib": hashlib})
    assert "0x" + helpers["receipt_digest"](bytes.fromhex(case["signedReceipt"][2:])) == case["receiptDigest"]


def test_recorded_native_payment_crosses_production_relay():
    # The RPC peer serves the committed native producer transcript unchanged;
    # PaymentRpc is the actual production relay client, not a replacement.
    from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
    import threading
    sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
    from layerx_sdk.x402_rpc import HostedGatewayRpc, PaymentRpc

    vectors = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    assert vectors["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    case = next(case for case in vectors["cases"] if case["id"] == "V1-layerxledger-pay-exact")
    observed = []

    class RecordedPeer(BaseHTTPRequestHandler):
        def log_message(self, *_args):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            name, params = request["method"], request["params"]
            assert self.path == ("/rpc" if name == "px_getReceiptKeys" else "/")
            observed.append((name, params))
            if name == "px_getReceiptKeys":
                assert params == []
                result = {"keys": [case["pinnedKey"]]}
            elif name == "eth_sendRawTransaction":
                assert params == [case["rawTransaction"]]
                result = case["receiptJson"]["txHash"]
            else:
                assert name == "pax_sequencerReceipt"
                assert params == [case["receiptJson"]["txHash"]]
                result = case["receiptJson"]
            body = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    peer = ThreadingHTTPServer(("127.0.0.1", 0), RecordedPeer)
    worker = threading.Thread(target=peer.serve_forever, daemon=True)
    worker.start()
    try:
        relay = PaymentRpc(f"http://127.0.0.1:{peer.server_port}/")
        gateway = HostedGatewayRpc(f"http://127.0.0.1:{peer.server_port}/rpc")
        pins = gateway.receipt_keys()
        transaction = relay.send(case["rawTransaction"])
        receipt = relay.receipt(transaction)
        assert receipt == case["receiptJson"]
        assert receipt["receipt"] == case["signedReceipt"]
        assert receipt["keyId"] == "0x" + pins[0].key_id.hex()
        assert pins[0].first_height <= int(receipt["height"], 16) <= pins[0].last_height
        assert [name for name, _ in observed] == ["px_getReceiptKeys", "eth_sendRawTransaction", "pax_sequencerReceipt"]
    finally:
        peer.shutdown()
        peer.server_close()
        worker.join(timeout=5)
        assert not worker.is_alive()


def docker_stages(path):
    stages = []
    for line in path.read_text().splitlines():
        if line.startswith("FROM "):
            fields = shlex.split(line)
            assert len(fields) in (2, 4)
            if len(fields) == 4:
                assert fields[2] == "AS"
            assert fields[1] in {stage[0] for stage in stages} or re.fullmatch(
                r"[a-z0-9./-]+:[A-Za-z0-9._-]+@sha256:[a-f0-9]{64}", fields[1]
            )
            assert ":latest@" not in fields[1]
            stages.append((fields[3] if len(fields) == 4 else "runtime", fields[1], [line]))
        elif stages:
            stages[-1][2].append(line)
    return stages


def test_xweb_packaging_stages_guards_match_build_fly_and_default_runtime():
    source = PACKAGING.read_text()
    stages = docker_stages(ROOT / "docker/x-websearch/Dockerfile")
    assert [stage[0] for stage in stages] == ["build", "fly", "runtime"]
    assert "expected exactly two stages" not in source
    assert 'FLY_STAGE=$(named_stage "$IMAGE_DOCKERFILE" fly)' in source
    assert 'RUNTIME_STAGE=$(final_stage "$IMAGE_DOCKERFILE")' in source
    assert 'every_from_pinned "$IMAGE_DOCKERFILE"' in source
    variables = dict(re.findall(r"^(RUST_BASE|DEBIAN_BASE|BUILD_LINE)='([^']+)'$", source, re.M))
    variables.update(BUILD_STAGE="\n".join(stages[0][2]), FLY_STAGE="\n".join(stages[1][2]),
                     RUNTIME_STAGE="\n".join(stages[2][2]), IMAGE_TEXT=(ROOT / "docker/x-websearch/Dockerfile").read_text(),
                     FLY_INIT_TEXT=(ROOT / "docker/x-websearch/entrypoint.sh").read_text())
    checked = {name: 0 for name in ("BUILD_STAGE", "FLY_STAGE", "RUNTIME_STAGE", "FLY_INIT_TEXT")}
    for line in source.splitlines():
        if not line.startswith("has_line "):
            continue
        fields = shlex.split(line)
        name = fields[1].removeprefix("$")
        if name not in variables:
            continue
        expected = fields[2]
        for variable, value in variables.items():
            expected = expected.replace("$" + variable, value)
        assert expected in variables[name].splitlines(), (name, expected)
        if name in checked:
            checked[name] += 1
    assert all(count > 1 for count in checked.values())
    assert stages[0][1] == variables["RUST_BASE"]
    assert all(stage[1] == variables["DEBIAN_BASE"] for stage in stages[1:])
    assert 'for target in "$FLY_STAGE" "$(final_stage "$IMAGE_DOCKERFILE")"' in source
    assert 'lacks_match "$target"' in source
    for _, _, lines in stages[1:]:
        assert not re.search(r"X_WEBSEARCH_[A-Z]+_KEY_FILE|\.key( |$)|config\.json /", "\n".join(lines))
    assert [line for line in stages[-1][2] if line.startswith("USER ")][-1] == "USER 65532:65532"
    fly_init = variables["FLY_INIT_TEXT"]
    assert 'drop=(setpriv --reuid=65532 --regid=65532 --clear-groups --inh-caps=-all --no-new-privs)' in fly_init
    assert '"${drop[@]}" /usr/local/bin/x-websearch "$@" &' in fly_init
    assert '"${drop[@]}" socat ' in fly_init
    assert '[ "$LAST_USER" = "USER 65532:65532" ] || fail' in source
    # Deployment consumer facts are parsed, not inferred from string mentions.
    unit = configparser.ConfigParser(interpolation=None, strict=False)
    unit.read(ROOT / "services/interop/deploy/x-websearch/x-websearch.service")
    assert unit["Service"]["User"] == "x-websearch"
    assert unit["Service"]["NoNewPrivileges"] == "yes"
    assert unit["Service"]["ExecStart"] == "/usr/local/bin/x-websearch --config /etc/x-websearch/config.json"
    node = docker_stages(ROOT / "docker/layerx/Dockerfile")
    assert any(stage[0] == "x-websearch" for stage in node)
    assert "COPY --from=x-websearch /src/.x-websearch-target/release/x-websearch /usr/bin/x-websearch" in node[-1][2]


def test_xweb_packaging_fields_walk_actual_receipt_key_arrays_and_loader_refusals():
    embedded = dict(embedded_python(PACKAGING))["PY"]
    namespace = functions(embedded, {"pointer_get", "pointer_set", "placeholder_cases"}, {"copy": copy})
    example, valid = json.loads(EXAMPLE.read_text()), json.loads(VALID.read_text())
    cases = namespace["placeholder_cases"](example)
    get, put = namespace["pointer_get"], namespace["pointer_set"]
    assert len(cases) == 16
    pointers = {pointer for pointer, _, _ in cases}
    assert not any("sequencer_" in pointer or "kernel_network" in pointer for pointer in pointers)
    for index in range(len(example["gateway"]["receipt_keys"])):
        for field in ("key_id", "public_key", "first_height", "last_height"):
            assert f"/gateway/receipt_keys/{index}/{field}" in pointers
    # Every placeholder currently present in the example has an exercised case.
    def leaves(value, pointer=""):
        if isinstance(value, dict):
            return [leaf for key, child in value.items() for leaf in leaves(child, pointer + "/" + key)]
        if isinstance(value, list):
            return [leaf for index, child in enumerate(value) for leaf in leaves(child, pointer + "/" + str(index))]
        return [(pointer, value)]
    for pointer, value in leaves(example):
        if pointer == "/note":
            continue
        if isinstance(value, str) and value.startswith("REPLACE_WITH"):
            assert pointer in pointers or pointer.startswith("/seeds/") and "/seeds" in pointers
    for pointer, field, refusal in cases:
        config = copy.deepcopy(valid)
        put(config, pointer, get(example, pointer))
        assert get(config, pointer) == get(example, pointer)
        put(config, pointer, get(valid, pointer))
        assert config == valid
        assert refusal == ("invalid" if pointer.endswith("height") else "a placeholder")
        if "/receipt_keys/" in pointer:
            assert re.fullmatch(r"gateway\.receipt_keys\[\d+\]\.\w+", field)
    # Multiple entries are traversed rather than hard-coding key zero.
    two = copy.deepcopy(example)
    two["gateway"]["receipt_keys"].append(copy.deepcopy(two["gateway"]["receipt_keys"][0]))
    assert len(namespace["placeholder_cases"](two)) == len(cases) + 4
    for fragment in ('[loader, "--config", path]', 'if code != 2 or stderr != message:',
                     'expect(baseline, "valid", "key refused: X_WEBSEARCH_RECEIVER_KEY_FILE is not set")',
                     'pointer_set(config, pointer, pointer_get(example, pointer))',
                     'f"configuration refused: {field} is {refusal}"',
                     'expect(config, "note", "configuration refused: note is not a known field")',
                     'expect(config, "filled", "key refused: X_WEBSEARCH_RECEIVER_KEY_FILE is not set")'):
        assert fragment in embedded
    loader = LOADER.read_text()
    gateway = loader[loader.index("fn gateway("):loader.index("fn payment(")]
    assert 'let entry_path = format!("{path}[{index}]")' in gateway
    assert 'entry.integer("first_height")?' in gateway and 'entry.integer("last_height")?' in gateway
    integer = loader[loader.index("    fn integer("):loader.index("    fn finish(")]
    assert ".as_u64()" in integer and "Refusal::Invalid" in integer
    assert 'Self::Invalid => "invalid"' in loader
    assert 'Self::Placeholder => "a placeholder"' in loader


def test_repair_record_maps_findings_and_limits_evidence():
    record = RECORD.read_text()
    for finding, name in {
        "xweb_synthetic_lifecycle": "test_xweb_synthetic_lifecycle_native_request_restart_and_callback_guards",
        "xweb_dry_run_config": "test_xweb_dry_run_config_uses_native_genesis_windows_and_production_loader_fields",
        "xweb_packaging_stages": "test_xweb_packaging_stages_guards_match_build_fly_and_default_runtime",
        "xweb_packaging_fields": "test_xweb_packaging_fields_walk_actual_receipt_key_arrays_and_loader_refusals",
    }.items():
        assert f"[finding.{finding}]" in record
        assert name in record
    assert "review_only" in record and "wiring" in record and "baseline" in record
    assert "not_run" in record
