import json
import os
import re
import shlex
import subprocess
import tempfile
from pathlib import Path
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[4]
FIXTURE = ROOT / "tools/qualification/localnode/v6120-node.sh"
MERGE_ADDRESSES = tuple(
    f"0x{suffix:040x}"
    for suffix in (*range(0x101A, 0x101E), *range(0x0901, 0x0909))
)


def eth_call(endpoint, address, data, request_id):
    request = Request(
        endpoint,
        data=json.dumps(
            {
                "jsonrpc": "2.0",
                "id": request_id,
                "method": "eth_call",
                "params": [{"to": address, "data": data}, "latest"],
            }
        ).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urlopen(request, timeout=15) as response:
        assert response.status == 200
        result = json.load(response)
    assert result["jsonrpc"] == "2.0", (address, result)
    assert result["id"] == request_id, (address, result)
    return result


def test_merge_precompiles_served():
    base = Path(os.environ.get("TMPDIR", "/tmp")) / "lx-localnode"
    base.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="merge-test.", dir=base) as directory:
        env_file = Path(directory) / "localnode.env"
        home = None
        try:
            started = subprocess.run(
                ["bash", str(FIXTURE), "--env-out", str(env_file)],
                cwd=ROOT,
                capture_output=True,
                text=True,
                timeout=300,
            )
            ready = re.search(
                r"^v6120-node: ready; home=(.+) env=(.+)$",
                started.stderr,
                re.MULTILINE,
            )
            if ready:
                home = ready.group(1)
            assert started.returncode == 0, started.stdout + started.stderr
            assert ready is not None, started.stderr
            assert Path(ready.group(2)) == env_file.resolve()
            environment = {}
            for line in env_file.read_text().splitlines():
                key, value = line.split("=", 1)
                parsed = shlex.split(value)
                assert len(parsed) == 1, line
                environment[key] = parsed[0]
            assert {
                "LOCAL_NODE_RPC",
                "LOCAL_NODE_KEY",
                "PERPS_DATA_TEST_RPC",
                "PERPS_DATA_TEST_COMET",
                "NEXT_PUBLIC_PERPS_GATEWAY_URL",
            } <= environment.keys()
            assert environment["PERPS_DATA_TEST_RPC"] == environment["LOCAL_NODE_RPC"]
            comet = environment["PERPS_DATA_TEST_COMET"]
            assert re.fullmatch(r"http://127\.0\.0\.1:[1-9][0-9]{0,4}", comet)
            assert 1 <= int(comet.rsplit(":", 1)[1]) <= 65535
            with urlopen(comet + "/status", timeout=15) as response:
                assert response.status == 200
                status = json.load(response)
            assert "error" not in status, status
            if "result" in status:
                status = status["result"]
            assert status["node_info"]["network"] == "hyperpax_125-1"
            for index, address in enumerate(MERGE_ADDRESSES, 1):
                result = eth_call(
                    environment["LOCAL_NODE_RPC"], address, "0xffffffff", index
                )
                assert "result" not in result, (address, result)
                assert result["error"]["code"] == 3, (address, result)
                assert any(
                    code in result["error"]["message"]
                    for code in (
                        "LXP_PRECOMPILE_UNKNOWN_METHOD",
                        "LXP_AGENT_NOT_ACTIVE",
                    )
                ), (address, result)
            result = eth_call(
                environment["LOCAL_NODE_RPC"],
                MERGE_ADDRESSES[0],
                "0x3f050cbc" + "11" * 32 + "00" * 32,
                len(MERGE_ADDRESSES) + 1,
            )
            assert "error" not in result, result
            assert re.fullmatch(r"0x[0-9a-fA-F]{64}", result["result"]), result
        finally:
            if home is not None:
                stopped = subprocess.run(
                    ["bash", str(FIXTURE), "--stop", home],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                assert stopped.returncode == 0, stopped.stdout + stopped.stderr
