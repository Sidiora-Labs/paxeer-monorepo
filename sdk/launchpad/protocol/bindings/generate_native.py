"""Generate the native launchpad binding input from the producer ABI."""

import json
from pathlib import Path


bindings = Path(__file__).resolve().parent
source = bindings.parents[3] / "precompiles/launchpad/abi.json"
methods = {
    "launchChain", "launchChainWithPermit", "openChainRevenueEpoch", "claimChainRevenue",
    "claimChainRevenueBatch", "pauseLaunchedChain", "resumeLaunchedChain", "retireLaunchedChain",
    "launchedChain", "launchedChainOfPool", "homeChainOf", "chainRevenueEpoch", "chainRevenueClaimable",
}
entries = json.loads(source.read_text())
present = {entry["name"] for entry in entries if entry["type"] == "function" and entry["name"] in methods}
if present != methods:
    raise ValueError("chain launch function inventory differs from the binding input")
selected = [
    entry for entry in entries
    if entry["type"] in {"event", "error"} or
    (entry["type"] == "function" and entry["name"] in methods)
]
(bindings / "abi/native_launchpad.json").write_text(json.dumps(selected, indent=2) + "\n")
