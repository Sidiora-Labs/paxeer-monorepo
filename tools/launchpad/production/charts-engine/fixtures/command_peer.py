"""Diagnostic subprocess peer; reports real process activity over loopback HTTP.

The measurement role deliberately exits after the controllers finish: this peer
does not produce chart measurements or claim provider qualification.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import sys
import time
from urllib.request import Request, urlopen


def exchange(url, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    request = Request(url, data=data, headers={"Content-Type": "application/json"})
    with urlopen(request, timeout=5) as response:
        return json.load(response)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input")
    parser.add_argument("--input-sha256")
    parser.add_argument("--config")
    parser.add_argument("--config-sha256")
    parser.add_argument("--output")
    parser.add_argument("--release", action="store_true")
    args = parser.parse_args()
    path = args.input or args.config
    expected = args.input_sha256 or args.config_sha256
    raw = Path(path).read_bytes()
    if hashlib.sha256(raw).hexdigest() != expected:
        raise ValueError("peer input digest differs")
    document = json.loads(raw)
    kind = document["kind"] if args.input else "measurement"
    receipt = {
        "kind": kind,
        "pid": os.getpid(),
        "inputSha256": expected,
        "environment": os.environ["CHART_PEER_TOKEN"],
        "release": args.release,
        "output": args.output,
    }
    url = os.environ["CHART_PEER_URL"]
    instruction = exchange(url + "/begin", receipt)
    print(json.dumps(receipt, sort_keys=True), flush=True)
    for stream, field, byte in (
        (sys.stdout.buffer, "stdoutBytes", b"o"),
        (sys.stderr.buffer, "stderrBytes", b"e"),
    ):
        stream.write(byte * instruction[field])
        stream.flush()
    time.sleep(instruction["sleepSeconds"])
    if kind == "measurement":
        while not exchange(url + "/finished")["finished"]:
            time.sleep(0.01)
        print("diagnostic peer: controllers finished; no provider measurement", flush=True)
        return 7
    exchange(url + "/end", receipt)
    return 0


if __name__ == "__main__":
    sys.exit(main())
