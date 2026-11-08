#!/usr/bin/env bash
# Checks workflow syntax, hosted fallbacks and retired runner labels.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

mapfile -t files < <(printf '%s\n' .github/workflows/*.yml .github/workflows/*.yaml | grep -Ev '/paxeer-release-(publish|check)\.yml$|\*' || true)
[ "${#files[@]}" -gt 0 ] || { echo "no workflows found" >&2; exit 1; }

python3 - "${files[@]}" <<'PY'
from pathlib import Path
import re, sys, yaml

routed = re.compile(
    r"^\$\{\{ \(github\.event_name == 'pull_request' && "
    r"github\.event\.pull_request\.head\.repo\.full_name != github\.repository\) && "
    r"'(ubuntu-24\.04|ubuntu-latest)' \|\| vars\.CI_LINUX_RUNNER \|\| "
    r"'\1' \}\}$"
)
stale = re.compile(r"fly-linux|self-hosted|layerx-testnet|layerx-b200")
live_mirrors = {
    ("mirror-live.yml", "publish-retrieve"),
    ("mirror-verification-live.yml", "mirror-only"),
}
bad = 0

def labels(v):
    return v if isinstance(v, list) else [v]

for path in sys.argv[1:]:
    try:
        doc = yaml.safe_load(open(path))
    except yaml.YAMLError as e:
        print(f"{path}: YAML does not parse: {e}"); bad += 1; continue
    jobs = (doc or {}).get("jobs") or {}
    if not isinstance(jobs, dict):
        print(f"{path}: jobs is not a mapping"); bad += 1; continue
    for name, job in jobs.items():
        if not isinstance(job, dict) or "runs-on" not in job:
            continue
        if (Path(path).name, name) in live_mirrors:
            if job["runs-on"] != ["self-hosted", "layerx-testnet"]:
                print(f"{path}: job {name}: live mirror runner labels changed"); bad += 1
            continue
        for label in labels(job["runs-on"]):
            label = str(label)
            if stale.search(label):
                print(f"{path}: job {name}: stale runner label: {label}"); bad += 1
            elif "CI_LINUX_RUNNER" in label and not routed.fullmatch(label):
                print(f"{path}: job {name}: CI_LINUX_RUNNER without matching hosted Ubuntu fallbacks: {label}"); bad += 1
    for n, line in enumerate(open(path), 1):
        if stale.search(line) and "runs-on" in line:
            if Path(path).name in {filename for filename, _ in live_mirrors} and line.strip() == "runs-on: [self-hosted, layerx-testnet]":
                continue
            print(f"{path}:{n}: stale runner label: {line.strip()}"); bad += 1

print(f"checked {len(sys.argv) - 1} workflows, {bad} problem(s)")
sys.exit(1 if bad else 0)
PY
