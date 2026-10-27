import json
import os
import stat
import subprocess
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


def protected(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as handle:
        info = os.fstat(handle.fileno())
        assert stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
        assert stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
        value = handle.read(4097)
        assert 0 < len(value) <= 4096
        return value


def fixed(value):
    decoded = bytes.fromhex(value)
    assert len(decoded) == 32 and decoded.hex() == value and any(decoded)
    return decoded


def write(path, value):
    with path.open("xb") as output:
        os.fchmod(output.fileno(), 0o600)
        output.write(value.encode() if isinstance(value, str) else value)
        output.flush()
        os.fsync(output.fileno())
    return path


class RpcError(RuntimeError):
    def __init__(self, method, error):
        super().__init__(f"JSON-RPC {error['code']}: {method}: {error['message']}")
        self.code = error["code"]
        self.message = error["message"]
        self.data = error.get("data")


class JourneyRpc:
    def __init__(self, gateway, ca, auth_config, output):
        assert gateway.startswith("https://")
        protected(auth_config)
        self.gateway = gateway.rstrip("/")
        self.ca = ca
        self.auth_config = auth_config
        self.output = output
        self.counter = 0

    def rpc(self, method, params, *, auth_config=None):
        assert method.startswith(("px_", "eth_", "pax_")), method
        config = self.auth_config if auth_config is None else auth_config
        protected(config)
        self.counter += 1
        body = json.dumps(
            {"jsonrpc": "2.0", "id": self.counter, "method": method, "params": params}
        ).encode()
        request = write(self.output / f"{self.counter:02d}-request.json", body)
        result = subprocess.run(
            [
                "curl",
                "--fail-with-body",
                "--silent",
                "--show-error",
                "--max-time",
                "120",
                "--cacert",
                str(self.ca),
                "--config",
                str(config),
                "--request",
                "POST",
                self.gateway + "/rpc",
                "--header",
                "Content-Type: application/json",
                "--data-binary",
                "@" + str(request),
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        write(self.output / f"{self.counter:02d}-response.json", result.stdout)
        value = json.loads(result.stdout)
        assert result.returncode == 0, f"journey transport failed: {result.returncode}"
        assert value.get("jsonrpc") == "2.0" and value.get("id") == self.counter, value
        assert ("error" in value) != ("result" in value), value
        if "error" in value:
            error = value["error"]
            assert type(error.get("code")) is int and isinstance(
                error.get("message"), str
            )
            raise RpcError(method, error)
        return value["result"]
