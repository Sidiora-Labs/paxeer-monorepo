from concurrent.futures import ThreadPoolExecutor
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shlex
import stat
from threading import Barrier, Thread
import urllib.error

import pytest


class LocalRpcNode:
    """Loopback peer with a block persisted in its disposable node home."""

    def __init__(self, home, identity):
        self.home = home
        home.mkdir(mode=0o700, parents=True)
        (home / "block.json").write_text(json.dumps({
            "number": "0x17e8", "hash": "0x" + f"{identity:064x}",
            "parentHash": "0x" + "00" * 32, "transactions": [],
        }))
        (home / "process.env").write_text("PAXD=in-process-paxd\n")
        self.requests = []
        self.port = 0
        self.server = None
        self.open()

    @property
    def pid(self):
        return os.getpid()

    @property
    def url(self):
        return f"http://127.0.0.1:{self.port}"

    def open(self):
        assert self.server is None
        node = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                assert self.path == "/"
                assert self.headers["Content-Type"] == "application/json"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert request["jsonrpc"] == "2.0"
                assert request["id"] == 1
                node.requests.append(request)
                if request["method"] == "eth_chainId":
                    assert request["params"] == []
                    result = "0x7d"
                elif request["method"] == "eth_getBlockByNumber":
                    assert request["params"] == ["earliest", False]
                    result = json.loads((node.home / "block.json").read_text())
                else:
                    raise AssertionError(f"unexpected RPC method: {request['method']}")
                body = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", self.port), Handler)
        self.port = self.server.server_port
        self.thread = Thread(target=self.server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
        self.thread.start()

    def close(self):
        if self.server is not None:
            self.server.shutdown()
            self.server.server_close()
            self.thread.join(timeout=5)
            assert not self.thread.is_alive()
            self.server = None


class LocalNodeHelper:
    """In-process peer for the committed node helper's command and file protocol."""

    def __init__(self, node, work, script, barrier=None):
        self.node = node
        self.work = work
        self.script = script
        self.barrier = barrier
        self.commands = []

    def execute(self, name, command, timeout=30):
        self.commands.append((name, command))
        assert command[:2] == ["bash", self.script]
        if name == "history-node-setup":
            assert timeout == 240
            assert command[2] == "--env-out"
            env_file = Path(command[3])
            assert env_file.parent == self.node.home.parent
            assert stat.S_IMODE(env_file.stat().st_mode) == 0o600
            environment = (
                f"LOCAL_NODE_RPC={self.node.url}\n"
                f"LOCAL_NODE_HOME={shlex.quote(str(self.node.home))}\n"
                "LOCAL_NODE_KEY=0x0123456789abcdef\n"
                f"PERPS_DATA_TEST_RPC={self.node.url}\n"
                "PERPS_DATA_TEST_COMET=http://127.0.0.1:26657\n"
                "NEXT_PUBLIC_PERPS_GATEWAY_URL=http://127.0.0.1:0\n"
            )
            (self.node.home / "localnode.env").write_text(environment)
            env_file.write_text(environment)
            (self.work / (name + ".log")).write_text(
                f"v6120-node: ready; home={self.node.home} env={env_file}\n"
            )
            if self.barrier is not None:
                self.barrier.wait(timeout=10)
        elif name == "history-node-stop":
            assert command[2:] == ["--stop", self.node.home]
            self.node.close()
        else:
            raise AssertionError(f"unexpected helper command: {name}")

    def start(self, name, command):
        self.commands.append((name, command))
        assert name == "history-node-restarted"
        assert command == ["in-process-paxd", "start", "--home", self.node.home]
        self.node.open()
        return self.node


def test_history_env_isolation_concurrent_stop_restart(data_boundary, tmp_path):
    base = tmp_path / "lx-localnode"
    barrier = Barrier(2)
    script = Path(data_boundary.__file__).resolve().parents[2] / "tools/qualification/localnode/v6120-node.sh"
    peers = []
    clients = []
    helpers = []
    try:
        for identity in (1, 2):
            work = tmp_path / f"private evidence {identity}"
            work.mkdir(mode=0o700)
            peer = LocalRpcNode(base / f"node.{identity}", identity)
            peers.append(peer)
            clients.append(data_boundary.HistoryLocalNode(work, script, tmpdir=tmp_path))
            helpers.append(LocalNodeHelper(peer, work, script, barrier))
        legacy = base / "history-readiness.env"
        legacy.write_text(f"LOCAL_NODE_RPC={peers[1].url}\n")
        with ThreadPoolExecutor(max_workers=2) as pool:
            futures = [pool.submit(client.setup, helper.execute)
                       for client, helper in zip(clients, helpers)]
            urls = [future.result(timeout=15) for future in futures]

        assert urls == [peer.url for peer in peers]
        assert clients[0].env_file != clients[1].env_file
        blocks = []
        for client, peer in zip(clients, peers):
            assert client.home == peer.home
            assert client.env_file.parent == base
            assert client.env_file != legacy
            assert stat.S_IMODE(client.env_file.stat().st_mode) == 0o600
            assert stat.S_IMODE(client.work.stat().st_mode) == 0o700
            evidence = client.work / "localnode.env"
            assert stat.S_IMODE(evidence.stat().st_mode) == 0o600
            assert evidence.read_bytes() == client.env_file.read_bytes()
            assert evidence.read_bytes() == (peer.home / "localnode.env").read_bytes()
            assert data_boundary.history_local_node_rpc(evidence) == peer.url
            assert client.rpc("eth_chainId") == "0x7d"
            blocks.append(client.rpc("eth_getBlockByNumber", ["earliest", False]))
        assert blocks[0]["hash"] != blocks[1]["hash"]

        for index, (client, peer, helper) in enumerate(zip(clients, peers, helpers)):
            other = clients[1 - index]
            client.stop(helper.execute)
            assert peer.server is None
            with pytest.raises(urllib.error.URLError):
                client.rpc("eth_chainId")
            assert other.rpc("eth_chainId") == "0x7d"
            assert other.rpc("eth_getBlockByNumber", ["earliest", False]) == blocks[1 - index]
            client.restart(helper.start)
            assert (client.home / "node.pid").read_text() == str(peer.pid) + "\n"
            assert client.rpc("eth_chainId") == "0x7d"
            assert client.rpc("eth_getBlockByNumber", ["earliest", False]) == blocks[index]
            assert client.evm_url == urls[index]
            assert client.env_file.exists()
            assert (client.work / "localnode.env").read_bytes() == client.env_file.read_bytes()
            assert len(peer.requests) >= 4
        for client, helper in zip(clients, helpers):
            client.stop(helper.execute)
    finally:
        for peer in peers:
            peer.close()


def test_history_env_isolation_missing_own_file_is_not_shared_fallback(data_boundary, tmp_path):
    work = tmp_path / "private evidence"
    work.mkdir(mode=0o700)
    client = data_boundary.HistoryLocalNode(work, Path("v6120-node.sh"), tmpdir=tmp_path)
    legacy = client.env_file.parent / "history-readiness.env"
    legacy.write_text("LOCAL_NODE_RPC=http://127.0.0.1:8545\n")
    assert client.env_file != legacy
    with pytest.raises(ValueError, match="missing LOCAL_NODE_RPC"):
        data_boundary.history_local_node_rpc(client.env_file)
    client.env_file.unlink()
    with pytest.raises(FileNotFoundError):
        data_boundary.history_local_node_rpc(client.env_file)


@pytest.mark.parametrize("value", [
    "http://example.invalid:8545", "https://127.0.0.1:8545",
    "http://user:secret@127.0.0.1:8545", "http://127.0.0.1:8545 extra",
])
def test_history_env_isolation_preserves_rpc_boundary(data_boundary, tmp_path, value):
    environment = tmp_path / "node.env"
    environment.write_text(f"LOCAL_NODE_RPC={value}\n")
    with pytest.raises(ValueError, match="LOCAL_NODE_RPC"):
        data_boundary.history_local_node_rpc(environment)
