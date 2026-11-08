"""Offline source qualification of wallet automation migration sources.

Trace the actual workflow/image/artifact contracts without executing Actions,
Docker, pnpm, bash or a deployment. Root registration remains an explicit request.
"""

import json
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
WORKFLOWS = ROOT / "apps/wallet/.github/workflows"
RECORD_PATH = ROOT / "tools/qualification/audit_repairs/repair_22_33_150.kvx"


class WorkflowLoader(yaml.SafeLoader):
    """Keep Actions' `on` key while retaining actual boolean scalar types."""

    yaml_implicit_resolvers = {
        key: [(tag, pattern) for tag, pattern in values if not tag.endswith(":bool")]
        for key, values in yaml.SafeLoader.yaml_implicit_resolvers.items()
    }


WorkflowLoader.add_implicit_resolver(
    "tag:yaml.org,2002:bool", re.compile(r"^(?:true|false)$", re.I), list("tTfF")
)


def workflow(name):
    return yaml.load((WORKFLOWS / name).read_text(), Loader=WorkflowLoader)


def action(job, name):
    steps = [step for step in job["steps"] if step.get("uses") == name]
    assert len(steps) == 1, name
    return steps[0]


def commands(job):
    return [step["run"] for step in job["steps"] if "run" in step]


def test_ci_discovery_wallet_pnpm_checks_gate_production_image():
    ci = workflow("ci.yml")
    assert set(ci["on"]) == {"push", "pull_request", "workflow_dispatch", "workflow_call"}
    validate = ci["jobs"]["validate"]
    assert validate["defaults"]["run"] == {
        "working-directory": "apps/wallet", "shell": "bash"
    }
    manifest = json.loads((ROOT / "apps/wallet/package.json").read_text())
    assert manifest["packageManager"].startswith("pnpm@")
    assert manifest["dependencies"]["@paxeer/wallet"] == "link:../../sdk/wallet"
    assert action(validate, "pnpm/action-setup@v4")["with"] == {
        "package_json_file": "apps/wallet/package.json"
    }
    setup = action(validate, "actions/setup-node@v4")["with"]
    assert setup["node-version"].startswith("22.")
    assert setup["cache"] == "pnpm"
    locks = setup["cache-dependency-path"].splitlines()
    assert locks == ["apps/wallet/pnpm-lock.yaml", "services/wallet/pnpm-lock.yaml"]
    for lock in locks:
        parsed = yaml.load((ROOT / lock).read_text(), Loader=WorkflowLoader)
        assert parsed["lockfileVersion"] and parsed["importers"]
    assert commands(validate) == [
        "pnpm install --frozen-lockfile --filter ../../sdk/wallet...",
        "pnpm --filter ../../sdk/wallet... run build",
        "pnpm install --frozen-lockfile",
        "pnpm run lint", "pnpm run type-check", "pnpm test", "pnpm run build",
    ]
    for step in validate["steps"]:
        if "run" in step:
            assert step.get("working-directory", "apps/wallet") == (
                "services/wallet" if "../../sdk/wallet" in step["run"] else "apps/wallet"
            )
    for script in ("lint", "type-check", "test", "build"):
        assert manifest["scripts"][script]
    assert ci["jobs"]["image"] == {
        "needs": "validate", "uses": "./.github/workflows/wallet-nextjs.yml"
    }


def test_pages_discovery_builds_actual_standalone_image_and_transports_it():
    image = workflow("nextjs.yml")
    assert set(image["on"]) == {"workflow_call", "workflow_dispatch"}
    job = image["jobs"]["build"]
    build = action(job, "docker/build-push-action@v6")["with"]
    assert build["context"] == "."
    assert build["file"] == "docker/wallet-pwa/Dockerfile"
    assert build["target"] == "runner" and build["push"] is False
    assert build["tags"] == "wallet-pwa:${{ github.sha }}"
    assert build["outputs"] == "type=docker,dest=/tmp/wallet-image.tar"
    dockerfile = (ROOT / build["file"]).read_text()
    instructions = [shlex.split(line) for line in dockerfile.splitlines()
                    if line.startswith(("FROM ", "WORKDIR ", "COPY ", "CMD "))]
    assert ["FROM", "node:22.22.0-alpine3.22@sha256:7aa86fa052f6e4b101557ccb56717cb4311be1334381f526fe013418fe157384", "AS", "runner"] in instructions
    assert ["WORKDIR", "/src/apps/wallet"] in instructions
    for source, destination in (
        ("/src/apps/wallet/.next/standalone", "./"),
        ("/src/apps/wallet/public", "./apps/wallet/public"),
        ("/src/apps/wallet/.next/static", "./apps/wallet/.next/static"),
    ):
        assert any(row[0] == "COPY" and "--from=builder" in row
                   and row[-2:] == [source, destination] for row in instructions)
    assert 'CMD ["/app/start-wallet.sh"]' in dockerfile
    assert "output: 'standalone'" in (ROOT / "apps/wallet/next.config.mjs").read_text()
    startup = (ROOT / "apps/wallet/deployment/start-wallet.sh").read_text()
    assert "node /app/apps/wallet/server.js" in startup
    assert "nginx -e /dev/stderr" in startup
    declared = dict((key, default) for key, default in re.findall(
        r"^ARG (NEXT_PUBLIC_\w+)(?:=(.*))?$", dockerfile, re.M
    ))
    supplied = dict(line.split("=", 1) for line in build["build-args"].splitlines())
    assert supplied == {
        key: default or "${{ vars." + key + " }}" for key, default in declared.items()
    }
    upload = action(job, "actions/upload-artifact@v4")["with"]
    assert upload == {
        "name": "wallet-image-${{ github.sha }}", "path": "/tmp/wallet-image.tar",
        "if-no-files-found": "error", "retention-days": 1,
    }
    source = (WORKFLOWS / "nextjs.yml").read_text()
    for obsolete in ("./out", "actions/configure-pages", "actions/deploy-pages",
                     "actions/upload-pages-artifact"):
        assert obsolete not in source


def test_deploy_discovery_consumes_checked_image_and_publishes_immutable_digest():
    deploy = workflow("deploy.yml")
    assert deploy["on"]["push"]["branches"] == ["main"]
    assert deploy["permissions"]["packages"] == "write"
    assert deploy["concurrency"] == {
        "group": "wallet-production", "cancel-in-progress": False
    }
    jobs = deploy["jobs"]
    assert jobs["checks"]["uses"] == "./.github/workflows/wallet-ci.yml"
    assert jobs["security"]["uses"] == "./.github/workflows/wallet-security.yml"
    publish = jobs["publish"]
    assert publish["needs"] == ["checks", "security"]
    assert publish["environment"] == "production"
    download = action(publish, "actions/download-artifact@v4")["with"]
    producer = action(workflow("nextjs.yml")["jobs"]["build"],
                      "actions/upload-artifact@v4")["with"]
    assert download["name"] == producer["name"]
    assert download["path"] + "/" + Path(producer["path"]).name == (
        "/tmp/wallet-image/wallet-image.tar"
    )
    assert action(publish, "docker/login-action@v3")["with"] == {
        "registry": "ghcr.io", "username": "${{ github.actor }}",
        "password": "${{ secrets.GITHUB_TOKEN }}",
    }
    script_steps = [step for step in publish["steps"] if "run" in step]
    assert len(script_steps) == 1
    assert script_steps[0]["env"] == {"IMAGE_REVISION": "${{ github.sha }}"}
    # Parse every real command in-process; no subprocess or production execution.
    tokens = [shlex.split(line) for line in script_steps[0]["run"].splitlines()]
    assert tokens == [
        ["set", "-euo", "pipefail"],
        ["image=ghcr.io/${GITHUB_REPOSITORY_OWNER,,}/wallet-pwa:${IMAGE_REVISION}"],
        ["docker", "load", "--input", "/tmp/wallet-image/wallet-image.tar"],
        ["docker", "tag", "wallet-pwa:${IMAGE_REVISION}", "$image"],
        ["docker", "push", "$image"],
        ["docker", "image", "inspect", "$image", "--format",
         "{{index .RepoDigests 0}}", ">", "wallet-image-digest.txt"],
        ["test", "-s", "wallet-image-digest.txt"],
        ["grep", "-Ex", "ghcr.io/[^/]+/wallet-pwa@sha256:[0-9a-f]{64}",
         "wallet-image-digest.txt"],
        ["cat", "wallet-image-digest.txt", ">>", "$GITHUB_STEP_SUMMARY"],
    ]
    upload = action(publish, "actions/upload-artifact@v4")["with"]
    assert upload == {
        "name": "wallet-image-digest-${{ github.sha }}",
        "path": "wallet-image-digest.txt", "if-no-files-found": "error",
    }
    assert "DEPLOY_KEY" not in (WORKFLOWS / "deploy.yml").read_text()
    assert "Add your deployment commands here" not in (WORKFLOWS / "deploy.yml").read_text()


def test_security_discovery_audit_codeql_and_dependency_review_are_blocking():
    security = workflow("security.yml")
    assert set(security["on"]) == {
        "push", "pull_request", "schedule", "workflow_dispatch", "workflow_call"
    }
    assert security["on"]["schedule"] == [{"cron": "0 0 * * 0"}]
    jobs = security["jobs"]
    audit = jobs["audit"]
    assert audit["defaults"]["run"] == {
        "working-directory": "apps/wallet", "shell": "bash"
    }
    assert action(audit, "pnpm/action-setup@v4")["with"]["package_json_file"] == (
        "apps/wallet/package.json"
    )
    setup = action(audit, "actions/setup-node@v4")["with"]
    assert setup["cache"] == "pnpm"
    assert setup["cache-dependency-path"] == "apps/wallet/pnpm-lock.yaml"
    assert commands(audit) == [
        "pnpm install --frozen-lockfile", "pnpm audit --audit-level=moderate"
    ]
    codeql = jobs["codeql"]
    assert "if" not in codeql
    assert codeql["permissions"]["security-events"] == "write"
    init = action(codeql, "github/codeql-action/init@v3")["with"]
    assert init["languages"] == "javascript-typescript"
    assert init["build-mode"] == "none"
    assert yaml.safe_load(init["config"]) == {
        "paths": ["apps/wallet", "sdk/wallet", "services/agent/sdk/typescript"]
    }
    action(codeql, "github/codeql-action/analyze@v3")
    review = jobs["dependency-review"]
    assert review["if"] == "github.event_name == 'pull_request'"
    action(review, "actions/dependency-review-action@v4")


def test_all_workflows_surface_failures_and_watch_real_dependency_paths():
    for name in ("ci.yml", "deploy.yml", "nextjs.yml", "security.yml"):
        doc = workflow(name)
        for job_name, job in doc["jobs"].items():
            assert "continue-on-error" not in job
            assert "if" not in job or job_name == "dependency-review"
            if "steps" in job:
                assert job["timeout-minutes"] > 0
            for step in job.get("steps", []):
                assert "continue-on-error" not in step and "if" not in step
                assert "|| true" not in step.get("run", "")
                assert not re.search(r"\bnpm\b", step.get("run", ""))
        for event in ("push", "pull_request"):
            if event in doc["on"]:
                watched = doc["on"][event]["paths"]
                for path in ("apps/wallet/**", "sdk/wallet/**",
                             "services/agent/sdk/typescript/**", "services/wallet/**",
                             ".github/workflows/wallet-*.yml"):
                    assert path in watched
                if name != "security.yml":
                    assert "docker/wallet-pwa/**" in watched


def test_record_maps_findings_to_tests_and_explicit_root_wiring_requests():
    record = tomllib.loads(RECORD_PATH.read_text())
    mapping = {
        "ci-discovery": ("ci.yml", "test_ci_discovery_wallet_pnpm_checks_gate_production_image"),
        "deploy-discovery": ("deploy.yml", "test_deploy_discovery_consumes_checked_image_and_publishes_immutable_digest"),
        "pages-discovery": ("nextjs.yml", "test_pages_discovery_builds_actual_standalone_image_and_transports_it"),
        "security-discovery": ("security.yml", "test_security_discovery_audit_codeql_and_dependency_review_are_blocking"),
    }
    assert set(record["finding"]) == set(mapping)
    targets = set()
    for finding, (filename, test_name) in mapping.items():
        entry = record["finding"][finding]
        assert entry["audit"] == "22.30.116"
        assert entry["file"] == "apps/wallet/.github/workflows/" + filename
        assert entry["test"] == test_name and callable(globals()[test_name])
        assert entry["owner_domain"] == 12
        assert entry["root_target"] == ".github/workflows/wallet-" + filename
        assert entry["status"] == "source-repaired-root-wiring-requested"
        assert entry["root_target"] in entry["wiring_request"]
        assert "domain 12" in entry["wiring_request"]
        assert entry["root_target"] in (WORKFLOWS / filename).read_text()
        targets.add("./" + entry["root_target"])
    for filename, _ in mapping.values():
        for job in workflow(filename)["jobs"].values():
            if "uses" in job:
                assert job["uses"] in targets
    task = record["task"]
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_150.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "revision" in task["evidence"] and "hf-done" in task["evidence"]
    assert "No pre-change verification" in task["baseline"]
    assert "not" in task["review_only"]
    assert "domain 12" in task["rollout_wiring_request"]
    assert "wallet-image-digest" in task["rollout_wiring_request"]
    assert "docker/wallet-pwa/Dockerfile" in task["image_wiring_request"]
