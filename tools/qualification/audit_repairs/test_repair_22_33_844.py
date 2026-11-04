"""Qualify the live governance inputs and production bridge-name guard.

hf-build owns the declared bash -n command. This Python-only gate parses shell
arguments and JSON templates, executes the embedded production fee calculation
and calls the production output guard against real files. It runs no toolchain.
"""

import ast
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import re
import runpy
import shlex
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
GOV = ROOT / "tools/paxeer-x/gov"
RECORD = Path(__file__).with_name("repair_22_33_844.kvx")
CONFIG = ROOT / "services/bridge/deploy/proposals/testdata/solana.json"
LIVE_REQUIRED = {
    "out", "height", "release-url", "release-sha256", "deposit", "pax-usd",
    "kernel-network-id",
}
LIVE_OPTIONAL = {
    "rate-update-height", "bridge-config", "bridge-manifest", "paxd", "from",
    "chain-id",
}
RETIRED = (
    "pax-asset-id", "sid-asset-id", "sid-pointer", "market-pax-sid",
    "market-btc-usd", "market-eth-usd", "perp-margin", "custody-network-id",
    "deposit-authority", "sequencer-id", "sequencer-pubkey", "first-batch",
    "last-batch",
)
TEMPLATES = {
    "01-software-upgrade.json", "02-fee-token-params.json", "03-xweb-fee.json",
}


def source(name):
    return (GOV / name).read_text(encoding="utf-8")


def logical(text):
    return text.replace("\\\n", " ")


def test_f005_readme_lists_live_templates_bridge_outputs_and_inputs():
    readme = source("README.md")
    renderer = source("render.sh")
    templates = {p.name: json.loads(p.read_text()) for p in (GOV / "templates").glob("*.json")}
    assert set(templates) == TEMPLATES
    rows = re.findall(r"^\| `([^`]+)` \|", readme, re.M)
    assert set(rows) == TEMPLATES | {"bridge/<name>/"}
    example = re.findall(r"```text\n(.*?)\n```", readme, re.S)
    assert len(example) == 1
    documented = set(re.findall(r"--([a-z][a-z0-9-]*)", example[0]))
    parsed = set(re.findall(r"^    --([a-z][a-z0-9-]*)\)", renderer, re.M))
    assert documented == parsed == LIVE_REQUIRED | LIVE_OPTIONAL
    for flag in RETIRED:
        assert "--" + flag not in readme
    assert not re.search(r"custody|exchange-market|anchor|sequencer", readme, re.I)
    for fact in (
        "v6.12.0", "rate_update_height", "3114000", "3400000",
        "kernel_network_id", "program_fee_denoms", "<file>.tx.json",
        "proposals/04-proposal-open-chain.json", "proposals/05-proposal-sidiora-cap.json",
        "Solana", "ASCII", "Paths and traversal names are refused",
        "--generate-only --offline", "exactly one message",
    ):
        assert fact in readme
    assert templates["01-software-upgrade.json"]["name"] == "v6.12.0"


def test_f003_renderer_requires_only_supported_inputs():
    renderer = source("render.sh")
    required = re.findall(r"^for name in (.*?); do\n  \[ -n", renderer, re.M)
    assert len(required) == 1
    assert {name.lower().replace("_", "-") for name in required[0].split()} == LIVE_REQUIRED
    assert 'set -euo pipefail' in renderer
    assert '[ $# -ge 2 ] || usage' in renderer
    assert '*) usage ;;' in renderer
    for flag in RETIRED:
        assert "--" + flag not in renderer
        assert flag.upper().replace("-", "_") not in renderer
    for retired in ("ANCHOR_CALLDATA", "setSequencerAuthorization", "0xc2661383", "PERP_MARGIN_ASSET_ID"):
        assert retired not in renderer
    for guarded in (
        'RATE_UPDATE_HEIGHT=${RATE_UPDATE_HEIGHT:-$HEIGHT}',
        '[[ $HEIGHT =~ ^[1-9][0-9]*$ ]] || die',
        '[[ $RATE_UPDATE_HEIGHT =~ ^[1-9][0-9]*$ ]] || die',
        '[[ $KERNEL_NETWORK_ID =~ ^[1-9][0-9]*$ && ${#KERNEL_NETWORK_ID} -le 10 && $KERNEL_NETWORK_ID -le 4294967295 ]]',
        '[[ $RELEASE_SHA256 =~ ^[0-9a-f]{64}$ ]] || die',
        'command -v "$PAXD" >/dev/null || die',
    ):
        assert guarded in renderer


def test_f003_live_variables_cover_every_real_template_placeholder():
    renderer = source("render.sh")
    args = renderer.split('VARS=$(jq -n \\\n', 1)[1].split("  '$ARGS.named')", 1)[0]
    tokens = shlex.split(logical(args))
    assert len(tokens) % 3 == 0
    bindings = {}
    kinds = {}
    for kind, key, value in zip(tokens[::3], tokens[1::3], tokens[2::3]):
        assert kind in {"--arg", "--argjson"}
        assert key not in bindings
        bindings[key], kinds[key] = value, kind
    placeholders = set()
    for path in (GOV / "templates").glob("*.json"):
        document = json.loads(path.read_text())
        placeholders.update(re.findall(r'"@([A-Z0-9_]+)@"', json.dumps(document)))
    assert set(bindings) == placeholders
    assert bindings == {
        "HEIGHT": "$HEIGHT", "UPGRADE_INFO": "$UPGRADE_INFO", "DEPOSIT": "$DEPOSIT",
        "FEE_TOKEN_RATE": "3114000", "RATE_UPDATE_HEIGHT": "$RATE_UPDATE_HEIGHT",
        "MAX_FEE_TOKEN_RATE_AGE": "3400000", "GOV_AUTHORITY": "$GOV_AUTHORITY",
        "XWEB_FEE": "$XWEB_FEE", "XWEB_MAX_PAYLOAD_BYTES": "8192",
        "XWEB_MAX_CALLBACK_GAS": "500000", "XWEB_TIMEOUT_BLOCKS": "3600",
        "XWEB_KERNEL_NETWORK_ID": "$KERNEL_NETWORK_ID",
        "XWEB_PROGRAM_FEE_DENOMS": '["uhpx","usid","uusdc","uusdl"]',
    }
    assert {key for key, kind in kinds.items() if kind == "--argjson"} == {
        "XWEB_MAX_PAYLOAD_BYTES", "XWEB_KERNEL_NETWORK_ID", "XWEB_PROGRAM_FEE_DENOMS",
    }
    assert '{binaries: {"linux/amd64": ($url + "?checksum=sha256:" + $sha)}}' in renderer
    assert 'walk(if type == "string" and test("^@[A-Z0-9_]+@$")' in renderer
    assert 'if ($v | has($k)) then $v[$k] else error("unset placeholder \\($k)") end' in renderer
    fee = json.loads(source("templates/02-fee-token-params.json"))
    assert fee["changes"] == [
        {"subspace": "evm", "key": "KeyFeeTokenEnabled", "value": True},
        {"subspace": "evm", "key": "KeyAllowedFeeDenoms", "value": [
            {"denom": "usid", "rate": "@FEE_TOKEN_RATE@", "rate_update_height": "@RATE_UPDATE_HEIGHT@"},
        ]},
        {"subspace": "evm", "key": "KeyMaxFeeTokenRateAge", "value": "@MAX_FEE_TOKEN_RATE_AGE@"},
    ]
    xweb = json.loads(source("templates/03-xweb-fee.json"))
    assert xweb["@type"] == "/paxprotocol.paxchain.layerxgov.LayerXProposal"
    assert len(xweb["messages"]) == 1
    assert xweb["messages"][0]["@type"] == "/paxprotocol.paxchain.xweb.MsgSetParams"
    assert xweb["messages"][0]["kernel_network_id"] == "@XWEB_KERNEL_NETWORK_ID@"
    assert xweb["messages"][0]["program_fee_denoms"] == "@XWEB_PROGRAM_FEE_DENOMS@"


@pytest.mark.parametrize("price, expected", [("0.02", "50000"), ("0.0064", "156250"), ("400", "3")])
def test_f003_embedded_production_fee_calculation(price, expected):
    code = source("render.sh").split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]
    arguments, output = sys.argv, io.StringIO()
    try:
        sys.argv = ["-", price]
        with redirect_stdout(output):
            exec(compile(code, "render.sh:xweb-fee", "exec"), {})
    finally:
        sys.argv = arguments
    assert output.getvalue() == expected + "\n"


@pytest.mark.parametrize("price", ["0", "-1", "NaN", "Infinity", "garbage", "2001"])
def test_f003_embedded_production_fee_refuses_invalid_prices(price):
    code = source("render.sh").split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]
    arguments, output = sys.argv, io.StringIO()
    try:
        sys.argv = ["-", price]
        with redirect_stdout(output), pytest.raises(SystemExit) as refusal:
            exec(compile(code, "render.sh:xweb-fee", "exec"), {})
        assert refusal.value.code
    finally:
        sys.argv = arguments
    assert output.getvalue() == ""


def test_f003_guarded_producer_transport_and_paxd_consumer():
    renderer = logical(source("render.sh"))
    for command in (
        'if ! "$PAXD" "$@" --generate-only --offline --chain-id "$CHAIN_ID" --from "$FROM"',
        '--home "$HOME_DIR" --output json >"$file.tx.json" 2>"$file.err"; then',
        'die "paxd refused $(basename "$file")"',
        "jq -e '.body.messages | length == 1'",
        'dry_run "$up" tx gov submit-proposal software-upgrade',
        'dry_run "$OUT/02-fee-token-params.json" tx gov submit-proposal param-change "$OUT/02-fee-token-params.json"',
        'dry_run "$OUT/03-xweb-fee.json" tx gov submit-proposal layerx-proposal "$OUT/03-xweb-fee.json" --deposit "$DEPOSIT"',
        'dry_run "$file" tx gov submit-proposal layerxbridge-proposal "$file" --deposit "$DEPOSIT"',
    ):
        assert command in renderer
    ordered = (
        'OUT=$(python3 "$OUTPUT_GUARD" prepare "$OUT")',
        'for template in "$TEMPLATES"/*.json; do',
        'dry_run "$up"',
        'for config in "${BRIDGE_CONFIGS[@]}"; do',
        'dir=$(python3 "$OUTPUT_GUARD" bridge "$OUT" "$config")',
        'stage=$(mktemp -d "$OUT/.bridge-render.XXXXXXXX")',
        'go run ./services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals -manifest "$BRIDGE_MANIFEST"',
        '-proposals "$stage/proposals" "$config" "$stage/bodies"',
        'python3 "$OUTPUT_GUARD" publish "$OUT" "$dir" "$stage"',
        'dry_run "$file" tx gov submit-proposal layerxbridge-proposal',
    )
    offsets = [renderer.index(command) for command in ordered]
    assert offsets == sorted(offsets)
    assert 'GOV_AUTHORITY=pax10d07y265gmmuvt4z0w9aw880jnsr700jxdwa9m' in renderer
    assert 'CHAIN_ID=hyperpax_125-1' in renderer
    assert 'PAXD=${PAXD:-paxd}' in renderer


def test_f063_shell_test_uses_live_inputs_real_paxd_and_preserves_assertions():
    script = source("test.sh")
    for flag in RETIRED:
        assert "--" + flag not in script
    assert 'set -euo pipefail' in script
    assert 'go build -tags netgo -o "$WORK/paxd" ./daemon/paxd' in script
    assert 'PAXD=$WORK/paxd' in script
    args = shlex.split(script.split('RENDER_ARGS=(\n', 1)[1].split('\n)', 1)[0])
    assert len(args) % 2 == 0
    supplied = dict(zip(args[::2], args[1::2]))
    assert set(supplied) == {"--" + flag for flag in LIVE_REQUIRED - {"out"}} | {"--paxd", "--bridge-manifest"}
    assert supplied["--paxd"] == "$PAXD"
    assert supplied["--kernel-network-id"] == "125"
    assert '"$ROOT/tools/paxeer-x/gov/render.sh" --out "$OUT" "${RENDER_ARGS[@]}"' in script
    for assertion in (
        'sid_docs_name_v6120\n', 'release_files_name_v6120: ok',
        '[ -s "$OUT/$f.json" ] || fail', '[ -s "$OUT/$f.json.tx.json" ] || fail',
        '.body.messages[0]["@type"] == "/cosmos.gov.v1beta1.MsgSubmitProposal"',
        'plan_named_v6120\n', '.height == "27000000"',
        'KeyFeeTokenEnabled', 'KeyAllowedFeeDenoms', 'KeyMaxFeeTokenRateAge',
        '.messages[0].fee == "50000"', '.messages[0].kernel_network_id == 125',
        '.messages[0].program_fee_denoms == ["uhpx","usid","uusdc","uusdl"]',
        'bridge/solana/proposals/04-proposal-open-chain',
        'bridge/solana/proposals/05-proposal-sidiora-cap',
    ):
        assert assertion in script


def test_f063_shell_traversal_cases_require_specific_refusal_and_preserve_outside():
    script = logical(source("test.sh"))
    body = script.split('bridge_names_refuse_traversal() {', 1)[1].split('\n}', 1)[0]
    names = shlex.split(body.split('for name in ', 1)[1].split('; do', 1)[0])
    assert names == [
        '../escape', '../../escape', 'solana/../../escape', 'solana/child',
        'solana\\child', '$WORK/escape', '.', '..', '',
    ]
    for command in (
        "jq --arg name \"$name\" '.name = $name'",
        'if "$ROOT/tools/paxeer-x/gov/render.sh" --out "$output" "${RENDER_ARGS[@]}"',
        '--bridge-config "$config" >"$WORK/refused.log" 2>&1; then',
        'fail "bridge_names_refuse_traversal: accepted $name"',
        "grep -Fq 'bridge name must be'",
        '[ ! -e "$output/bridge" ] || fail',
        '[ "$(cat "$WORK/escape/sentinel")" = \'operator data must survive\' ]',
        '[ "$(ls -A "$WORK/escape")" = sentinel ]',
    ):
        assert command in body
    assert '\nbridge_names_refuse_traversal\n' in script


@pytest.mark.parametrize("name", ["solana", "Ethereum-main_1", "125"])
def test_f063_production_guard_accepts_live_bridge_names(tmp_path, name):
    guard = runpy.run_path(str(GOV / "output.py"))
    config = json.loads(CONFIG.read_text())
    config["name"] = name
    path = tmp_path / "config.json"
    path.write_text(json.dumps(config))
    output = guard["prepare_output"](tmp_path / "out")
    stdout = io.StringIO()
    with redirect_stdout(stdout):
        guard["main"](["bridge", str(output), str(path)])
    destination = output / "bridge" / name
    assert stdout.getvalue() == str(destination) + "\n"
    assert destination.is_dir() and destination.resolve().is_relative_to(output)
    assert json.loads(path.read_text()) == config


@pytest.mark.parametrize("name", [
    "../escape", "../../escape", "solana/../../escape", "solana/child",
    "solana\\child", "/escape", ".", "..", "", "chain\n", None, 125,
])
def test_f063_production_guard_refuses_traversal_before_creating_outputs(tmp_path, name):
    guard = runpy.run_path(str(GOV / "output.py"))
    output = guard["prepare_output"](tmp_path / "out")
    outside = tmp_path / "escape"
    outside.mkdir()
    sentinel = outside / "sentinel.json"
    sentinel.write_bytes(CONFIG.read_bytes())
    config = json.loads(CONFIG.read_text())
    config["name"] = name
    path = tmp_path / "config.json"
    path.write_text(json.dumps(config))
    stdout = io.StringIO()
    with redirect_stdout(stdout), pytest.raises(ValueError, match="bridge name must be"):
        guard["main"](["bridge", str(output), str(path)])
    assert stdout.getvalue() == ""
    assert list(output.iterdir()) == []
    assert list(outside.iterdir()) == [sentinel]
    assert sentinel.read_bytes() == CONFIG.read_bytes()


def test_record_maps_each_finding_to_executed_proofs():
    record = tomllib.loads(RECORD.read_text())
    audit = tomllib.loads((ROOT / 'spec/layerx-merge/programme/audit/records/22.30.181.kvx').read_text())
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith('test_')
    }
    assert set(record['finding']) == {'f003', 'f005', 'f063'}
    mapped = set()
    for finding, details in record['finding'].items():
        for key in ('file', 'line', 'class', 'severity', 'owner_domain'):
            assert details[key] == audit['finding'][finding][key]
        assert details['audit'] == '22.30.181'
        assert details['tests'] and len(details['tests']) == len(set(details['tests']))
        assert set(details['tests']) <= declared
        assert any(name.startswith('test_' + finding + '_') for name in details['tests'])
        mapped.update(details['tests'])
    assert mapped == declared - {'test_record_maps_each_finding_to_executed_proofs'}
    task = record['task']['22']['33']['844']
    assert task['wiring_requests'] == []
    assert task['evidence_ledger'] == '/out/run/gates.jsonl'
    assert task['verify_cmd'] == 'python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_844.py'
    assert task['build_cmd'] == 'bash -n tools/paxeer-x/gov/render.sh tools/paxeer-x/gov/test.sh'
    assert 'No pre-change verification was run' in task['baseline']
    assert 'not executed' in task['review_only']
