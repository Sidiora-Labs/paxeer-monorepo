import json
import subprocess
from contextlib import contextmanager
from pathlib import Path
from urllib.request import Request, urlopen


def exchange(url, body=None):
    request = Request(
        url,
        data=None if body is None else json.dumps(body).encode(),
        headers={} if body is None else {"Content-Type": "application/json"},
    )
    with urlopen(request, timeout=30) as response:
        return json.load(response)


@contextmanager
def node_peer():
    """The Python owner's gate starts the same real JS peer as the Node gate."""
    process = subprocess.Popen(
        ["node", str(Path(__file__).with_name("interop-peer.mjs"))],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        ready = process.stdout.readline()
        if not ready:
            raise RuntimeError(process.stderr.read())
        endpoint = json.loads(ready)["endpoint"]
        yield endpoint, exchange(endpoint + "fixture")
    finally:
        if process.poll() is None:
            process.stdin.write("close\n")
            process.stdin.flush()
        _, errors = process.communicate(timeout=30)
        if process.returncode != 0:
            raise RuntimeError(errors)
