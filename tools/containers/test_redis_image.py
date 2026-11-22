import os
from pathlib import Path
import shutil
import socket
import subprocess
import time

import pytest


ROOT = Path(__file__).resolve().parents[2]
ENTRYPOINT = ROOT / "docker/redis/entrypoint.sh"
DEFAULT_USER = "user default off ~* &* +@all\n"
NAMED_USER = "user application on >application-password ~* &* +@all\n"


def render(mode, supplied="", role="redis-router"):
    result = subprocess.run(
        ["sh", str(ENTRYPOINT), mode],
        input=supplied,
        env={"PATH": os.environ["PATH"], "LAYERX_ROLE": role},
        capture_output=True,
        text=True,
        timeout=5,
    )
    assert result.returncode == 0, result.stderr
    return result.stdout


@pytest.mark.parametrize("role", ["redis-router", "redis-internal"])
@pytest.mark.parametrize(
    ("supplied", "expected"),
    [
        ("user default off\n" + NAMED_USER, DEFAULT_USER + NAMED_USER),
        (
            "user default off resetkeys resetchannels -@all >unused\n" + NAMED_USER,
            DEFAULT_USER + NAMED_USER,
        ),
        (NAMED_USER, DEFAULT_USER + NAMED_USER),
        (
            "user default on nopass ~public:* &updates +get\n" + NAMED_USER,
            "user default on nopass ~public:* &updates +get\n" + NAMED_USER,
        ),
        ("", DEFAULT_USER),
        (
            "# application access\n\n" + NAMED_USER + "\tuser  default off\n",
            "# application access\n\n" + NAMED_USER + DEFAULT_USER,
        ),
        (
            "user default on >off ~* &* +@all\n" + NAMED_USER,
            "user default on >off ~* &* +@all\n" + NAMED_USER,
        ),
        (DEFAULT_USER + NAMED_USER, DEFAULT_USER + NAMED_USER),
    ],
    ids=[
        "disabled",
        "disabled-with-rules",
        "missing-default",
        "enabled",
        "empty",
        "preserve-other-lines",
        "password-is-not-off-rule",
        "effective-list",
    ],
)
def test_effective_access_list(supplied, expected, role):
    effective = render("render-acl", supplied, role)
    assert effective == expected
    assert (
        sum(line.split()[:2] == ["user", "default"] for line in effective.splitlines())
        == 1
    )


@pytest.mark.parametrize("role", ["redis-router", "redis-internal"])
def test_configuration_uses_effective_access_list(role):
    config = render("render", role=role).splitlines()
    assert config[0] == "include /run/secrets/redis.conf"
    assert [line for line in config if line.startswith("aclfile ")] == [
        "aclfile /run/layerx/users.acl"
    ]
    assert "appendonly yes" in config
    assert "appendfsync always" in config


def command(stream, *arguments):
    encoded = [argument.encode() for argument in arguments]
    stream.write(
        b"*%d\r\n" % len(encoded)
        + b"".join(b"$%d\r\n" % len(value) + value + b"\r\n" for value in encoded)
    )
    line = stream.readline()
    assert line.endswith(b"\r\n"), line
    if line.startswith(b"$"):
        length = int(line[1:-2])
        assert length >= 0, line
        value = stream.read(length)
        assert len(value) == length
        assert stream.read(2) == b"\r\n"
        return value
    return line[:-2]


def start_server(binary, config, unix_socket, log):
    process = subprocess.Popen(
        [binary, str(config)], stdout=log, stderr=subprocess.STDOUT
    )
    try:
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            assert process.poll() is None, "Redis exited before accepting connections"
            try:
                with socket.socket(socket.AF_UNIX) as connection:
                    connection.settimeout(1)
                    connection.connect(str(unix_socket))
                return process
            except (FileNotFoundError, ConnectionRefusedError):
                time.sleep(0.02)
        pytest.fail("Redis did not accept connections within five seconds")
    except BaseException:
        process.kill()
        process.wait(timeout=5)
        raise


def stop_server(process):
    process.terminate()
    try:
        assert process.wait(timeout=5) == 0
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)


def test_append_only_recovery_with_disabled_default_user(tmp_path):
    binary = shutil.which("redis-server")
    if binary is None:
        pytest.skip("redis-server is not available on PATH")

    acl = tmp_path / "users.acl"
    acl.write_text(render("render-acl", "user default off\n" + NAMED_USER))
    acl.chmod(0o600)
    unix_socket = tmp_path / "redis.sock"
    config = tmp_path / "redis.conf"
    config.write_text(
        f"port 0\nunixsocket {unix_socket}\nunixsocketperm 700\n"
        f"aclfile {acl}\ndir {tmp_path}\n"
        'save ""\nappendonly yes\nappendfsync always\n'
        "aof-use-rdb-preamble no\nauto-aof-rewrite-percentage 0\n"
    )

    with (tmp_path / "redis.log").open("wb") as log:
        process = start_server(binary, config, unix_socket, log)
        try:
            with socket.socket(socket.AF_UNIX) as connection:
                connection.settimeout(2)
                connection.connect(str(unix_socket))
                with connection.makefile("rwb", buffering=0) as stream:
                    assert command(stream, "PING").startswith(b"-NOAUTH ")
                    assert (
                        command(stream, "AUTH", "application", "application-password")
                        == b"+OK"
                    )
                    assert command(
                        stream,
                        "EVAL",
                        "redis.call('SET', KEYS[1], ARGV[1]); "
                        "redis.call('SET', KEYS[2], ARGV[2]); return 2",
                        "2",
                        "script:first",
                        "script:second",
                        "first-value",
                        "second-value",
                    ) == b":2"
                    assert command(stream, "SET", "plain", "plain-value") == b"+OK"
        finally:
            stop_server(process)

        append_files = [path for path in tmp_path.rglob("*.aof") if path.is_file()]
        assert append_files, "Redis must persist an append-only file"
        append_data = b"".join(path.read_bytes() for path in append_files)
        assert b"\r\nMULTI\r\n" in append_data
        assert b"\r\nEXEC\r\n" in append_data

        process = start_server(binary, config, unix_socket, log)
        try:
            with socket.socket(socket.AF_UNIX) as connection:
                connection.settimeout(2)
                connection.connect(str(unix_socket))
                with connection.makefile("rwb", buffering=0) as stream:
                    assert command(stream, "PING").startswith(b"-NOAUTH ")
                    assert (
                        command(stream, "AUTH", "application", "application-password")
                        == b"+OK"
                    )
                    for key, value in [
                        ("script:first", b"first-value"),
                        ("script:second", b"second-value"),
                        ("plain", b"plain-value"),
                    ]:
                        assert command(stream, "GET", key) == value
        finally:
            stop_server(process)
