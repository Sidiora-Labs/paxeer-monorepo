"""Shared integration-image identity and fail-closed GHCR retention."""

import argparse
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import re
import urllib.request

CONFIG = Path(__file__).resolve().parents[1] / "integration-registry.json"


def registry_config():
    config = json.loads(CONFIG.read_text(encoding="utf-8"))
    if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", config["owner"]):
        raise ValueError("invalid registry owner")
    if set(config["packages"]) != {"localnode", "rpcnode"}:
        raise ValueError("invalid integration package roles")
    for package in config["packages"].values():
        if not re.fullmatch(r"pax-chain-integration-test-(localnode|rpcnode)", package):
            raise ValueError("invalid integration package")
    return config


def image_environment(config, repository, run_id):
    if repository.split("/")[0].lower() != config["owner"]:
        raise ValueError("repository token owner differs from registry owner")
    if not re.fullmatch(r"[1-9][0-9]*", str(run_id)):
        raise ValueError("invalid workflow run id")
    result = {"GHCR_OWNER": config["owner"]}
    for role, package in config["packages"].items():
        name = f"GHCR_{role.upper()}"
        result[name] = f"ghcr.io/{config['owner']}/{package}"
        result[name + "_RUN_IMAGE"] = result[name] + ":" + str(run_id)
    return result


class RegistryAPI:
    def __init__(self, token, base="https://api.github.com"):
        self.token = token
        self.base = base.rstrip("/")

    def request(self, path, method="GET"):
        request = urllib.request.Request(
            self.base + path, method=method,
            headers={
                "Authorization": "Bearer " + self.token,
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read()
        return json.loads(body) if body else None

    def versions(self, owner, package):
        page = 1
        while True:
            versions = self.request(
                f"/orgs/{owner}/packages/container/{package}/versions?per_page=100&page={page}"
            )
            if not isinstance(versions, list):
                raise ValueError("invalid package versions response")
            yield from versions
            if len(versions) < 100:
                break
            page += 1


def cleanup_candidates(config, repository, versions, runs, cutoff):
    # Validate every candidate before a caller can delete any version.
    image_environment(config, repository, "1")
    candidates = []
    for package in config["packages"].values():
        for version in versions[package]:
            tags = version["metadata"]["container"]["tags"]
            if not tags or not all(re.fullmatch(r"[1-9][0-9]*", tag) for tag in tags):
                continue
            created = datetime.fromisoformat(version["created_at"].replace("Z", "+00:00"))
            if created >= cutoff:
                continue
            version_id = version["id"]
            if not isinstance(version_id, int) or isinstance(version_id, bool) or version_id <= 0:
                raise ValueError("invalid package version id")
            for tag in tags:
                run = runs[tag]
                if (
                    str(run["id"]) != tag
                    or run["repository"]["full_name"].lower() != repository.lower()
                    or run["path"] != config["publisher_workflow"]
                    or run["status"] != "completed"
                ):
                    raise ValueError("tag does not identify a completed integration publisher run")
            candidates.append({"package": package, "version_id": version_id, "tags": tags})
    return candidates


def cleanup(config, repository, api, older_than_days, dry_run, now, output):
    if not isinstance(older_than_days, int) or older_than_days < 1:
        raise ValueError("retention days must be a positive integer")
    image_environment(config, repository, "1")
    cutoff = now - timedelta(days=older_than_days)
    versions = {
        package: list(api.versions(config["owner"], package))
        for package in config["packages"].values()
    }
    run_ids = {
        tag
        for entries in versions.values() for version in entries
        if version["metadata"]["container"]["tags"]
        and all(re.fullmatch(r"[1-9][0-9]*", tag) for tag in version["metadata"]["container"]["tags"])
        and datetime.fromisoformat(version["created_at"].replace("Z", "+00:00")) < cutoff
        for tag in version["metadata"]["container"]["tags"]
    }
    runs = {
        tag: api.request(f"/repos/{repository}/actions/runs/{tag}")
        for tag in sorted(run_ids)
    }
    candidates = cleanup_candidates(config, repository, versions, runs, cutoff)
    for candidate in candidates:
        print(json.dumps(dict(candidate, dry_run=dry_run), sort_keys=True), file=output)
    output.flush()
    if not dry_run:
        for candidate in candidates:
            api.request(
                f"/orgs/{config['owner']}/packages/container/{candidate['package']}"
                f"/versions/{candidate['version_id']}", method="DELETE",
            )
    return candidates


def main():
    import os
    import sys

    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("environment", "cleanup"))
    args = parser.parse_args()
    config = registry_config()
    repository = os.environ["GITHUB_REPOSITORY"]
    if args.mode == "environment":
        values = image_environment(config, repository, os.environ["GITHUB_RUN_ID"])
        with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as output:
            for key, value in values.items():
                print(f"{key}={value}", file=output)
    else:
        dry_run = os.environ["DRY_RUN"]
        if dry_run not in ("true", "false"):
            raise ValueError("dry-run must be true or false")
        cleanup(
            config, repository, RegistryAPI(os.environ["GH_TOKEN"]),
            int(os.environ["OLDER_THAN_DAYS"]), dry_run == "true",
            datetime.now(timezone.utc), sys.stdout,
        )


if __name__ == "__main__":
    main()
