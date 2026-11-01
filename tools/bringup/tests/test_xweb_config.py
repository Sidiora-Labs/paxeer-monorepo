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

        class AssetHandler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append(request)
                response = {"jsonrpc": "2.0", "id": request["id"]}
                if request["method"] == "px_listAssets" and request["params"] == []:
                    response["result"] = {
                        "height": 1,
                        "assets": [
                            {"asset_id": asset["asset_id"], "layerx": asset}
                            for asset in reversed(list(assets.values()))
                        ],
                    }
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
            XWEB_SERVE_NAMES="api1 api2",
            XWEB_SEQUENCER_ID=hashlib.sha256(b"sequencer").hexdigest(),
            XWEB_SEQUENCER_PUBLIC_KEY=hashlib.sha256(b"sequencer-public-key").hexdigest(),
            XWEB_SEEDS="https://example.com/",
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
                files = [path for path in Path(directory).rglob("*") if path.is_file()]
                self.assertTrue(files, "render serve must write configuration files")
                self.assertEqual(
                    {path.name for path in files}, {"api1.json", "api2.json"}
                )
                for path in files:
                    self.assertNotIn("lx_", path.read_text(encoding="utf-8"), str(path))
                    config = json.loads(path.read_text(encoding="utf-8"))
                    self.assertEqual(set(config["assets"]), set(assets))
                    for symbol, asset in assets.items():
                        self.assertEqual(
                            config["assets"][symbol]["asset_id"], asset["asset_id"]
                        )
                    other = "api2" if path.stem == "api1" else "api1"
                    self.assertEqual(config["peers"], [f"https://{other}"])
                self.assertTrue(requests, "render serve must query the asset gateway")
                for request in requests:
                    self.assertEqual(request["method"], "px_listAssets")
                    self.assertEqual(request["params"], [])
                self.assertNotIn(
                    "lx_", (root / "tools/bringup/xweb-config.sh").read_text()
                )

            with self.subTest(command="retired callers"):
                self.assertEqual(retired.returncode, 0, retired.stdout + retired.stderr)
