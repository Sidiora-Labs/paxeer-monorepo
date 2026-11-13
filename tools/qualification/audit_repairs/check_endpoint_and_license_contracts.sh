#!/usr/bin/env bash
set -euo pipefail

TASK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$TASK_ROOT"

python3 - <<'PY'
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import tomllib


def require(condition, message):
    if not condition:
        raise SystemExit(message)


deploy = Path("tools/hpx/hosting/deploy.sh")
stake = Path("tools/hpx/stake-fleet.sh")
config = Path("modules/evm/replay/config.go")
rosetta = Path("sdk/server/rosetta/client_online.go")
launchpad = Path("services/launchpad/LICENSE")
ranking = Path("sdk/launchpad/packages/ranking-algo-sdk/package.json")
cargo = Path("services/explorer/services/libs/blockscout-client/crate/Cargo.toml")
readme = Path("services/platform/perf-db/README.md")
paths = (deploy, stake, config, rosetta, launchpad, ranking, cargo, readme)
removed = (
    "31.220.74.140",
    "44.234.105.54",
    "wtf?",
    "contracts/LICENSE",
    "SEE LICENSE IN ../../../LICENSE",
    "Apache 2.0",
    "MIT OR Apache-2.0",
    "MIT_OR_Apache--2.0",
    "LICENSE-MIT",
)
for path in paths:
    text = path.read_text()
    for value in removed:
        require(value not in text, f"{path}: forbidden content {value!r}")

references = re.findall(r"`([^`]*LICENSE)`", launchpad.read_text())
require(references, f"{launchpad}: missing license file reference")
for reference in references:
    require((launchpad.parent / reference).is_file(),
            f"{launchpad}: license reference does not exist: {reference}")
require(json.loads(ranking.read_text())["license"] ==
        "LicenseRef-Paxlabs-HyperPax-OS-Protocol", "ranking SDK license differs from launchpad terms")
require(tomllib.loads(cargo.read_text())["package"]["license"] == "Apache-2.0",
        "blockscout-client license must use the Apache-2.0 SPDX identifier")
require("MIT" not in readme.read_text(), "PerfDB must declare Apache-2.0 only")
require("SPDX-License-Identifier: Apache-2.0" in readme.read_text(),
        "PerfDB must declare Apache-2.0")
require((readme.parent / "../../../LICENSE").is_file(), "PerfDB license file is missing")

config_text = config.read_text()
require(re.search(r'var DefaultConfig = Config\{[^}]*EthRPC:\s*""', config_text),
        "replay RPC default must be empty")
require(re.search(r'if cfg.Enabled && cfg.EthRPC == ""\s*\{\s*return cfg, fmt.Errorf\(',
                  config_text), "enabled replay must reject an empty EthRPC")

# Execute the scripts' actual configuration sections, stopping before deployment
# or staking operations. No network calls or host changes are needed for these checks.
cases = (
    (deploy, 'FONT_ORIGIN=', {
        "HPX_SEED_PEERS": "configured-peer",
        "HPX_STATESYNC_RPC": "configured-state-rpc",
        "HPX_STATESYNC_RPC_SERVERS": "configured-state-servers",
    }, ("SEED_PEERS", "STATESYNC_RPC", "STATESYNC_SERVERS")),
    (stake, 'readonly DENOM=', {
        "HPX_RPC": "configured-rpc",
        "HPX_EVM_RPC": "configured-evm-rpc",
    }, ("RPC", "EVM_RPC")),
)
with tempfile.NamedTemporaryFile(suffix=".env") as env_file:
    for path, boundary, required, assigned in cases:
        text = path.read_text()
        require(text.count(boundary) == 1, f"{path}: configuration boundary changed")
        prefix = text.split(boundary, 1)[0]
        env = {
            "PATH": os.environ["PATH"],
            "HPX_ENV_FILE": env_file.name,
            "HPX_SOURCE_REVISION": "0" * 40,
            **required,
        }
        command = ["bash", "-c", "source /dev/stdin"]
        for variable in required:
            for empty in (False, True):
                missing = env.copy()
                if empty:
                    missing[variable] = ""
                else:
                    del missing[variable]
                result = subprocess.run(command, input=prefix, text=True, env=missing,
                                        capture_output=True, check=False)
                require(result.returncode != 0 and variable in result.stderr,
                        f"{path}: must reject {'empty' if empty else 'unset'} {variable} by name: {result.stderr}")
        output = '\n'.join(f'printf "%s\\n" "${{{name}}}"' for name in assigned)
        result = subprocess.run(command, input=prefix + output, text=True, env=env,
                                capture_output=True, check=False)
        require(result.returncode == 0 and result.stdout.splitlines() == list(required.values()),
                f"{path}: must preserve explicitly configured endpoints: {result.stderr}")
print("22.31.601: endpoint requirements, removed content and license references verified")
PY
