"""Record real JSON-RPC execution data; requires @foundry-rs/anvil 1.7.1.

Usage: python3 record.py /path/to/anvil
Mixed history: uv run --no-project --with eth-account python record.py /path/to/geth --geth-history
Geth history uses 1.14.13 and a disposable local genesis driven through Engine V2/V3.
The recordings are replayed by ExUnit without requiring a node or network access.
"""

import base64
import hashlib
import hmac
import json
import os
import pathlib
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request


def constructor(runtime):
    size = len(bytes.fromhex(runtime))
    return f"0x60{size:02x}600c60003960{size:02x}6000f3{runtime}"


class Recorder:
    def __init__(self, executable, fork):
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.port = sock.getsockname()[1]
        self.fork = fork
        self.process = subprocess.Popen(
            [executable, "--host", "127.0.0.1", "--port", str(self.port),
             "--hardfork", fork, "--timestamp", "1710000000", "--silent"],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        )
        for _ in range(100):
            try:
                self.accounts = self.rpc("eth_accounts", [])['response']['result']
                break
            except (OSError, ValueError):
                time.sleep(0.05)
        else:
            raise RuntimeError("Anvil did not start")
        if fork == "cancun":
            self.rpc("anvil_mine", ["0x100"])

    def rpc(self, method, params, allow_error=False):
        request = {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
        data = json.dumps(request).encode()
        with urllib.request.urlopen(urllib.request.Request(
            f"http://127.0.0.1:{self.port}", data, {"Content-Type": "application/json"}
        ), timeout=10) as response:
            result = json.load(response)
        if "error" in result and not allow_error:
            raise RuntimeError(result["error"])
        return {"request": request, "response": result}

    def send(self, sender, data="0x", to=None):
        transaction = {"from": sender, "data": data, "gas": "0x100000"}
        if to is not None:
            transaction["to"] = to
        submission = self.rpc("eth_sendTransaction", [transaction])
        tx_hash = submission["response"]["result"]
        for _ in range(100):
            receipt = self.rpc("eth_getTransactionReceipt", [tx_hash])
            if receipt["response"]["result"] is not None:
                break
            time.sleep(0.05)
        else:
            raise RuntimeError(f"Transaction was not mined: {tx_hash}")
        if receipt["response"]["result"]["status"] != "0x1":
            raise RuntimeError(f"Transaction failed: {tx_hash}")
        height = receipt["response"]["result"]["blockNumber"]
        return {
            "submission": submission,
            "transaction": self.rpc("eth_getTransactionByHash", [tx_hash]),
            "receipt": receipt,
            "block": self.rpc("eth_getBlockByNumber", [height, False]),
            "trace": self.rpc("debug_traceTransaction", [tx_hash, {"tracer": "callTracer"}]),
        }

    def code(self, address, record):
        height = record["receipt"]["response"]["result"]["blockNumber"]
        return self.rpc("eth_getCode", [address, height])

    def record(self, name, account_index, creation=None, redeploy=False):
        sender = self.accounts[account_index + (5 if self.fork == "cancun" else 0)]
        runtime = "33ff"  # CALLER; SELFDESTRUCT
        init = constructor(runtime)
        records = []
        if creation is None:
            deployed = self.send(sender, init)
            address = deployed["receipt"]["response"]["result"]["contractAddress"]
            records.append(deployed)
            before = self.code(address, deployed)
            destroyed = self.send(sender, to=address)
        else:
            # Copy init code from calldata, CREATE/CREATE2, and either return the
            # child address or CALL it before the factory transaction commits.
            create = "3660006000f0" if creation == "create" else "60003660006000f5"
            finish = "60005260206000f3" if redeploy else "600052600060006000600060006000515af15000"
            factory = self.send(sender, constructor("366000600037" + create + finish))
            factory_address = factory["receipt"]["response"]["result"]["contractAddress"]
            deployed = self.send(sender, init, factory_address)
            create_trace = next(c for c in deployed["trace"]["response"]["result"]["calls"]
                                if c["type"].lower() == creation)
            address = create_trace["to"]
            records.extend([factory, deployed])
            before = self.code(address, deployed)
            destroyed = self.send(sender, to=address) if redeploy else deployed
        if destroyed is not records[-1]:
            records.append(destroyed)
        at_destruction = self.code(address, destroyed)
        if redeploy:
            records.append(self.send(sender, init, factory_address))
        current = self.code(address, records[-1])

        def calls(frame):
            yield frame
            for child in frame.get("calls", []):
                yield from calls(child)

        created = next(
            frame for record in records for frame in calls(record["trace"]["response"]["result"])
            if frame["type"] in ("CREATE", "CREATE2") and frame["to"] == address
        )
        output = {
            "producer": self.rpc("web3_clientVersion", []),
            "fork": self.fork,
            "name": name,
            "address": address,
            "cached_code": created["output"],
            "before": before,
            "at_destruction": at_destruction,
            "current": current,
            "destruction_block": destroyed["receipt"]["response"]["result"]["blockNumber"],
            "records": records,
        }
        path = pathlib.Path(__file__).parent / f"{self.fork}_{name}.json"
        path.write_text(json.dumps(output, indent=2) + "\n")

    def close(self):
        self.process.terminate()
        self.process.wait(timeout=10)


class GethHistoryRecorder(Recorder):
    """Capture one canonical chain spanning Cancun, including batching cases."""

    def __init__(self, executable):
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.port = sock.getsockname()[1]
        self.directory = tempfile.TemporaryDirectory(prefix="selfdestruct-geth-")
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.engine_port = sock.getsockname()[1]
        self.jwt = os.urandom(32)
        jwt_path = pathlib.Path(self.directory.name) / "jwt.hex"
        jwt_path.write_text(self.jwt.hex())
        self.activation = int(time.time()) + 40
        subprocess.run([executable, "--datadir", self.directory.name, "--lightkdf",
                        "account", "new", "--password", "/dev/null"],
                       check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        keyfile = next((pathlib.Path(self.directory.name) / "keystore").iterdir())
        from eth_account import Account
        self.signer = Account.from_key(Account.decrypt(keyfile.read_text(), ""))
        account = self.signer.address[2:].lower()
        genesis = json.loads(subprocess.check_output([executable, "--dev", "dumpgenesis"],
                                                     stderr=subprocess.PIPE))
        genesis["config"]["cancunTime"] = self.activation
        genesis["config"].pop("pragueTime", None)
        genesis["timestamp"] = hex(int(time.time()) - 5)
        genesis["alloc"][account] = {"balance": "0x100000000000000000000000"}
        genesis_path = pathlib.Path(self.directory.name) / "genesis.json"
        genesis_path.write_text(json.dumps(genesis))
        subprocess.run([executable, "--datadir", self.directory.name, "init", str(genesis_path)],
                       check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.process = subprocess.Popen(
            [executable, "--syncmode", "full", "--datadir", self.directory.name, "--http",
             "--http.addr", "127.0.0.1", "--http.port", str(self.port),
             "--http.api", "eth,web3,debug,admin", "--authrpc.addr", "127.0.0.1",
             "--authrpc.port", str(self.engine_port), "--authrpc.jwtsecret", str(jwt_path),
             "--ipcdisable", "--nodiscover",
             "--port", "0", "--maxpeers", "0", "--verbosity", "0"],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        )
        for _ in range(200):
            try:
                self.accounts = self.rpc("eth_accounts", [])['response']['result']
                break
            except (OSError, ValueError):
                time.sleep(0.05)
        else:
            raise RuntimeError(f"Geth did not start: {self.process.stderr.read().decode() if self.process.poll() is not None else self.process.poll()}")

    def rpc(self, method, params, allow_error=False):
        if method == "eth_sendTransaction":
            transaction = params[0]
            nonce = self.rpc("eth_getTransactionCount", [self.signer.address, "pending"])["response"]["result"]
            gas_price = self.rpc("eth_gasPrice", [])["response"]["result"]
            transaction = {"nonce": int(nonce, 16), "gasPrice": int(gas_price, 16),
                           "gas": int(transaction["gas"], 16), "data": transaction.get("data", "0x"),
                           "value": 0, "chainId": 1337,
                           **({"to": bytes.fromhex(transaction["to"][2:])} if "to" in transaction else {})}
            signed = self.signer.sign_transaction(transaction)
            method, params = "eth_sendRawTransaction", ["0x" + bytes(signed.raw_transaction).hex()]
        if not method.startswith("engine_"):
            return super().rpc(method, params, allow_error=allow_error)
        request = {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
        encode = lambda value: base64.urlsafe_b64encode(value).rstrip(b"=")
        token = encode(b'{"alg":"HS256","typ":"JWT"}') + b"." + encode(
            json.dumps({"iat": int(time.time())}).encode())
        token += b"." + encode(hmac.new(self.jwt, token, hashlib.sha256).digest())
        headers = {"Content-Type": "application/json", "Authorization": "Bearer " + token.decode()}
        with urllib.request.urlopen(urllib.request.Request(
            f"http://127.0.0.1:{self.engine_port}", json.dumps(request).encode(), headers
        ), timeout=10) as response:
            result = json.load(response)
        if "error" in result:
            raise RuntimeError(f"{method}: {result['error']}")
        return {"request": request, "response": result}

    def mine(self, timestamp=None):
        head = self.rpc("eth_getBlockByNumber", ["latest", False])["response"]["result"]
        timestamp = timestamp or int(head["timestamp"], 16) + 1
        cancun = timestamp >= self.activation
        version = "V3" if cancun else "V2"
        state = {"headBlockHash": head["hash"], "safeBlockHash": head["hash"],
                 "finalizedBlockHash": head["hash"]}
        attributes = {"timestamp": hex(timestamp), "prevRandao": "0x" + "00" * 32,
                      "suggestedFeeRecipient": self.accounts[0], "withdrawals": []}
        if cancun:
            attributes["parentBeaconBlockRoot"] = "0x" + "00" * 32
        time.sleep(0.05)
        result = self.rpc("engine_forkchoiceUpdated" + version, [state, attributes])["response"]["result"]
        if result["payloadStatus"]["status"] != "VALID":
            raise RuntimeError(result)
        envelope = self.rpc("engine_getPayload" + version, [result["payloadId"]])["response"]["result"]
        payload = envelope["executionPayload"]
        params = [payload, [], "0x" + "00" * 32] if cancun else [payload]
        status = self.rpc("engine_newPayload" + version, params)["response"]["result"]
        if status["status"] != "VALID":
            raise RuntimeError(status)
        state = dict.fromkeys(state, payload["blockHash"])
        self.rpc("engine_forkchoiceUpdated" + version, [state, None])

    def completed(self, submission):
        tx_hash = submission["response"]["result"]
        for _ in range(400):
            try:
                receipt = self.rpc("eth_getTransactionReceipt", [tx_hash])
                if receipt["response"]["result"] is not None:
                    break
            except RuntimeError as error:
                if "indexing" not in str(error):
                    raise
            time.sleep(0.05)
        else:
            raise RuntimeError(f"Transaction was not indexed: {tx_hash}")
        if receipt["response"]["result"]["status"] != "0x1":
            raise RuntimeError(f"Transaction failed: {tx_hash}")
        height = receipt["response"]["result"]["blockNumber"]
        return {
            "submission": submission,
            "transaction": self.rpc("eth_getTransactionByHash", [tx_hash]),
            "receipt": receipt,
            "block": self.rpc("eth_getBlockByNumber", [height, False]),
            "trace": self.rpc("debug_traceTransaction", [tx_hash, {"tracer": "callTracer"}]),
        }

    def send(self, sender, data="0x", to=None):
        params = {"from": sender, "data": data, "gas": "0x100000"}
        if to is not None:
            params["to"] = to
        submission = self.rpc("eth_sendTransaction", [params])
        self.mine()
        return self.completed(submission)

    def record_history(self):
        sender = self.accounts[0]
        records = []
        contracts = {}

        def send(data="0x", to=None):
            record = self.send(sender, data, to)
            records.append(record)
            return record

        for name in ("first", "second", "third", "survivor"):
            deployed = send(constructor("33ff"))
            address = deployed["receipt"]["response"]["result"]["contractAddress"]
            contracts[name] = {"address": address, "cached_code": "0x33ff",
                               "before": self.code(address, deployed)}

        submissions = [self.rpc("eth_sendTransaction", [{"from": sender, "to": contracts[name]["address"],
                                                        "gas": "0x100000"}])
                       for name in ("first", "second")]
        self.mine()
        destroyed = [self.completed(submission) for submission in submissions]
        if len({record["receipt"]["response"]["result"]["blockHash"] for record in destroyed}) != 1:
            raise RuntimeError("Batch transactions were not mined in the same block")
        records.extend(destroyed)
        for name, record in zip(("first", "second"), destroyed):
            contracts[name].update(destruction_block=record["block"]["response"]["result"]["number"],
                                   at_destruction=self.code(contracts[name]["address"], record))
        destroyed = send(to=contracts["third"]["address"])
        contracts["third"].update(destruction_block=destroyed["block"]["response"]["result"]["number"],
                                  at_destruction=self.code(contracts["third"]["address"], destroyed))

        factory = send(constructor("36600060003760003660006000f560005260206000f3"))
        factory_address = factory["receipt"]["response"]["result"]["contractAddress"]
        deployed = send(constructor("33ff"), factory_address)
        child = deployed["trace"]["response"]["result"]["calls"][0]["to"]
        before = self.code(child, deployed)
        first_destruction = send(to=child)
        send(constructor("33ff"), factory_address)
        destroyed = send(to=child)
        contracts["repeated"] = {
            "address": child, "cached_code": "0x33ff", "before": before,
            "destruction_block": destroyed["block"]["response"]["result"]["number"],
            "first_destruction_block": first_destruction["block"]["response"]["result"]["number"],
            "at_destruction": self.code(child, destroyed),
        }
        if int(destroyed["block"]["response"]["result"]["timestamp"], 16) >= self.activation:
            raise RuntimeError("Pre-Cancun history crossed the activation time")

        self.mine(self.activation)
        destroyed = send(to=contracts["survivor"]["address"])
        contracts["survivor"].update(
            destruction_block=destroyed["block"]["response"]["result"]["number"],
            at_destruction=self.code(contracts["survivor"]["address"], destroyed),
        )
        head = destroyed["block"]["response"]["result"]
        canonical = []
        for contract in contracts.values():
            contract["current"] = self.code(contract["address"], destroyed)
            canonical.append(self.rpc("eth_getCode", [contract["address"],
                                                     {"blockHash": head["hash"], "requireCanonical": True}]))
        missing_canonical = [
            self.rpc("eth_getCode", [contract["address"],
                     {"blockHash": "0x" + "ff" * 32, "requireCanonical": True}], allow_error=True)
            for contract in contracts.values()
        ]
        node = self.rpc("admin_nodeInfo", [])
        config = node["response"]["result"]["protocols"]["eth"]["config"]
        output = {"producer": self.rpc("web3_clientVersion", []), "fork": "mixed",
                  "name": "geth_history", **contracts["first"], "contracts": contracts,
                  "records": records, "canonical": canonical, "missing_canonical": missing_canonical,
                  "chain_spec": {"config": config}}
        path = pathlib.Path(__file__).parent / "geth_history.json"
        path.write_text(json.dumps(output, indent=2) + "\n")
        print(f"Recorded {len(records)} transactions spanning Cancun at {self.activation}")

    def close(self):
        super().close()
        self.directory.cleanup()


if __name__ == "__main__" and len(sys.argv) > 2 and sys.argv[2] == "--geth-history":
    recorder = GethHistoryRecorder(sys.argv[1])
    try:
        recorder.record_history()
    finally:
        recorder.close()
elif __name__ == "__main__":
    for fork in ("shanghai", "cancun"):
        recorder = Recorder(sys.argv[1], fork)
        try:
            recorder.record("existing", 0)
            recorder.record("same_transaction_create", 1, "create")
            recorder.record("same_transaction_create2", 2, "create2")
            if fork == "shanghai":
                recorder.record("redeployed", 3, "create2", redeploy=True)
        finally:
            recorder.close()
