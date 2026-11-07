import re
import shlex
from pathlib import Path


def test_probe_calls_use_rpc():
    root = Path(__file__).resolve().parents[3]
    script = (root / "tools/bringup/value-loop.sh").read_text(encoding="utf-8")
    probe = (
        root / "services/platform/hosted/node/tests/probe/src/main.rs"
    ).read_text(encoding="utf-8")

    logical_lines = script.replace("\\\n", " ")
    calls = [
        match.group(1)
        for line in logical_lines.splitlines()
        if not line.strip().startswith("for tool in ")
        for match in re.finditer(r"\blayerx-node-probe\s+([^\n]+)", line)
    ]
    assert calls, "No probe invocations found"
    commands = set()
    for call in calls:
        arguments = shlex.split(call)
        commands.add(arguments[0])
        assert "--socket" not in arguments, call
        for option, value in (
            ("--rpc", "$rpc"),
            ("--trust-anchor", "$trust_anchor"),
            ("--network-id", "$network_id"),
        ):
            assert option in arguments, call
            assert arguments[arguments.index(option) + 1] == value, call
    assert commands == {"handshake", "balance", "$send_command"}
    assert "local send_command=write-send" in script
    assert re.search(
        r'if \[ "\$explicit_asset" = 1 \]; then\s+send_command=write-asset-send\s+fi',
        script,
    )

    assert "rpc=${LAYERX_NODE_RPC:-}" in script
    assert "trust_anchor=${LAYERX_NODE_TRUST_ANCHOR_DER:-}" in script
    first_balance = script.index('answer=$(balance "${dids[sender]}")')
    for variable, environment in (
        ("rpc", "LAYERX_NODE_RPC"),
        ("trust_anchor", "LAYERX_NODE_TRUST_ANCHOR_DER"),
    ):
        guard = re.search(
            rf'^\t\[ -n "\${variable}" \] \|\| precondition [^\n]*{environment}[^\n]*$',
            script,
            re.MULTILINE,
        )
        assert guard, f"Missing unconditional {environment} precondition"
        assert script.index("main() {") < guard.start() < first_balance

    handshake = re.search(r"handshake_committed\(\) \{(.*?)\n\}", script, re.DOTALL)
    assert handshake
    assert "jq -er '.latest_committed_height'" in handshake.group(1)
    assert "latest_sealed_batch" not in script
    assert "handshake_sealed" not in script
    assert "committed_height=$(handshake_committed)" in script
    assert 'echo "block hash=$block committed_height=$committed_height"' in script
    assert '"$SSL_CERT_FILE" "$network_id" <"$state/send.bin"' in script

    session = re.search(r"fn rpc_session\(.*?\n\}", probe, re.DOTALL)
    assert session
    assert 'let url = required(parsed, "rpc")?;' in session.group(0)
    assert 'required(parsed, "socket")' not in session.group(0)
    for retired in ("layerxd.lni.sock", "--socket", "receipt-authority", "LAYERX_AUTHORITY_LNI_SOCKET"):
        assert retired not in script
    assert 'committed-transaction "$rpc" "0x$activity"' in script
    assert '"$committed_height" -ge "$((inclusion_height))"' in script
    assert 'send_journal reconcile' in script
    assert 'eth_sendRawTransaction' in script
    assert 'eth_getTransactionByHash' in script
    assert 'SSL_CERT_FILE' in script
