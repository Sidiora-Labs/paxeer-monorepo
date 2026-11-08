import copy
import hashlib
import json
import os
import subprocess
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


class XwebConfigTest(unittest.TestCase):
    def test_render_emits_no_lx_route(self):
        root = Path(__file__).resolve().parents[3]
        assets = {
            symbol: {
                "asset_id": hashlib.sha256(symbol.encode()).hexdigest(),
                "symbol": symbol,
                "decimals": decimals,
                "paused": False,
                "verification": "authenticated_committed_snapshot",
            }
            for symbol, decimals in (("SID", 18), ("PAX", 18), ("USDC", 6), ("USDL", 6))
        }
        requests = []
        capture = json.loads((root / "services/platform/hosted/node/tests/fixtures/repair_22_31_355/native_configuration.json").read_text())
        history = next(record["result"] for record in capture["rpc_exchanges"]
                       if record["method"] == "px_getReceiptKeys")
        loader = Path(os.environ.get("XWEB_CONFIG_LOADER", str(root / "tools/bringup/tests/xweb-loader/target/debug/bringup-xweb-config-loader")))
        self.assertTrue(loader.is_file() and os.access(loader, os.X_OK),
                        "build tools/bringup/tests/xweb-loader before native loader qualification")

        def load_config(path, accepted=True):
            loaded = subprocess.run([str(loader), str(path)], cwd=root, capture_output=True,
                                    text=True, timeout=30, check=False)
            if accepted:
                self.assertEqual(loaded.returncode, 0, loaded.stdout + loaded.stderr)
                expected = [{"key_id": list(bytes.fromhex(key["keyId"][2:])),
                             "public_key": list(bytes.fromhex(key["publicKey"][2:])),
                             "first_height": key["firstHeight"], "last_height": key["lastHeight"]}
                            for key in history["keys"]]
                self.assertEqual(json.loads(loaded.stdout)["receipt_keys"], expected)
            else:
                self.assertNotEqual(loaded.returncode, 0, "native loader accepted altered configuration")
                self.assertIn("configuration refused:", loaded.stderr)


        class AssetHandler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append(request)
                response = {"jsonrpc": "2.0", "id": request["id"]}
                if request["method"] == "px_listAssets" and request["params"] == []:
                    response["result"] = {
                        "height": 1,
                        "assets": [
                            asset
                            for asset in reversed(list(assets.values()))
                        ],
                    }
                elif request["method"] == "px_getReceiptKeys" and request["params"] == []:
                    response["result"] = history
                else:
                    response["error"] = {"code": -32601, "message": "Method not found"}
                body = json.dumps(response).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, format, *args):
                pass

        env = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith("XWEB_") and key != "BRINGUP_HOSTS_FILE"
        }
        env.update(
            XWEB_SERVE_NAMES="api1.mainnet-beta.paxeer.network api2.mainnet-beta.paxeer.network",
            XWEB_SEEDS="https://paxeer.network/",
            XWEB_SID_PER_PAX="3.114",
            XWEB_PAX_USD_PRICE="13.44",
            NO_PROXY="127.0.0.1",
            no_proxy="127.0.0.1",
        )
        for symbol, asset in assets.items():
            env["XWEB_ASSET_ID_" + symbol] = asset["asset_id"]
            env["XWEB_DECIMALS_" + symbol] = str(asset["decimals"])
        with tempfile.TemporaryDirectory() as directory, HTTPServer(
            ("127.0.0.1", 0), AssetHandler
        ) as server:
            env["XWEB_GATEWAY_ENDPOINT"] = f"http://127.0.0.1:{server.server_port}/rpc"
            env["XWEB_RECEIPT_KEYS_ENDPOINT"] = f"http://127.0.0.1:{server.server_port}/"
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                rendered = subprocess.run(
                    ["bash", "tools/bringup/xweb-config.sh", "render", "serve", directory],
                    cwd=root,
                    env=env,
                    capture_output=True,
                    text=True,
                    timeout=120,
                    check=False,
                )
                attestor_dir = Path(directory) / "attestors"
                attestors = subprocess.run(
                    ["bash", "tools/bringup/xweb-config.sh", "render", "attestor", str(attestor_dir)],
                    cwd=root, env=env, capture_output=True, text=True, timeout=120, check=False)
                self.assertEqual(attestors.returncode, 0, attestors.stdout + attestors.stderr)
                self.assertEqual(len(list(attestor_dir.glob("*.json"))), 4)
                for path in attestor_dir.glob("*.json"):
                    load_config(path)
            finally:
                server.shutdown()
                thread.join()
            retired = subprocess.run(
                [
                    "python3",
                    "tools/paxeer-x/tests/retired_callers_test.py",
                    "--component",
                    "kernel-web",
                    "--path",
                    "tools/bringup/xweb-config.sh",
                ],
                cwd=root,
                capture_output=True,
                text=True,
                timeout=60,
                check=False,
            )

            with self.subTest(command="render serve"):
                self.assertEqual(
                    rendered.returncode, 0, rendered.stdout + rendered.stderr
                )
                files = sorted(Path(directory).glob("*.json"))
                self.assertTrue(files, "render serve must write configuration files")
                self.assertEqual(
                    {path.name for path in files}, {"api1.mainnet-beta.paxeer.network.json", "api2.mainnet-beta.paxeer.network.json"}
                )
                for path in files:
                    self.assertNotIn("lx_", path.read_text(encoding="utf-8"), str(path))
                    config = json.loads(path.read_text(encoding="utf-8"))
                    self.assertEqual(config["gateway"]["endpoint"], env["XWEB_GATEWAY_ENDPOINT"])
                    self.assertEqual(config["evm"]["endpoint"], "http://127.0.0.1:8545")
                    self.assertEqual(set(config["assets"]), set(assets))
                    for symbol, asset in assets.items():
                        self.assertEqual(
                            config["assets"][symbol]["asset_id"], asset["asset_id"]
                        )
                    other = "api2.mainnet-beta.paxeer.network" if path.stem == "api1.mainnet-beta.paxeer.network" else "api1.mainnet-beta.paxeer.network"
                    self.assertEqual(config["peers"], [f"https://{other}"])
                    load_config(path)
                    for alteration in ("history", "unsupported", "legacy", "weak-key"):
                        changed = copy.deepcopy(config)
                        if alteration == "history":
                            entry = changed["gateway"]["receipt_keys"][0]
                            entry["first_height"] = entry["last_height"] + 1
                        elif alteration == "unsupported":
                            changed["kernel"] = {"endpoint": env["XWEB_GATEWAY_ENDPOINT"]}
                        elif alteration == "legacy":
                            changed["gateway"]["sequencer_id"] = "ab" * 32
                        else:
                            changed["gateway"]["receipt_keys"][0]["public_key"] = "00" * 32
                        altered = Path(directory) / "refused.json"
                        altered.write_text(json.dumps(changed))
                        load_config(altered, accepted=False)
                        altered.unlink()
                self.assertTrue(requests, "render serve must query the asset gateway")
                self.assertEqual({request["method"] for request in requests}, {"px_listAssets", "px_getReceiptKeys"})
                for request in requests:
                    self.assertIn(request["method"], ("px_listAssets", "px_getReceiptKeys"))
                    self.assertEqual(request["params"], [])
                self.assertNotIn(
                    "lx_", (root / "tools/bringup/xweb-config.sh").read_text()
                )

            with self.subTest(command="retired callers"):
                self.assertEqual(retired.returncode, 0, retired.stdout + retired.stderr)
