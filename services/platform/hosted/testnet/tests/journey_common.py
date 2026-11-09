import json
import os
import stat
import re
import ssl
import urllib.parse
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


def root_endpoint(endpoint):
    url = urllib.parse.urlsplit(endpoint)
    assert (
        url.scheme == "https" and url.hostname and url.path in ("", "/")
        and not url.username and not url.password and not url.query and not url.fragment
    ), "journeys require explicit HTTPS root endpoints"
    return endpoint.rstrip("/") + "/"


def authorization(config):
    text = protected(config).decode().removesuffix("\n")
    match = re.fullmatch(r'header = "Authorization: ([A-Za-z-]+ [A-Za-z0-9_.:-]+)"', text)
    assert match, "one explicit Authorization header is required"
    return match[1]


class JourneyRpc:
    HOSTED_METHODS = frozenset({"px_requestFunds", "px_register"})
    NODE_METHODS = frozenset({
        "eth_chainId", "eth_getBalance", "eth_getTransactionCount", "eth_gasPrice",
        "eth_estimateGas", "eth_sendRawTransaction", "pax_sequencerReceipt", "px_getBalances",
    })

    def __init__(self, gateway, ca, auth_config, output, *, node, node_auth_config=None):
        self.gateway = root_endpoint(gateway)
        self.node = root_endpoint(node)
        authorization(auth_config)
        if node_auth_config is not None:
            authorization(node_auth_config)
        self.auth_config = auth_config
        self.node_auth_config = node_auth_config
        self.output = output
        self.counter = 0
        self.opener = urllib.request.build_opener(
            NoRedirect(), urllib.request.HTTPSHandler(
                context=ssl.create_default_context(cafile=str(ca))
            )
        )

    def rpc(self, method, params, *, auth_config=None):
        assert method in self.HOSTED_METHODS | self.NODE_METHODS, method
        if method in self.HOSTED_METHODS:
            endpoint = self.gateway
            config = self.auth_config if auth_config is None else auth_config
        else:
            assert auth_config is None, "hosted credentials cannot be used on the node"
            endpoint, config = self.node, self.node_auth_config
        headers = {"Content-Type": "application/json"}
        if config is not None:
            headers["Authorization"] = authorization(config)
        self.counter += 1
        body = json.dumps(
            {"jsonrpc": "2.0", "id": self.counter, "method": method, "params": params}
        ).encode()
        write(self.output / f"{self.counter:02d}-request.json", body)
        request = urllib.request.Request(endpoint, data=body, headers=headers, method="POST")
        with self.opener.open(request, timeout=120) as response:
            raw = response.read(9 * 1048576 + 1)
            assert len(raw) <= 9 * 1048576, "oversized journey response"
        write(self.output / f"{self.counter:02d}-response.json", raw)
        value = json.loads(raw)
        assert value.get("jsonrpc") == "2.0" and type(value.get("id")) is int, value
        assert value["id"] == self.counter, value
        assert ("error" in value) != ("result" in value), value
        if "error" in value:
            error = value["error"]
            assert type(error.get("code")) is int and isinstance(error.get("message"), str)
            raise RpcError(method, error)
        return value["result"]
