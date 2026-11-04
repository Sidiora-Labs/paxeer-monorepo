"""f040 bootstrap proof; the out-of-scope publisher migration is recorded separately."""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
INIT = ROOT / "docker/gas-station/init.sh"
RECORD = Path(__file__).with_name("repair_22_33_885.kvx")


def bootstrap(volume, **overrides):
    """Run the exact production validation/render commands, before binary preflight.

    Only the directory binding changes: no production command or RPC peer is
    replaced. Each call starts and stops a new shell on the same test volume.
    """
    source = INIT.read_text()
    commands = source[source.index("# need <NAME>"):source.index("check=0\n")]
    env = {
        "PATH": "/usr/bin:/bin",
        "GAS_STATION_LISTEN": "[::]:8080",
        "GAS_STATION_CHAIN_ID": "125",
        "GAS_STATION_ENDPOINTS": "https://node-a.example https://node-b.example/",
        "GAS_STATION_PAYMASTER": "0x" + "44" * 20,
        "GAS_STATION_TOKEN": "0x21f7b20a555199fa73A238B1a91FD0f549068fEe",
        "GAS_STATION_DECIMALS": "6",
        "GAS_STATION_MAX_RATE_AGE": "300",
        "GAS_STATION_SPREAD_BPS": "500",
        "GAS_STATION_MARGIN_BPS": "100",
        "GAS_STATION_PER_ACCOUNT_LIMIT": "4000000",
        "GAS_STATION_PER_INTERVAL_LIMIT": "8000000",
        "GAS_STATION_PER_QUOTE_LIMIT": "3000000",
        "GAS_STATION_INTERVAL_SECONDS": "60",
        "GAS_STATION_BALANCE_FLOOR": "100",
        "GAS_STATION_GAS_LIMIT": "200000",
        "GAS_STATION_MAX_PRIORITY_FEE_PER_GAS": "1000000000",
        "GAS_STATION_DID_GAS_LIMIT": "60000",
        "GAS_STATION_RELAYER_KEY_ENV": "TEST_SPONSOR",
        "GAS_STATION_RATE_OWNER_KEY_ENV": "TEST_OWNER",
        "TEST_SPONSOR": "01" * 32,
        "TEST_OWNER": "02" * 32,
        "GAS_STATION_RATE_CADENCE_SECONDS": "120",
        "GAS_STATION_RATE_GAS_BUDGET_PER_DAY": "1000000",
        "GAS_STATION_RATE_BALANCE_FLOOR": "1000000000000000",
        "GAS_STATION_RATE_MAX_FEE_PER_GAS": "5000000000",
        "GAS_STATION_RATE_CONFIRMATION_RETRY_SECONDS": "60",
    }
    env.update(overrides)
    return subprocess.run(
        ["bash", "--noprofile", "--norc", "-c",
         'set -eu\numask 077\ndir="$1"\nconfig="$dir/station.json"\n' + commands,
         "bootstrap-proof", str(volume)],
        env=env, capture_output=True, text=True, timeout=10,
    )


def test_f040_shell_and_retired_authority_removed():
    parsed = subprocess.run(["bash", "-n", str(INIT)], capture_output=True, text=True)
    assert parsed.returncode == 0, parsed.stderr
    source = INIT.read_text()
    for retired in ("px_getReceiptKeys", "GAS_STATION_RECEIPT_KEYS",
                    "sequencer_receipt_keys", "refuse_receipt_keys", "curl"):
        assert retired not in source
    assert "set -eu\numask 077\ndir=/data/gas-station" in source


def test_f040_real_renderer_transports_only_node_configuration(tmp_path):
    result = bootstrap(tmp_path)
    assert result.returncode == 0, result.stderr
    station = json.loads((tmp_path / "station.json").read_text())
    rate = json.loads((tmp_path / "rate.json").read_text())
    assert station["chain_id"] == rate["chain_id"] == 125
    assert station["endpoints"] == rate["endpoints"] == [
        "https://node-a.example", "https://node-b.example/"
    ]
    shared = {k: v for k, v in station.items() if k not in {
        "listen", "gas_limit", "max_priority_fee_per_gas"
    }}
    assert {k: rate[k] for k in shared} == shared
    assert rate.keys() - shared.keys() == {
        "max_priority_fee_per_gas", "rate_owner_key_env", "rate_cadence_seconds",
        "rate_gas_budget_per_day", "rate_balance_floor", "rate_max_fee_per_gas",
        "rate_confirmation_retry_seconds",
    }
    assert rate["rate_owner_key_env"] == "TEST_OWNER"
    assert rate["relayer_key_env"] == station["relayer_key_env"] == "TEST_SPONSOR"
    for path in (tmp_path / "station.json", tmp_path / "rate.json"):
        assert path.stat().st_mode & 0o777 == 0o644
        assert "01" * 32 not in path.read_text()
        assert "02" * 32 not in path.read_text()
        assert not path.with_suffix(".json.new").exists()


@pytest.mark.parametrize("endpoint", [
    "https://host.example/rpc", "https://host.example/px",
    "https://host.example?token=x", "https://host.example/#fragment",
    "https://user@host.example", "http://host.example", "https:///",
    'https://host.example/"injection',
])
def test_f040_hosted_routes_cannot_be_node_rpc(tmp_path, endpoint):
    result = bootstrap(tmp_path, GAS_STATION_ENDPOINTS=endpoint)
    assert result.returncode == 2
    assert "GAS_STATION_ENDPOINTS is unset or malformed" in result.stderr
    assert not (tmp_path / "station.json").exists()
    assert not (tmp_path / "rate.json").exists()


def test_f040_bootstrap_restart_preserves_publisher_inputs(tmp_path):
    first = bootstrap(tmp_path)
    assert first.returncode == 0, first.stderr
    # Opaque retained files test bootstrap ownership of the volume, not a
    # fabricated successful publication or a substitute publisher journal.
    retained = {"rate.jsonl": b"retained publisher journal\n",
                "sponsorship.jsonl": b"retained station journal\n",
                "rate.toml": b"rate = 3114000\nset_at = 1700000000\n"}
    for name, content in retained.items():
        (tmp_path / name).write_bytes(content)
    before = {name: (tmp_path / name).stat().st_ino for name in retained}
    old_config = (tmp_path / "rate.json").read_bytes()
    second = bootstrap(tmp_path, GAS_STATION_RATE_CADENCE_SECONDS="100")
    assert second.returncode == 0, second.stderr
    assert (tmp_path / "rate.json").read_bytes() != old_config
    assert json.loads((tmp_path / "rate.json").read_text())["rate_cadence_seconds"] == 100
    for name, content in retained.items():
        with (tmp_path / name).open("rb") as recovered:
            assert recovered.read() == content
        assert (tmp_path / name).stat().st_ino == before[name]


def test_f040_preflight_and_persistent_consumer_commands_remain_guarded():
    source = INIT.read_text()
    check = '/usr/local/bin/paxeer-gas-station check --config "$config" --rate-config "$dir/rate.json" || check=$?'
    refusal = 'if [ "$check" -ne 0 ]; then'
    assert source.index(check) < source.index(refusal) < source.index('exit "$check"')
    assert source.index('exit "$check"') < source.index("chown -R 4020:4020")
    assert source.index('mv "$dir/rate.json.new" "$dir/rate.json"') < source.index(check)
    flat = source.replace("\\\n", " ")
    commands = re.findall(r'^env -u .*? &$', flat, re.M)
    assert len(commands) == 2
    station, publisher = [shlex.split(command) for command in commands]
    binary = "/usr/local/bin/paxeer-gas-station"
    assert station[:3] == ["env", "-u", "$GAS_STATION_RATE_OWNER_KEY_ENV"]
    assert publisher[:3] == ["env", "-u", "$GAS_STATION_RELAYER_KEY_ENV"]
    assert station[station.index(binary):] == [
        binary, "--config", "$config", "--journal", "$dir/sponsorship.jsonl", "&"
    ]
    assert publisher[publisher.index(binary):] == [
        binary, "rate", "--config", "$dir/rate.json", "--journal", "$dir/rate.jsonl",
        "--rate-file", "$dir/rate.toml", "&"
    ]
    assert 'trap \'kill -TERM "$station" "$publisher" 2>/dev/null\' TERM INT' in source
    assert 'wait -n "$station" "$publisher" || status=$?' in source
    assert 'kill -TERM "$station" "$publisher" 2>/dev/null || true\nwait || true\nexit "$status"' in source


def test_f040_record_names_proofs_and_external_publication_recovery():
    record = tomllib.loads(RECORD.read_text())
    assert record["meta"]["task"] == "22.33.885"
    audit = tomllib.loads((ROOT / record["meta"]["audit_record"]).read_text())
    assert record["finding"]["f040"]["path"] == audit["finding"]["f040"]["file"]
    functions = {node.name for node in ast.parse(Path(__file__).read_text()).body
                 if isinstance(node, ast.FunctionDef) and node.name.startswith("test_f040_")}
    assert set(record["finding"]["f040"]["tests"]) == functions - {
        "test_f040_record_names_proofs_and_external_publication_recovery"
    }
    assert "not run" in record["baseline"]["verification"]
    assert "not proven" in record["proof"]["publication_recovery"]
    assert record["wiring"]["publisher"]["owner_domain"] == 11
    assert record["wiring"]["publisher"]["path"] == "services/interop/crates/layerx-gas-station/src/rate.rs"
    assert "restart" in record["wiring"]["publisher"]["request"]
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
