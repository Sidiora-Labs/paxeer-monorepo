"""Offline source and production readiness-validator proof for finding 025."""

import ast
import json
from pathlib import Path
import re
import shlex
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
PROBE = ROOT / "services/platform/hosted/agentd/probe.sh"
SOURCE = PROBE.read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1409.kvx")


def curl_commands(source):
    return [
        shlex.split(command.replace("\\\n", " "))
        for command in re.findall(r"^ *curl .*?(?<!\\)\n", source, re.M | re.S)
    ]


def test_025_default_help_and_success_claim_only_health_authentication():
    help_text = SOURCE.split("cat <<'USAGE'\n", 1)[1].split("\nUSAGE", 1)[0]
    assert "The default journey proves daemon health and authentication only" in help_text
    assert (
        "health never\nproves verified tenant reads or tenant write admission"
        in help_text
    )
    assert "default read journey proves transport and verified reads" not in help_text
    tail = SOURCE.rsplit("\nprintf 'agentd-probe: %s ready", 1)[1]
    assert shlex.split(tail.split("\n", 1)[1]) == shlex.split(
        """if [ "$journey" = write ]; then
    printf 'agentd-probe: authenticated tenant transport and verified reads ready; tenant writes admitted\\n'
elif [ "$tenant_probe" = 1 ]; then
    printf 'agentd-probe: authenticated tenant transport and verified reads ready; no tenant write claim\\n'
else
    printf 'agentd-probe: daemon health and authentication only; no tenant read or write claim\\n'
fi
"""
    )
    assert SOURCE.count(
        "printf 'agentd-probe: authenticated tenant transport and verified reads ready;"
    ) == 2
    assert "transport and verified reads only" not in SOURCE


def test_025_health_claim_follows_authenticated_health_and_rejection_guards():
    health = SOURCE.split("\nstatus=0\ncurl ", 1)[1]
    health = "curl " + health.split('\nif [ "$tenant_probe" = 1 ]; then', 1)[0]
    authenticated, anonymous, no_certificate = curl_commands(health)
    for command in (authenticated, anonymous, no_certificate):
        assert command[command.index("--cacert") + 1] == "$ca"
        assert "$url/healthz" in command
    for command in (authenticated, anonymous):
        assert command[command.index("--cert") + 1] == "$client_cert"
        assert command[command.index("--key") + 1] == "$client_key"
    assert authenticated[authenticated.index("--config") + 1] == "$work/bearer.conf"
    assert "--config" not in anonymous
    assert "--cert" not in no_certificate and "--key" not in no_certificate
    assert no_certificate[no_certificate.index("--config") + 1] == "$work/bearer.conf"
    for guard in (
        'if [ "$status" != 0 ]; then',
        'if [ "$(cat "$work/ready.code")" != 200 ]; then',
        "if ! grep -q '\"ready\":true' \"$work/ready.json\"; then",
        'if ! kill -0 "$stalled" 2>/dev/null; then',
        'if [ "$(cat "$work/anonymous.code")" != 401 ]; then',
        'if [ "$status" = 0 ]; then',
    ):
        refusal = health.split(guard, 1)[1].split("\nfi", 1)[0]
        assert "exit 1" in refusal, guard
    assert "set -eu" in SOURCE.splitlines()[:3]


def test_025_verified_read_claim_requires_credentialed_tenant_journey():
    setup = SOURCE.split("tenant_probe=0\n", 1)[1].split("\nwork=", 1)[0]
    assert setup.startswith(
        'if [ "$journey" = write ] || [ -n "$rpc_url$gateway_key_file$credential_file" ]; then\n'
    )
    refusal = setup.split(
        'if [ -z "$rpc_url" ] || [ -z "$gateway_key_file" ] || [ -z "$credential_file" ]; then',
        1,
    )[1]
    assert "exit 2" in refusal.split("\n    fi", 1)[0]
    assert setup.endswith("    tenant_probe=1\nfi\n")
    assert SOURCE.count("tenant_probe=1") == 1
    assert "'operation': 'tenant.readiness'" in SOURCE
    assert "'request': {}, 'credential': credential" in SOURCE
    assert "Authorization: LayerX-Key " in SOURCE
    tenant = SOURCE.rsplit('\nif [ "$tenant_probe" = 1 ]; then\n', 1)[1]
    (command,) = curl_commands(tenant)
    for option, value in (
        ("--cacert", "$ca"),
        ("--cert", "$client_cert"),
        ("--key", "$client_key"),
        ("--config", "$work/gateway.conf"),
        ("--header", "Content-Type: application/json"),
        ("--data-binary", "@$work/tenant-request.json"),
        ("--output", "$work/tenant-readiness.json"),
    ):
        assert command[command.index(option) + 1] == value
    assert "$rpc_url" in command
    for guard in (
        'if [ "$status" != 0 ]; then',
        'if [ "$(cat "$work/tenant-readiness.code")" != 200 ]; then',
    ):
        assert "exit 1" in tenant.split(guard, 1)[1].split("\n    fi", 1)[0]
    assert tenant.index("<<'PY'") < tenant.index("verified reads ready")
    assert "\nPY\nfi\n\nprintf 'agentd-probe: %s ready" in tenant


@pytest.mark.parametrize(
    "journey,transport,reads,writes,reason,error",
    [
        ("read", True, True, True, None, None),
        ("write", True, True, True, None, None),
        ("read", True, True, False, "recovery_pending", None),
        ("write", True, True, False, "recovery_pending", "tenant writes are not admitted"),
        (
            "read", True, False, False,
            "verified_read_unavailable", "verified reads are unavailable",
        ),
        (
            "read", False, False, False,
            "transport_unavailable", "verified reads are unavailable",
        ),
        ("write", True, False, True, None, "flags contradict the owner contract"),
    ],
)
def test_025_production_validator_gates_tenant_read_and_write_claims(
    tmp_path, monkeypatch, journey, transport, reads, writes, reason, error
):
    response = tmp_path / "tenant-readiness.json"
    response.write_text(
        json.dumps(
            {
                "request_id": "1",
                "verification_status": {"state": "achieved", "level": "Unverified"},
                "value": {
                    "transport_ready": transport,
                    "verified_reads_ready": reads,
                    "writes_admitted": writes,
                    "recovery_reason": reason,
                },
            }
        ),
        encoding="utf-8",
    )
    validator = SOURCE.rsplit("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]
    program = compile(validator, str(PROBE), "exec")
    monkeypatch.setattr(sys, "argv", ["-", str(response), journey])
    if error is None:
        exec(program, {"__name__": "__main__"})
    else:
        with pytest.raises(SystemExit, match=error):
            exec(program, {"__name__": "__main__"})


def test_025_record_maps_finding_to_proving_tests_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["025"]
    assert finding["audit"] == "22.30.79"
    assert finding["file"] == "services/platform/hosted/agentd/probe.sh"
    assert finding["wiring_requests"] == []
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_025_record_maps_finding_to_proving_tests_and_evidence"
    }
    task = record["task"]["22"]["33"]["1409"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1409.py"
    )
    assert task["build_cmd"] == "bash -n services/platform/hosted/agentd/probe.sh"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["baseline"]
