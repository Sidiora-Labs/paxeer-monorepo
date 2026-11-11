"""Generate the local ABI union from the node's versioned signature registry.

Run without arguments to regenerate; --stdout emits the same ABI without writes.
The historical snapshot is the table served at audit 22.30.135.
"""

import argparse
import json
from pathlib import Path
import re
import sys

HERE = Path(__file__).resolve().parent
ROOT = next(parent for parent in HERE.parents if (parent / "node/app.go").is_file())


def generate():
    setup = (ROOT / "precompiles/setup.go").read_text()
    registry = setup.split("var precompileInfoSources =", 1)[1].split("\n}", 1)[0]
    sources = re.findall(r'"([^"\n]+)":\s*\{[^\n]*?,\s*"([^"\n]+)"\}', registry)
    if not sources:
        raise ValueError("node precompileInfoSources registry is missing or changed")
    inline_registry = setup.split("var mergeAgentInfoABIs =", 1)[1].split("\n}\n", 1)[0]
    inline = dict(re.findall(r'"([^"\n]+)":\s*`([^`]+)`', inline_registry))
    merge_table = setup.split("var MergeAddressTable =", 1)[1].split("\n}", 1)[0]
    merge_addresses = {name: address for address, name in re.findall(r'HexToAddress\("([^"]+)"\),\s*"([^"]+)"', merge_table)}
    merge_names = list(merge_addresses)
    if not merge_names or not inline or not set(inline).issubset(merge_names):
        raise ValueError("node merge address/ABI registry is missing or inconsistent")

    # This registry includes the versioned v6.12.0 replacements and additions.
    # Refuse to generate if a constructor used by the version switch lacks an ABI.
    app = (ROOT / "node/app.go").read_text()
    switch = app.split("func (app *App) v6120Contracts()", 1)[1].split("\n}\n", 1)[0]
    node_sources = "\n".join(path.read_text() for path in sorted((ROOT / "node").glob("*.go")) if not path.name.endswith("_test.go"))
    imports = dict(re.findall(r'(\w+)\s+"github.com/Sidiora-Labs/Paxeer-X-Network/(precompiles/[^"\n]+)"', node_sources))
    packages = {path.split("/")[0] for _, path in sources}
    custom = setup.split("func GetCustomPrecompilesE(", 1)[1].split("\n}\n", 1)[0]
    for package in set(re.findall(r'(\w+)\.GetVersioned\(', custom)):
        if package not in packages:
            raise ValueError(f"registered producer {package} has no ABI inventory entry")
    historical = []
    for alias in re.findall(r'(\w+)\.NewPrecompile(?:WithKeeper)?\(', switch):
        package = imports[alias]
        if "/legacy/" in package:
            historical.append(package + "/abi.json")
        elif package.removeprefix("precompiles/") not in packages:
            raise ValueError(f"versioned producer {package} has no signature registry entry")
    if "spotprecompile.NewServedSet(keepers)" not in switch:
        raise ValueError("node spot version switch changed; update inventory derivation")

    entries = []
    inventory = []

    def add(name, origin, abi, status):
        if not isinstance(abi, list):
            raise ValueError(f"{origin} must be an ABI array")
        entries.extend(item for item in abi if item["type"] in ("function", "event", "error"))
        inventory.append({"name": name, "origin": origin, "status": status})

    for name, path in sources:
        origin = "precompiles/" + path
        status = "historical" if path == "layerxexchange/abi.json" else "versioned"
        add(name, origin, json.loads((ROOT / origin).read_text()), status)
    for name in merge_names:
        if name in inline:
            # Read the served constructor's ABI, including wrapper-added events,
            # rather than assuming the node's cold metadata has identical coverage.
            matches = []
            for package in sorted((ROOT / "precompiles/layerxagent").iterdir()):
                files = [path for path in sorted(package.glob("*.go")) if not path.name.endswith("_test.go")]
                code = "\n".join(path.read_text() for path in files)
                if merge_addresses[name].lower() in code.lower() and "RegisterMergeConstructor" in code:
                    matches.append(files)
            if len(matches) != 1:
                raise ValueError(f"no unique served merge producer for {name}")
            files = matches[0]
            code = "\n".join(path.read_text() for path in files)
            readers = set(re.findall(r'abi.JSON\(strings.NewReader\((\w+)\)\)', code))
            found = set()
            for path in files:
                for symbol, raw in re.findall(r'(\w+)\s*=\s*`([^`]+)`', path.read_text()):
                    if symbol in readers:
                        add(name, str(path.relative_to(ROOT)) + "#" + symbol, json.loads(raw), "merge")
                        found.add(symbol)
            if not readers or readers != found:
                raise ValueError(f"unresolved served ABI constants for {name}: {readers - found}")
        else:
            origin = f"precompiles/{name}/abi.json"
            add(name, origin, json.loads((ROOT / origin).read_text()), "merge")
    for origin in sorted(set(historical)):
        add("version-switch predecessor", origin, json.loads((ROOT / origin).read_text()), "historical")
    add("audit-22.30.135 snapshot", "precompiles_historical.json", json.loads((HERE / "precompiles_historical.json").read_text()), "historical")

    # Keep distinct indexed layouts and overloads; canonical JSON only deduplicates
    # identical ABI records, never signatures with different input types.
    unique = {json.dumps(item, sort_keys=True, separators=(",", ":")): item for item in entries}
    abi = [unique[key] for key in sorted(unique)]
    return abi, inventory


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stdout", action="store_true")
    args = parser.parse_args()
    abi, inventory = generate()
    rendered = json.dumps({"inventory": inventory, "abi": abi}, indent=2) + "\n"
    if args.stdout:
        sys.stdout.write(rendered)
    else:
        (HERE / "precompiles.json").write_text(rendered)


if __name__ == "__main__":
    main()
