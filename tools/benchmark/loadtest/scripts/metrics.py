"""Python compatibility API for the production benchmark metrics client."""

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path

import requests

# File-based imports used by the historical transcript suite do not add this
# script's directory to sys.path.
sys.path.insert(0, str(Path(__file__).resolve().parent))
from operations_cli import request

node_base_url = "localhost"
rpc_base_url = "http://localhost:26657"


def _request(operation, **values):
    payload = {"operation": operation, "rpc": rpc_base_url,
               "rest": f"http://{node_base_url}:1317", **values}
    completed = request("metric", json.dumps(payload))
    if not completed.stdout:
        raise RuntimeError(completed.stderr or "benchmark operations returned no response")
    envelope = json.loads(completed.stdout)
    error = envelope.get("error")
    if error:
        if error["kind"] == "http":
            response = requests.Response()
            response.status_code = error["status"]
            response.url = error["url"]
            raise requests.HTTPError(error["message"], response=response)
        if error["kind"] == "rpc":
            raise RuntimeError(error["message"])
        raise ValueError(error["message"])
    if completed.returncode:
        raise RuntimeError(completed.stderr)
    return envelope["result"]


def get_block_height(tx_hash_str):
    return _request("height", hash=tx_hash_str)


def get_all_heights():
    filename = Path.home() / "outputs/test_tx_hash"
    with filename.open() as stream:
        heights = [get_block_height(value.strip()) for value in stream if value.strip()]
    return sorted({height for height in heights if height is not None})


def get_block(height):
    return _request("block", height=height)


def get_block_info(height):
    info = _request("info", height=height)
    info["timestamp"] = datetime.fromisoformat(info["timestamp"].replace("Z", "+00:00"))
    return info


def get_block_time(height):
    return get_block_info(height)["timestamp"]


def get_transaction_breakdown(height):
    return _request("breakdown", height=height)


def get_best_block_stats(block_info_list):
    blocks = [{**block, "timestamp": block["timestamp"].isoformat()} for block in block_info_list]
    return tuple(_request("best", blocks=blocks))


def get_metrics():
    return _request("metrics", path=str(Path.home() / "outputs/test_tx_hash"))


def main():
    global node_base_url, rpc_base_url
    parser = argparse.ArgumentParser()
    parser.add_argument("--node", default="localhost")
    args = parser.parse_args()
    node_base_url = args.node
    rpc_base_url = f"http://{node_base_url}:26657"
    print(json.dumps(get_metrics(), indent=4))


if __name__ == "__main__":
    main()
