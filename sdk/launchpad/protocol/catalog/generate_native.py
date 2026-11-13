"""Copy the native launchpad ABI into the protocol catalog."""

import json
from pathlib import Path


catalog = Path(__file__).resolve().parent
source = catalog.parents[3] / "precompiles/launchpad/abi.json"
raw = source.read_bytes()
entries = json.loads(raw)
if not any(entry.get("type") == "event" and entry.get("name") == "ChainLaunched" for entry in entries):
    raise ValueError("ChainLaunched event is absent from the native launchpad ABI")
if not any(entry.get("type") == "function" and entry.get("name") == "launchChain" for entry in entries):
    raise ValueError("launchChain function is absent from the native launchpad ABI")
(catalog / "native_launchpad.json").write_bytes(raw)
