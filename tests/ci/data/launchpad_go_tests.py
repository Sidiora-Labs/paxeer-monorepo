"""Run launchpad package tests with the reviewed, temporary failure baseline."""

import argparse
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile

BASELINE = Path(__file__).with_name("launchpad_go_test_baseline.json")
SHARED = "github.com/Sidiora-Technologies/KindleLaunch/shared/"
PROTOCOL = "github.com/Sidiora-Technologies/KindleLaunch/protocol"
API = "github.com/Sidiora-Technologies/KindleLaunch/core/api/"
API_PACKAGES = {API + "internal/" + name for name in (
    "app", "broker", "cache", "rest", "store"
)}

INDEXER = "github.com/Sidiora-Technologies/KindleLaunch/core/indexer/"
INDEXER_PACKAGES = {INDEXER + "internal/" + name for name in (
    "app", "chainreader", "config", "decode", "httpapi", "processor", "receipt", "runtimedocs", "source"
)}

PNL = "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/"
PNL_PACKAGES = {
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/cmd/pnld',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/app',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/migrate',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender/fonts',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlmath',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/referral',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store',
    'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/migrations',
}

SERVICE_PACKAGES = {
    'ranking-algo': {
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/cmd/rankingd',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/httpapi',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker',
        'github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store',
    },
    'stats-workers': {
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/cmd/statsd',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store',
        'github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/migrations',
    },
    'trading-charts': {
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/cmd/charts-measure',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/cmd/charts-rebuild',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/cmd/chartsd',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/integration',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/consumer',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/httpapi',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store',
        'github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/migrations',
    },
    'gateway': {
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/cmd/mediagatewayd',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cache',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/common',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/devidentity',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/image',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/serve',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage',
        'github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/upload',
    },
    'livestream': {
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/cmd/livestreamd',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/db/sqlcdb',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/migrations',
        'github.com/Sidiora-Technologies/KindleLaunch/media/livestream/pkg/types',
    },
    'metadata': {
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/cmd/metadatad',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/migrations',
        'github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types',
    },
    'social': {
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/cmd/sociald',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/common',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/social',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/migrations',
        'github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types',
    },
    'user': {
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/cmd/userd',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/db/sqlcdb',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/image',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/migrate',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/user',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/migrations',
        'github.com/Sidiora-Technologies/KindleLaunch/media/user/pkg/types',
    },
    'discovery': {
        'github.com/Sidiora-Technologies/KindleLaunch/core/discovery',
    },
    'notifications': {
        'github.com/Sidiora-Technologies/KindleLaunch/core/notifications',
        'github.com/Sidiora-Technologies/KindleLaunch/core/notifications/cmd/release-measure',
    },
    'security': {
        'github.com/Sidiora-Technologies/KindleLaunch/security/policy',
    },
    'security/control': {
        'github.com/Sidiora-Technologies/KindleLaunch/security/control',
    },
}
REMAINING_PACKAGES = set().union(*SERVICE_PACKAGES.values())


def load_baseline(path=BASELINE):
    entries = json.loads(Path(path).read_text())
    if not isinstance(entries, list):
        raise ValueError("baseline must be a list")
    seen = set()
    for entry in entries:
        if set(entry) != {"package", "test", "reason", "retiring_task"}:
            raise ValueError("invalid baseline fields")
        if entry["package"] not in {
            SHARED + suffix
            for suffix in ("auth", "chain", "db/production", "delivery",
                           "delivery/natsdriver", "storage")
        } | {PROTOCOL, PROTOCOL + "/bindings", PROTOCOL + "/coverage"} | API_PACKAGES | INDEXER_PACKAGES | PNL_PACKAGES | REMAINING_PACKAGES:
            raise ValueError("baseline package is outside the reviewed inventory")
        if not re.fullmatch(r"Test[A-Za-z0-9_]+", entry["test"]):
            raise ValueError("baseline must name one top-level test")
        if not isinstance(entry["reason"], str) or not entry["reason"].strip():
            raise ValueError("baseline requires a reason")
        if entry["package"] in REMAINING_PACKAGES:
            owner = "22.39.168"
        elif entry["package"] in INDEXER_PACKAGES:
            owner = "22.39.162"
        elif entry["package"] in API_PACKAGES | PNL_PACKAGES:
            owner = "22.39.154"
        else:
            owner = "22.39.117"
        if entry["retiring_task"] != owner:
            raise ValueError("baseline requires its retiring task")
        identity = entry["package"], entry["test"]
        if identity in seen:
            raise ValueError("duplicate baseline test")
        seen.add(identity)
    return entries


def skip_pattern(entries, package):
    names = sorted(entry["test"] for entry in entries if entry["package"] == package)
    return "^(" + "|".join(names) + ")$" if names else ""


def run_tests(module, go="go", flags=(), profile=None):
    entries = load_baseline()
    packages = subprocess.check_output(
        [go, "list", "./..."], cwd=module, text=True
    ).split()
    if not packages:
        return 0
    with tempfile.TemporaryDirectory(prefix="launchpad-coverage-") as temporary:
        profiles = []
        status = 0
        for index, package in enumerate(packages):
            command = [go, "test", *flags]
            pattern = skip_pattern(entries, package)
            if pattern:
                owners = sorted({entry["retiring_task"] for entry in entries
                                 if entry["package"] == package})
                print(f"BASELINE {package}: {pattern}; retiring task {', '.join(owners)}",
                      flush=True)
                command += ["-skip", pattern]
            if profile:
                part = Path(temporary) / f"{index}.out"
                profiles.append(part)
                command += ["-covermode=atomic", f"-coverprofile={part}"]
            result = subprocess.run([*command, package], cwd=module)
            if result.returncode:
                status = result.returncode
        if status:
            return status
        if profile:
            lines = ["mode: atomic"]
            for part in profiles:
                content = part.read_text().splitlines()
                if not content or content[0] != "mode: atomic":
                    raise ValueError(f"invalid package coverage profile: {part}")
                lines.extend(content[1:])
            Path(profile).write_text("\n".join(lines) + "\n")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--module", type=Path, required=True)
    parser.add_argument("--go", default="go")
    parser.add_argument("--flags", default="")
    parser.add_argument("--profile", type=Path)
    args = parser.parse_args()
    return run_tests(args.module, args.go, shlex.split(args.flags), args.profile)


if __name__ == "__main__":
    raise SystemExit(main())
