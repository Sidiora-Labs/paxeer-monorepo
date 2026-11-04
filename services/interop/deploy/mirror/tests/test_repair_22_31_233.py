import base64
import copy
import json
import os
import ssl
import subprocess
import sys
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest


MIRROR = Path(__file__).resolve().parents[1]
HARNESS = MIRROR / "Cargo.toml"
TOKEN = "mirror-deployment-repair-test"


def run(arguments, **kwargs):
    result = subprocess.run(arguments, capture_output=True, text=True, timeout=900, **kwargs)
    assert result.returncode == 0, result.stdout + result.stderr
    return result


@pytest.fixture(scope="session")
def clients(tmp_path_factory):
    target = tmp_path_factory.mktemp("mirror-rust")
    run(["cargo", "build", "--locked", "--manifest-path", str(HARNESS), "--bins", "--target-dir", str(target)])
    return target / "debug"


@pytest.fixture(scope="session")
def certificates(tmp_path_factory):
    directory = tmp_path_factory.mktemp("mirror-tls")
    ca = directory / "ca.pem"
    run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
         "-subj", "/CN=mirror-repair-ca", "-keyout", str(directory / "ca.key"), "-out", str(ca),
         "-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign"])
    run(["openssl", "req", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=localhost",
         "-keyout", str(directory / "server.key"), "-out", str(directory / "server.csr")])
    extensions = directory / "extensions"
    extensions.write_text("subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:FALSE\n"
                          "keyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n")
    run(["openssl", "x509", "-req", "-in", str(directory / "server.csr"), "-CA", str(ca),
         "-CAkey", str(directory / "ca.key"), "-CAcreateserial", "-days", "1",
         "-extfile", str(extensions), "-out", str(directory / "server.pem")])
    run(["openssl", "x509", "-in", str(ca), "-outform", "DER", "-out", str(directory / "ca.der")])
    token = directory / "token"
    token.write_text(TOKEN)
    token.chmod(0o600)
    return directory


@contextmanager
def rpc_servers(certificates, dispatch):
    requests = []
    failures = []

    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            try:
                assert self.path == "/rpc"
                assert self.headers["Content-Type"] == "application/json"
                assert request["jsonrpc"] == "2.0"
                assert set(request) == {"jsonrpc", "id", "method", "params"}
                if self.headers.get("User-Agent") == "LayerX-Mirror/2":
                    assert self.headers["Authorization"] == "Bearer " + TOKEN
                else:
                    assert self.headers.get("Authorization") is None
                requests.append((self.server.server_port, request))
                response = {"jsonrpc": "2.0", "id": request["id"], "result": dispatch(request)}
            except Exception as error:
                failures.append(repr(error))
                response = {"jsonrpc": "2.0", "id": request["id"],
                            "error": {"code": -32602, "message": str(error)}}
            body = json.dumps(response).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(body)
            self.wfile.flush()
            self.close_connection = True
            try:
                self.connection.unwrap()
            except (ssl.SSLError, OSError):
                # The client may close after receiving our TLS close_notify.
                pass

    servers = []
    threads = []
    for _ in range(2):
        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(certificates / "server.pem", certificates / "server.key")
        server.socket = context.wrap_socket(server.socket, server_side=True)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        servers.append(server)
        threads.append(thread)
    rpc = {
        "endpoints": [
            {"url": f"https://127.0.0.1:{server.server_port}/rpc",
             "ca_certificate_der": str(certificates / "ca.der"),
             "bearer_token_file": str(certificates / "token"),
             "independent_backend": f"repair-backend-{index}"}
            for index, server in enumerate(servers)
        ],
        "quorum": 2, "connect_timeout_ms": 1000, "request_timeout_ms": 3000,
        "maximum_response_bytes": 16 * 1024 * 1024,
    }
    try:
        yield rpc, requests
    finally:
        for server in servers:
            server.shutdown()
            server.server_close()
        for thread in threads:
            thread.join(timeout=5)
            assert not thread.is_alive()
        assert not failures, failures


def test_mirror_verify_shape(clients, certificates, tmp_path):
    produced = json.loads(run([str(clients / "repair-driver"), "produce"]).stdout)
    example = json.loads((MIRROR / "verify-config.example.json").read_text())
    publisher = json.loads((MIRROR / "config.example.json").read_text())
    assert set(example) == {"layerx_network_id", "receipt_keys", "sources"}
    assert example["receipt_keys"] == publisher["paxeer"]["receipt_keys"] == produced["receipt_keys"]
    assert example["layerx_network_id"] == publisher["paxeer"]["chain_id"] == 125
    source = example["sources"][0]

    def dispatch(request):
        method, params = request["method"], request["params"]
        if method == "eth_chainId":
            assert params == []
            return hex(source["chain_id"])
        if method == "eth_getBlockByNumber":
            assert params[1] is False
            assert params[0] in ("0x0", "finalized", "0x64")
            return {"number": "0x0" if params[0] == "0x0" else "0x64", "hash": "0x" + "11" * 32}
        if method == "eth_getCode":
            assert params == ["0x" + source["archive_contract_hex"], "0x64"]
            return produced["code"]
        assert method == "eth_call"
        assert params[1] == "0x64"
        assert params[0]["to"] == "0x" + source["archive_contract_hex"]
        data = params[0]["data"][2:]
        for kind, selector in produced["selectors"].items():
            if data.startswith(selector):
                expected = selector
                if kind != "publisher":
                    expected += produced["commitment"]
                if kind == "chunk":
                    expected += "00" * 32
                assert data == expected
                return produced[kind]
        raise AssertionError("unknown archive ABI call")

    with rpc_servers(certificates, dispatch) as (rpc, requests):
        # Preserve the example's complete schema, network and receipt-key windows.
        source.update(rpc=rpc, genesis_hash_hex="11" * 32,
                      archive_code_hash_hex=produced["code_hash"], publisher_hex="01" * 20)
        request = {"batch_number": str(produced["height"]), "evidence": {"kind": "receipt", "index": 0},
                   "policy": {"kind": "exact", "candidate": {"source": 0, "commitment_hex": produced["commitment"]}}}

        def verify(document):
            config = tmp_path / "verify.json"
            config.write_text(json.dumps(document))
            return json.loads(run([str(clients / "layerx-mirror-verify"), str(config)], input=json.dumps(request)).stdout)

        for index in (0, 1):
            request["evidence"]["index"] = index
            result = verify(example)
            assert result["ok"] is True, result
            report = result["verification"]
            assert report["level"] == "Committed"
            assert report["committedHeight"] == str(produced["height"])
            assert report["committedBlockHash"] == produced["block_hash"]
            assert len(report["evidenceDigest"]) == 64
            assert report["sourceId"] == "ethereum-primary"
            assert report["provenance"] == "Canonical"
        for first, last in ((1001, 1999), (1, 999)):
            outside = copy.deepcopy(example)
            outside["receipt_keys"][0].update(first_height=first, last_height=last)
            assert verify(outside) == {"ok": False, "error": "verification"}
        wrong_key = copy.deepcopy(example)
        wrong_key["receipt_keys"][0]["public_key_hex"] = "09" * 32
        assert verify(wrong_key) == {"ok": False, "error": "verification"}
        obsolete = copy.deepcopy(example)
        obsolete["sequencer_public_key_hex"] = obsolete["receipt_keys"][0]["public_key_hex"]
        assert verify(obsolete) == {"ok": False, "error": "configuration"}
        assert {port for port, _ in requests} == {int(endpoint["url"].split(":")[2].split("/")[0]) for endpoint in rpc["endpoints"]}
        assert any(request["method"] == "eth_call" for _, request in requests)


def test_solana_mutable_deployment(clients, certificates, tmp_path):
    elf = Path("/bin/true")
    fixture = json.loads(run([str(clients / "repair-driver"), "loader-fixture", str(elf)]).stdout)
    deployment = fixture["identity"]
    state = {"data": fixture["mutable"], "root": 100, "account_slot": 100,
             "status": "finalized", "owner": deployment["upgradeable_loader_id"], "executable": False}

    def dispatch(request):
        method, params = request["method"], request["params"]
        if method == "getGenesisHash":
            assert params == []
            return deployment["genesis_hash"]
        if method == "getSlot":
            assert params == [{"commitment": "finalized"}]
            return state["root"]
        if method == "getSignatureStatuses":
            assert params == [[deployment["deployment_signature"]], {"searchTransactionHistory": True}]
            return {"context": {"slot": 100}, "value": [
                {"slot": 42, "confirmations": None, "err": None, "status": {"Ok": None},
                 "confirmationStatus": state["status"]}]}
        assert method == "getAccountInfo"
        assert params[1]["commitment"] == "finalized"
        assert params[1]["encoding"] == "base64"
        if "minContextSlot" in params[1]:
            assert params[1]["minContextSlot"] == state["root"]
        address = params[0]
        if address == deployment["program_id"]:
            value = {"data": [fixture["program"], "base64"], "executable": True,
                     "owner": deployment["upgradeable_loader_id"], "lamports": 1, "rentEpoch": 0}
        elif address == deployment["program_data_account"]:
            value = {"data": [state["data"], "base64"], "executable": state["executable"],
                     "owner": state["owner"], "lamports": 1, "rentEpoch": 0}
        else:
            value = None
        return {"context": {"slot": state["account_slot"]}, "value": value}

    with rpc_servers(certificates, dispatch) as (rpc, requests):
        arguments = [sys.executable, str(MIRROR / "verify-solana-deployment.py"),
                     "--rpc-url", rpc["endpoints"][0]["url"], "--genesis-hash", deployment["genesis_hash"],
                     "--program-id", deployment["program_id"], "--program-data", deployment["program_data_account"],
                     "--deployment-signature", deployment["deployment_signature"], "--elf", str(elf)]
        environment = {**os.environ, "SSL_CERT_FILE": str(certificates / "ca.pem")}

        def check(extra=()):
            return subprocess.run(arguments + list(extra), env=environment, capture_output=True, text=True, timeout=30)

        def clients_observe(name):
            config = tmp_path / (name + ".json")
            config.write_text(json.dumps({"rpc": rpc, "deployment": deployment, "code_hash": fixture["code_hash"]}))
            return json.loads(run([str(clients / "repair-driver"), "identity", str(config)]).stdout)

        before = check(["--expected-authority", deployment["publisher_ed25519_public_key"]])
        assert before.returncode == 0, before.stderr
        assert json.loads(before.stdout)["upgrade_authority"] == deployment["publisher_ed25519_public_key"]
        refused = check()
        assert refused.returncode == 1 and "program remains mutable" in refused.stderr
        assert refused.stdout == ""
        assert clients_observe("mutable") == {"publisher": "ProgramMutable", "reader": "ProgramMutable"}

        # Both snapshots are emitted by the real Solana loader's bincode codec.
        state["data"] = fixture["immutable"]
        ready = check()
        assert ready.returncode == 0, ready.stderr
        record = json.loads(ready.stdout)
        assert record == {"rooted_slot": 100, "deployment_finalized_slot": 42, "upgrade_authority": None,
                          "program_elf_sha256": fixture["code_hash"], "program_elf_bytes": fixture["elf_bytes"],
                          "on_chain_elf_verified": True}
        assert clients_observe("immutable") == {"publisher": "accepted", "reader": "accepted"}

        original = base64.b64decode(state["data"])
        changed = bytearray(original)
        changed[-1] ^= 1
        state["data"] = base64.b64encode(changed).decode()
        refused = check()
        assert refused.returncode == 1 and "ELF identity mismatch" in refused.stderr
        assert clients_observe("different-elf") == {"publisher": "ProgramIdentity", "reader": "ProgramIdentity"}
        state["data"] = fixture["immutable"]
        for field, invalid, message in (("account_slot", 99, "behind the rooted observation"),
                                        ("root", 41, "not rooted successfully"),
                                        ("status", "confirmed", "not rooted successfully"),
                                        ("owner", deployment["program_id"], "loader account identity mismatch"),
                                        ("executable", True, "loader account identity mismatch")):
            previous = state[field]
            state[field] = invalid
            refused = check()
            assert refused.returncode == 1 and message in refused.stderr, refused.stderr
            assert refused.stdout == ""
            state[field] = previous
        assert any(request["method"] == "getAccountInfo" and "minContextSlot" in request["params"][1]
                   for _, request in requests)


def test_finalization_requires_explicit_authorization(tmp_path):
    record = tmp_path / "deployment.json"
    environment = {**os.environ, "LAYERX_SOLANA_MIRROR_RPC_URL": "https://127.0.0.1:1/rpc",
                   "LAYERX_SOLANA_MIRROR_KEYPAIR_FILE": str(tmp_path / "publisher.json"),
                   "LAYERX_SOLANA_MIRROR_DEPLOYMENT_RECORD": str(record),
                   "LAYERX_SOLANA_MIRROR_TOOLCHAIN_BIN": str(tmp_path)}
    environment.pop("LAYERX_SOLANA_MIRROR_FINALIZE", None)
    result = subprocess.run(["bash", str(MIRROR / "deploy-solana-mirror.sh")], env=environment,
                            capture_output=True, text=True, timeout=10)
    assert result.returncode == 1
    assert "LAYERX_SOLANA_MIRROR_FINALIZE=YES" in result.stderr
    assert not record.exists()

    # The executable deployment path must perform both checks around finalization.
    script = (MIRROR / "deploy-solana-mirror.sh").read_text()
    before = script.index('"${verify[@]}" --expected-authority "$publisher"')
    finalize = script.index('"$SOLANA" program set-upgrade-authority "$program_id"')
    after = script.index('"${verify[@]}" > "$work/ready.json"')
    publish = script.index('> "$LAYERX_SOLANA_MIRROR_DEPLOYMENT_RECORD"')
    assert before < finalize < after < publish
    assert '--final \\\n    --commitment finalized' in script
    assert '"$work/ready.json" >' in script
