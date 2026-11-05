import contextlib
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request


GENESIS_TIME = 1_700_000_000
PUBLISHER = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
BEARER = "repair2231231-local-rpc"


def rpc(url, method, params):
    request = urllib.request.Request(
        url,
        data=json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=10) as response:
        document = json.load(response)
    assert "error" not in document, (method, document)
    return document["result"]


def next_block(url):
    height = int(rpc(url, "eth_blockNumber", []), 16)
    rpc(url, "evm_setNextBlockTimestamp", [GENESIS_TIME + height + 1])


def wait_receipt(url, transaction):
    deadline = time.monotonic() + 10
    while True:
        receipt = rpc(url, "eth_getTransactionReceipt", [transaction])
        if receipt is not None:
            return receipt
        assert time.monotonic() < deadline, f"transaction was not mined: {transaction}"
        time.sleep(0.01)


def cast(*arguments):
    return subprocess.check_output(["cast", *arguments], text=True).strip()


def word(number):
    return number.to_bytes(32, "big").hex()


class Backend(http.server.ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, url, peers, tls):
        super().__init__(("127.0.0.1", 0), Forward)
        self.url = url
        self.peers = peers
        self.requests = []
        self.socket = tls.wrap_socket(self.socket, server_side=True)

    def shutdown_request(self, request):
        request.setblocking(False)
        try:
            request.unwrap()
        except ssl.SSLWantReadError:
            # close_notify is sent; the publisher closes after reading it.
            pass
        finally:
            super().shutdown_request(request)


class Forward(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        assert self.headers.get("Authorization") == f"Bearer {BEARER}"
        body = self.rfile.read(int(self.headers["Content-Length"]))
        document = json.loads(body)
        self.server.requests.append(document)
        if document["method"] == "eth_sendRawTransaction":
            # Anvil has no peer network. Relay identical signed bytes to both
            # independent EVMs before acknowledging acceptance to the publisher.
            responses = []
            for peer in self.server.peers:
                next_block(peer)
                request = urllib.request.Request(
                    peer, data=body, headers={"Content-Type": "application/json"}
                )
                with urllib.request.urlopen(request, timeout=10) as upstream:
                    response = upstream.read()
                result = json.loads(response)
                assert "error" not in result, result
                receipt = wait_receipt(peer, result["result"])
                assert receipt["status"] == "0x1", "real publication transaction reverted"
                responses.append(response)
            assert json.loads(responses[0]) == json.loads(responses[1])
            response = responses[self.server.peers.index(self.server.url)]
        else:
            response = self.forward(body)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(response)

    def forward(self, body):
        # Forward the producer's JSON-RPC response unchanged, including logs.
        request = urllib.request.Request(
            self.server.url, data=body, headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(request, timeout=10) as upstream:
            return upstream.read()


def certificates(directory):
    def openssl(*arguments):
        subprocess.run(
            ["openssl", *arguments], cwd=directory, check=True,
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        )

    openssl("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
            "-subj", "/CN=Repair2231231 CA", "-addext", "basicConstraints=critical,CA:TRUE",
            "-keyout", "ca.key", "-out", "ca.pem")
    openssl("req", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=localhost",
            "-keyout", "server.key", "-out", "server.csr")
    (directory / "server.ext").write_text(
        "basicConstraints=critical,CA:FALSE\n"
        "keyUsage=critical,digitalSignature,keyEncipherment\n"
        "extendedKeyUsage=serverAuth\nsubjectAltName=IP:127.0.0.1\n"
    )
    openssl("x509", "-req", "-in", "server.csr", "-CA", "ca.pem", "-CAkey", "ca.key",
            "-CAcreateserial", "-days", "1", "-extfile", "server.ext", "-out", "server.pem")
    openssl("x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der")
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    tls.load_cert_chain(directory / "server.pem", directory / "server.key")
    return tls


def start_anvil(stack, directory, index):
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    log = stack.enter_context((directory / f"anvil-{index}.log").open("w"))
    process = subprocess.Popen(
        ["anvil", "--host", "127.0.0.1", "--port", str(port), "--timestamp", str(GENESIS_TIME),
         "--chain-id", "31337", "--gas-limit", "30000000", "--silent"],
        stdout=log, stderr=log,
    )

    def stop():
        process.terminate()
        process.wait(timeout=10)

    stack.callback(stop)
    url = f"http://127.0.0.1:{port}"
    deadline = time.monotonic() + 15
    while True:
        assert process.poll() is None, "Anvil exited before readiness"
        try:
            assert rpc(url, "eth_chainId", []) == "0x7a69"
            return url
        except urllib.error.URLError:
            if time.monotonic() >= deadline:
                raise
            time.sleep(0.05)


def check_manifest(receipt, contract, submitted, topics):
    assert receipt["status"] == "0x1"
    assert receipt["to"].lower() == contract
    assert len(receipt["logs"]) == 1
    log = receipt["logs"][0]
    assert log["address"].lower() == contract
    assert log["topics"] == [topics["manifest"], submitted["commitment"]]
    archive = bytes.fromhex(submitted["archive"][2:])
    assert log["data"] == (
        "0x" + word(125) + word(submitted["height"]) + submitted["blockHash"][2:]
        + word(len(archive)) + word(submitted["chunks"]) + submitted["digest"][2:]
    )
    assert log["topics"][0] != cast("keccak", "ManifestOpened(bytes32,uint32,uint64,uint64,uint32,bytes32)")


def main():
    source = Path(__file__).resolve().parent
    repo = source.parents[5]
    creation, runtime = sys.argv[1:]
    with tempfile.TemporaryDirectory(prefix="repair2231231-") as temporary, contextlib.ExitStack() as stack:
        directory = Path(temporary)
        os.chmod(directory, 0o700)
        # Keep Cargo's generated lock and build output in the private run directory.
        package = directory / "publisher"
        shutil.copytree(source / "src", package / "src")
        manifest = (source / "Cargo.toml").read_text().replace(
            "../../../../crates/", str(repo / "services/interop/crates") + "/"
        )
        (package / "Cargo.toml").write_text(manifest)
        subprocess.run(["cargo", "build", "--quiet", "--manifest-path", str(package / "Cargo.toml")],
                       cwd=repo, check=True)
        binary = package / "target/debug/repair2231231-publisher"
        tls = certificates(directory)
        (directory / "bearer").write_text(BEARER)
        os.chmod(directory / "bearer", 0o600)
        urls, backends, contracts, genesis = [], [], [], []
        for index in range(2):
            url = start_anvil(stack, directory, index)
            urls.append(url)
            rpc(url, "anvil_setBalance", [PUBLISHER, hex(10**22)])
            genesis.append(rpc(url, "eth_getBlockByNumber", ["0x0", False])["hash"])
            deployer = rpc(url, "eth_accounts", [])[0]
            next_block(url)
            transaction = rpc(url, "eth_sendTransaction", [{
                "from": deployer, "data": creation, "gas": hex(29_000_000), "gasPrice": hex(2_000_000_000),
            }])
            receipt = wait_receipt(url, transaction)
            assert receipt["status"] == "0x1", "real archive deployment failed"
            contract = receipt["contractAddress"].lower()
            contracts.append(contract)
            assert rpc(url, "eth_getCode", [contract, "latest"]) == runtime
        for url in urls:
            backend = Backend(url, urls, tls)
            stack.callback(backend.server_close)
            threading.Thread(target=backend.serve_forever, daemon=True).start()
            stack.callback(backend.shutdown)
            backends.append(backend)
        assert genesis[0] == genesis[1], "independent backends disagree on genesis"
        assert contracts[0] == contracts[1], "independent deployments disagree"
        contract = contracts[0]
        settings = {
            "directory": str(directory), "fixture": str(repo / "services/interop/crates/layerx-mirror/tests/fixtures/committed_blocks.json"),
            "endpoints": [f"https://127.0.0.1:{backend.server_port}/" for backend in backends],
            "genesis": genesis[0], "contract": contract, "runtime": runtime,
        }
        settings_file = directory / "settings.json"
        settings_file.write_text(json.dumps(settings))

        def publisher(mode):
            return json.loads(subprocess.check_output([str(binary), mode, str(settings_file)], cwd=repo, text=True))

        topics = {
            "manifest": cast("keccak", "ManifestOpened(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32)"),
            "chunk": cast("keccak", "ChunkStored(bytes32,uint32,bytes32,uint32)"),
            "finalize": cast("keccak", "ArchiveFinalized(bytes32,bytes32)"),
        }
        submitted = publisher("submit")  # This process exits with begin still Pending in its journal.
        receipts = [wait_receipt(url, submitted["transaction"]) for url in urls]
        assert receipts[0] == receipts[1], "real begin receipts disagree"
        check_manifest(receipts[0], contract, submitted, topics)
        sent_before_recovery = sum(r["method"] == "eth_sendRawTransaction" for b in backends for r in b.requests)
        resumed = publisher("recover")  # A new process reopens and verifies that exact receipt.
        assert resumed["commitment"] == submitted["commitment"]
        assert sum(r["method"] == "eth_sendRawTransaction" for b in backends for r in b.requests) == (
            sent_before_recovery + 1
        ), "recovery must only submit chunk zero, never another begin"
        finished = publisher("finish")  # Exit/reopen once more while chunk zero is Pending.
        assert finished["commitment"] == submitted["commitment"]
        archive = bytes.fromhex(submitted["archive"][2:])
        assert hashlib.sha256(archive).hexdigest() == submitted["digest"][2:]
        manifest_selector = cast("sig", "manifest(bytes32)")
        chunk_selector = cast("sig", "chunk(bytes32,uint32)")
        for url, backend in zip(urls, backends):
            metadata = rpc(url, "eth_call", [{"to": contract, "data": manifest_selector + submitted["commitment"][2:]}, "latest"])
            assert metadata == "0x" + word(len(archive)) + word(submitted["chunks"]) + submitted["digest"][2:] + word(1)
            coordinate_slot = cast("keccak", "0x" + submitted["commitment"][2:] + word(0))
            assert int(rpc(url, "eth_getStorageAt", [contract, coordinate_slot, "latest"]), 16) == 125 | (1000 << 32)
            block_hash_slot = hex(int(coordinate_slot, 16) + 1)
            assert rpc(url, "eth_getStorageAt", [contract, block_hash_slot, "latest"]) == submitted["blockHash"]
            logs = rpc(url, "eth_getLogs", [{"address": contract, "fromBlock": "0x0", "toBlock": "latest"}])
            assert len(logs) == submitted["chunks"] + 2
            assert logs[0] == receipts[0]["logs"][0]
            for index in range(submitted["chunks"]):
                chunk = archive[index * 1024:(index + 1) * 1024]
                log = logs[index + 1]
                assert log["topics"] == [topics["chunk"], submitted["commitment"]]
                assert log["data"] == "0x" + word(index) + hashlib.sha256(chunk).hexdigest() + word(len(chunk))
                data = chunk_selector + submitted["commitment"][2:] + word(index)
                encoded = rpc(url, "eth_call", [{"to": contract, "data": data}, "latest"])
                assert encoded == "0x" + word(32) + word(len(chunk)) + chunk.hex() + "00" * (-len(chunk) % 32)
            assert logs[-1]["topics"] == [topics["finalize"], submitted["commitment"]]
            assert logs[-1]["data"] == submitted["digest"]
        broadcasts = [request for backend in backends for request in backend.requests
                      if request["method"] == "eth_sendRawTransaction"]
        assert len(broadcasts) == submitted["chunks"] + 2, "unexpected replay or extra publication"
        assert len({request["params"][0] for request in broadcasts}) == len(broadcasts), "transaction replay"
        sys.stdout.write(
            submitted["commitment"] + submitted["digest"][2:] + submitted["blockHash"][2:]
            + word(submitted["height"]) + word(submitted["chunks"]) + word(1)
        )


if __name__ == "__main__":
    main()
