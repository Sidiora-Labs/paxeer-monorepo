import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import re
import shlex
import sys
import threading
import time

import pytest
import yaml

from runner import CommandExecutionError, TestRunner as IntegrationRunner


@pytest.fixture
def rpc_peer():
    requests = []
    results = {"web3_clientVersion": "", "eth_blockNumber": "0x7d"}

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            assert self.path == "/"
            assert self.headers["Content-Type"] == "application/json"
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert request["jsonrpc"] == "2.0"
            assert request["params"] == []
            requests.append(request)
            response = json.dumps({
                "jsonrpc": "2.0", "id": request["id"],
                "result": results[request["method"]],
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/", results, requests
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=2)
        assert not worker.is_alive()


def python_command(source, *args):
    return [sys.executable, "-c", source, *args]


def rpc_command(url, method):
    return python_command(
        "import json,sys,urllib.request; "
        "body=json.dumps({'jsonrpc':'2.0','id':1,'method':sys.argv[2],"
        "'params':[]}).encode(); "
        "request=urllib.request.Request(sys.argv[1],data=body,"
        "headers={'Content-Type':'application/json'}); "
        "response=urllib.request.urlopen(request,timeout=2); "
        "result=json.load(response)['result']; "
        "print(int(result,16) if sys.argv[2]=='eth_blockNumber' else result)",
        url, method,
    )


@pytest.mark.parametrize("boundary", ["shell", "expression", "multiline"])
def test_python_interpolation_boundaries(tmp_path, rpc_peer, boundary):
    url, results, requests = rpc_peer
    marker = tmp_path / "injected"
    source = f"from pathlib import Path; Path({str(marker)!r}).write_text('owned')"
    payloads = {
        "shell": "'; " + shlex.join(python_command(source)) + "; #",
        "expression": (
            '" or __import__(\'pathlib\').Path('
            + repr(str(marker)) + ").write_text('owned') or \""
        ),
        "multiline": "quotes '\" \\ $HOME $(untrusted) `untrusted`\nsecond line",
    }
    payload = results["web3_clientVersion"] = payloads[boundary]
    suite = [{
        "name": "captured RPC output remains data",
        "inputs": [
            {"cmd": rpc_command(url, "web3_clientVersion"), "env": "CAPTURE"},
            {"cmd": python_command("import os; print(os.environ['CAPTURE'])"),
             "env": "ECHO"},
            {"cmd": rpc_command(url, "eth_blockNumber"), "env": "HEIGHT"},
        ],
        "verifiers": [
            {"type": "eval", "expr": "CAPTURE == ECHO and HEIGHT == 125"},
            {"type": "regex", "result": "ECHO", "expr": re.escape(payload) + "$"},
        ],
    }]
    suite_path = tmp_path / "suite.yaml"
    suite_path.write_text(yaml.safe_dump(suite))
    runner = IntegrationRunner(docker=False)
    runner.process_data(runner.load_yaml_file(suite_path))
    assert [request["method"] for request in requests] == [
        "web3_clientVersion", "eth_blockNumber",
    ]
    assert not marker.exists()


def test_docker_argv_keeps_captures_and_command_in_single_arguments(tmp_path):
    runner = IntegrationRunner()
    marker = tmp_path / "injected"
    source = f"from pathlib import Path; Path({str(marker)!r}).touch()"
    value = "'; " + shlex.join(python_command(source)) + "; #\n\"quoted\""
    container = "node; " + shlex.join(python_command(source))
    command = "printf '%s\\n' \"$CAPTURE\" | cat"
    argv = runner._command_argv(command, True, container, {"CAPTURE": value})
    transported = json.loads(runner.run_bash_command(python_command(
        "import json,sys; print(json.dumps(sys.argv[1:]))", *argv,
    )))
    assert transported == [
        "docker", "exec", "-e", "CAPTURE=" + value, "--", container,
        "/bin/bash", "-c",
        "export PATH=$PATH:/root/go/bin:/root/.foundry/bin && " + command,
    ]
    assert not marker.exists()


@pytest.mark.parametrize("expression", [
    "__import__('os').getcwd()", "VALUE.__class__", "[x for x in VALUE]",
    "(lambda: True)()", "f'{VALUE}'",
])
def test_expression_rejects_executable_syntax(expression):
    with pytest.raises(ValueError, match="Unsupported integration verifier"):
        IntegrationRunner().verify_result(
            {"VALUE": "data"}, {"type": "eval", "expr": expression},
        )


def test_bound_expression_values_preserve_numbers_and_precedence():
    runner = IntegrationRunner()
    assert runner.verify_result(
        {"AMOUNT": "5000000000333333333333", "FRACTION": "0.45", "N": "2"},
        {"type": "eval", "expr": (
            "AMOUNT == 5000000000333333333333 and FRACTION == 0.45 "
            "and N * 3 + 1 == 7 and (N == 2 or N == 3)"
        )},
    )
    assert not runner.verify_result(
        {"VALUE": '" or True or "'}, {"type": "eval", "expr": 'VALUE == "ready"'},
    )
    assert runner.verify_result(
        {"VALUE": "nan"}, {"type": "eval", "expr": 'VALUE == "nan"'},
    )


@pytest.mark.parametrize("expected", [None, 7])
def test_python_status_timeout_stops_setup_on_unexpected_exit(
    tmp_path, capsys, expected,
):
    marker = tmp_path / "later-setup"
    first = {"cmd": python_command(
        "import sys; print('partial'); "
        "sys.stderr.write('private failure diagnostics\\n'); sys.exit(4)",
    ), "env": "FAILED"}
    if expected is not None:
        first["expected_status"] = expected
    later = {"cmd": python_command(
        "from pathlib import Path; import sys; Path(sys.argv[1]).touch()",
        str(marker),
    )}
    runner = IntegrationRunner(docker=False)
    with pytest.raises(CommandExecutionError) as failure:
        runner.process_data([
            {"name": "failed setup", "inputs": [first, later], "verifiers": []},
            {"name": "later case", "inputs": [later], "verifiers": []},
        ])
    assert failure.value.returncode == 4
    assert failure.value.stdout == "partial\n"
    assert failure.value.stderr == "private failure diagnostics\n"
    assert not failure.value.timed_out
    assert "private failure diagnostics" not in str(failure.value)
    captured = capsys.readouterr()
    assert "private failure diagnostics" not in captured.err
    assert "later case" not in captured.out
    assert not marker.exists()


def test_explicit_negative_status_is_verified_and_setup_continues(tmp_path):
    marker = tmp_path / "continued"
    runner = IntegrationRunner(docker=False)
    runner.run_test({
        "name": "explicit rejection",
        "inputs": [
            {"cmd": python_command("import sys; print('rejected'); sys.exit(7)"),
             "expected_status": 7, "env": "RESULT"},
            {"cmd": python_command(
                "from pathlib import Path; import sys; Path(sys.argv[1]).touch()",
                str(marker),
            )},
        ],
        "verifiers": [{"type": "eval", "expr": 'RESULT == "rejected"'}],
    })
    assert marker.exists()


def test_python_status_timeout_kills_process_group_and_stops_setup(tmp_path, capsys):
    marker = tmp_path / "later-setup"
    descendant = "import time; time.sleep(30)"
    hung = python_command(
        "import subprocess,sys,time; "
        "child=subprocess.Popen([sys.executable,'-c',sys.argv[1]]); "
        "print(child.pid,flush=True); "
        "sys.stderr.write('private timeout diagnostics\\n'); sys.stderr.flush(); "
        "time.sleep(30)", descendant,
    )
    runner = IntegrationRunner(timeout=5, docker=False)
    started = time.monotonic()
    with pytest.raises(CommandExecutionError) as failure:
        runner.run_test({
            "name": "bounded setup",
            "inputs": [
                {"cmd": hung, "timeout": 0.5},
                {"cmd": python_command(
                    "from pathlib import Path; import sys; Path(sys.argv[1]).touch()",
                    str(marker),
                )},
            ],
            "verifiers": [],
        })
    assert time.monotonic() - started < 3
    assert failure.value.timed_out
    assert failure.value.returncode == -9
    assert failure.value.stderr == "private timeout diagnostics\n"
    descendant_pid = int(failure.value.stdout.strip())
    stat = Path(f"/proc/{descendant_pid}/stat")
    # A killed orphan may await reaping by the sandbox init process.
    if stat.exists():
        assert stat.read_text().split(") ", 1)[1].split()[0] == "Z"
    assert not marker.exists()
    assert "private timeout diagnostics" not in str(failure.value)
    assert "private timeout diagnostics" not in capsys.readouterr().err


@pytest.mark.parametrize("timeout", [0, -1, float("inf"), float("nan"), True, "1"])
def test_invalid_deadlines_are_rejected_before_launch(timeout, tmp_path):
    marker = tmp_path / "launched"
    command = python_command(
        "from pathlib import Path; import sys; Path(sys.argv[1]).touch()", str(marker),
    )
    with pytest.raises(ValueError, match="positive finite"):
        IntegrationRunner().run_bash_command(command, timeout=timeout)
    assert not marker.exists()


@pytest.mark.parametrize("status", [-1, 256, True, "0"])
def test_invalid_expected_status_is_rejected_before_launch(status):
    with pytest.raises(ValueError, match="Expected status"):
        IntegrationRunner().run_bash_command(
            python_command("raise AssertionError('must not launch')"),
            expected_status=status,
        )
