"""The indexer health front serves the first backfill and the live service."""

import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
INDEXER = ROOT / "docker/platform-indexer"
HEALTH_CONFIG = INDEXER / "health-config.sh"
BASE = "7f00e056b077488ac35595777558ca3a18656953"


@pytest.mark.parametrize("port", ["1", "8080", "65535"])
def test_backfill_health_config_needs_only_port(port):
    result = subprocess.run(
        ["sh", str(HEALTH_CONFIG), "backfill"],
        env={"PORT": port}, capture_output=True, text=True, check=True,
    )
    assert result.stderr == ""
    config = result.stdout
    assert f"listen [::]:{port} ipv6only=off;" in config
    assert re.search(
        r'location = /healthz\s*\{\s*default_type text/plain;\s*return 200 "backfill";\s*\}',
        config,
    )
    assert "location / { return 404; }" in config
    assert not re.search(r"\bproxy_pass\b", config)


@pytest.mark.parametrize("port,tls_port", [("8080", "8443"), ("65535", "1")])
def test_live_health_config_preserves_output(port, tls_port):
    env = {
        "PORT": port,
        "LAYERX_INDEXER_LISTEN": f"[::]:{tls_port}",
        "LAYERX_INDEXER_HEALTH_SERVER_NAME": "indexer",
    }
    original = subprocess.run(
        ["git", "show", f"{BASE}:docker/platform-indexer/health-config.sh"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout
    expected = subprocess.run(
        ["sh"], input=original, env=env, capture_output=True, text=True, check=True,
    )
    actual = subprocess.run(
        ["sh", str(HEALTH_CONFIG)], env=env, capture_output=True, text=True, check=True,
    )
    assert actual.stdout == expected.stdout
    assert actual.stderr == expected.stderr == ""
    backfill = subprocess.run(
        ["sh", str(HEALTH_CONFIG), "backfill"],
        env={"PORT": port}, capture_output=True, text=True, check=True,
    ).stdout
    assert (
        backfill.split("\t\tlocation = /healthz")[0]
        == actual.stdout.split("\t\tlocation = /healthz")[0]
    )


@pytest.mark.parametrize("args", [[], ["backfill"]])
@pytest.mark.parametrize(
    "port", ["", "0", "08080", "65536", "999999", "-1", "abc", "8080;return 200"],
)
def test_health_config_rejects_invalid_port(args, port):
    result = subprocess.run(
        ["sh", str(HEALTH_CONFIG), *args],
        env={
            "PORT": port,
            "LAYERX_INDEXER_LISTEN": "[::]:8443",
            "LAYERX_INDEXER_HEALTH_SERVER_NAME": "indexer",
        },
        capture_output=True, text=True,
    )
    assert result.returncode == 2
    assert result.stdout == ""
    assert result.stderr


@pytest.mark.parametrize("args", [["unknown"], [""], ["backfill", "extra"]])
def test_health_config_rejects_unknown_arguments(args):
    result = subprocess.run(
        ["sh", str(HEALTH_CONFIG), *args], env={}, capture_output=True, text=True,
    )
    assert result.returncode == 2
    assert result.stdout == ""
    assert result.stderr == "usage: indexer-health-config [backfill]\n"


def test_backfill_health_front_precedes_backfill_and_stops_before_service():
    text = (INDEXER / "init.sh").read_text()
    branch = text.index('if [ ! -e "$state_dir/backfill-done" ]; then')
    configure = text.index("/usr/local/bin/indexer-health-config backfill >", branch)
    ownership = text.index("chown -R 4020:4020 /run/indexer-health", configure)
    start = text.index(
        "setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs", ownership,
    )
    front = text.index(
        "nginx -p /run/indexer-health/ -c /run/indexer-health/nginx.conf -g 'daemon off;' &",
        start,
    )
    track = text.index("health_pid=$!", front)
    backfill = text.index("/usr/local/bin/indexer-env-init backfill &", track)
    backfill_wait = text.index('wait "$indexer_pid"', backfill)
    stop = text.index('kill "$health_pid"', backfill_wait)
    stopped = text.index('wait "$health_pid"', stop)
    clear = text.index("health_pid=", stopped)
    end_branch = text.index("\nfi\n", clear)
    live_config = text.index("/usr/local/bin/indexer-health-config >", end_branch)
    service = text.index("/usr/local/bin/indexer-env-init serve &", live_config)
    live_front = text.index(
        "setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs", service,
    )
    assert branch < configure < ownership < start < front < track < backfill
    assert (
        backfill < backfill_wait < stop < stopped < clear < end_branch
        < live_config < service < live_front
    )
    assert 'set -eu\n' in text
    assert 'trap cleanup EXIT' in text
    cleanup = text.split("cleanup() {", 1)[1].split("\n}", 1)[0]
    assert 'kill "$indexer_pid"' in cleanup
    assert 'wait "$indexer_pid"' in cleanup
    assert 'kill "$health_pid"' in cleanup
    assert 'wait "$health_pid"' in cleanup


@pytest.mark.parametrize("script", ["init.sh", "health-config.sh"])
def test_indexer_scripts_have_valid_shell_syntax(script):
    result = subprocess.run(
        ["sh", "-n", str(INDEXER / script)], capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stderr
