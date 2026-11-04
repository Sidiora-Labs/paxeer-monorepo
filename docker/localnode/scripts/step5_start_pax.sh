#!/usr/bin/env bash
set -euo pipefail
# Own paxd until exit. Check loopback chain identity/readiness before publication,
# revoke the per-node launch marker on health loss, and propagate failure to deploy.
exec python3 docker/localnode/scripts/localnode_runtime.py supervise
