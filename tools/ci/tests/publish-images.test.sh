#!/usr/bin/env bash
# Checks that the shared beta image table covers every image Dockerfile in the repository and that
# the publish-images workflow builds the table as a matrix with GHCR write permission.
set -euo pipefail

REPO_ROOT=$(cd "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)
cd "$REPO_ROOT"
errors=0
fail() { printf 'publish-images.test: %s\n' "$*" >&2; errors=$((errors + 1)); }

source services/platform/hosted/tests/beta-images.sh

image_contexts_match_dockerfile_inputs() {
    local name
    [ "$(image_context explorer-backend)" = services/explorer/backend ] || fail "explorer-backend context is not services/explorer/backend"
    [ "$(image_context platform-registry-builder)" = @registry-builder ] || fail "platform-registry-builder context is not prepared"
    for name in "${IMAGE_NAMES[@]}"; do
        case "$name" in
            explorer-backend|platform-registry-builder) continue ;;
        esac
        [ "$(image_context "$name")" = . ] || fail "$name: context is not the repository root"
    done
    python3 - <<'PY' || fail "image_contexts_match_dockerfile_inputs failed"
from pathlib import Path
import re
import shlex
import yaml

context = Path("services/explorer/backend")
copies = 0
for line in Path("docker/explorer-backend/Dockerfile").read_text().splitlines()[:24]:
    if not re.match(r"^\s*COPY\s+", line, re.IGNORECASE):
        continue
    words = shlex.split(line, comments=True)
    assert len(words) >= 3, f"COPY has no source or destination: {line}"
    for source in words[1:-1]:
        assert (context / source).exists(), f"missing explorer COPY input: {source}"
        copies += 1
assert copies > 0, "no explorer COPY inputs checked"

workflow = Path(".github/workflows/publish-images.yml").read_text()
jobs = yaml.safe_load(workflow)["jobs"]
plan = workflow.split("  plan:\n", 1)[1].split("\n  images:\n", 1)[0]
assert '--arg context "$(image_context "$name")"' in plan, "matrix does not read image_context"
assert "context: $context" in plan, "matrix rows do not carry context"
for name in ("images", "images-dependent"):
    steps = jobs[name]["steps"]
    builds = [step for step in steps if step.get("uses", "").startswith("docker/build-push-action@")]
    assert len(builds) == 1, f"{name} must have one image build"
    assert builds[0]["with"]["context"] == "${{ steps.registry-builder.outputs.context || matrix.image.context }}", f"{name} build does not use the selected context"
    preparations = [step for step in steps if step.get("id") == "registry-builder"]
    assert len(preparations) == 1, f"{name} must have one context preparation step"
    assert preparations[0]["if"] == "matrix.image.context == '@registry-builder'", f"{name} context preparation has the wrong condition"
    assert steps.index(preparations[0]) < steps.index(builds[0]), f"{name} prepares the context after the build"
    job = workflow.split(f"\n  {name}:\n", 1)[1]
    job = re.split(r"\n  [\w-]+:\n", job, maxsplit=1)[0]
    assert not re.search(r"^\s*context:\s*\.\s*(?:#.*)?$", job, re.MULTILINE), f"{name} uses a fixed root context"
    assert "if: matrix.image.context == '@registry-builder'" in job, f"{name} does not select registry preparation"
    assert 'bash services/platform/hosted/registry/builder-environment/build-env.sh --prepare-context "$RUNNER_TEMP/registry-builder"' in job, f"{name} does not prepare registry inputs"
    assert "printf 'context=%s/context\\n' \"$RUNNER_TEMP/registry-builder\" >> \"$GITHUB_OUTPUT\"" in job, f"{name} does not export the prepared context"
    assert "context: ${{ steps.registry-builder.outputs.context || matrix.image.context }}" in job, f"{name} does not use the selected context"
PY
}

image_contexts_match_dockerfile_inputs

declare -A table_paths=()
for name in "${IMAGE_NAMES[@]}"; do
    read -r canonical dockerfile <<<"$(image_source "$name")"
    [ "$canonical" = "ghcr.io/sidiora-labs/$name:${canonical##*:}" ] || fail "$name: canonical image $canonical is not ghcr.io/sidiora-labs/$name"
    case "$dockerfile" in
        /*|*..*|'') fail "$name: Dockerfile path $dockerfile is not under the repository" ;;
    esac
    resolved=$(realpath -m -- "$REPO_ROOT/$dockerfile")
    [[ $resolved == "$REPO_ROOT"/* ]] || fail "$name: Dockerfile path $dockerfile escapes the repository"
    table_paths[$dockerfile]=1
done

for dockerfile in docker/*/Dockerfile services/platform/hosted/feeder/Dockerfile; do
    [ -f "$dockerfile" ] || continue
    [ -n "${table_paths[$dockerfile]+present}" ] || fail "no beta-images.sh row for $dockerfile"
done
while IFS= read -r -d '' dockerfile; do
    dockerfile=${dockerfile#./}
    [ -n "${table_paths[$dockerfile]+present}" ] || fail "no beta-images.sh row for $dockerfile"
done < <(find services/wallet services/interop -name Dockerfile -not -path '*/node_modules/*' -print0 2>/dev/null)
for dockerfile in docker/intent-ingester/Dockerfile docker/redis/Dockerfile; do
    [ -n "${table_paths[$dockerfile]+present}" ] || fail "no beta-images.sh row for $dockerfile"
done

workflow=.github/workflows/publish-images.yml
chain_source_paths() {
    if python3 -c 'import yaml'; then
        python3 - "$workflow" <<'PY' || fail "workflow structure check failed"
import sys
import yaml

doc = yaml.safe_load(open(sys.argv[1]))
on = doc.get("on", doc.get(True))
push = on["push"]
assert "main" in push["branches"], "push to main is not a trigger"
assert push["tags"], "tag trigger dropped"
for path in (
    "docker/**", "services/platform/**", "services/interop/**", "services/wallet/**",
    "sdk/wallet/**", "apps/explorer/**", "services/explorer/**",
    "go.mod", "go.sum", "Makefile", "chain.mk", "cmd/**", "daemon/**", "node/**",
    "modules/**", "precompiles/**", "rpc/**", "engine/**", "sdk/**", "src/**",
    "include/**", "programs/**", "contracts/config/**", "tools/release/version.json",
):
    assert path in push["paths"], f"missing push path {path}"
print("publish-images.test: chain_source_paths ok", flush=True)
assert "workflow_dispatch" in on, "dispatch trigger dropped"
jobs = doc["jobs"]
for name in ("images", "images-dependent"):
    job = jobs[name]
    assert job["permissions"]["packages"] == "write", f"{name} lacks packages: write"
    assert "matrix.image" not in str(job["runs-on"])
    assert "fromJSON(needs.plan.outputs" in str(job["strategy"]["matrix"]["image"]), f"{name} matrix is not the table plan"
    steps = [s for s in job["steps"] if str(s.get("uses", "")).startswith("docker/build-push-action@")]
    assert len(steps) == 1, f"{name} must build with docker/build-push-action"
    w = steps[0]["with"]
    assert w["push"] is True and "ghcr.io/sidiora-labs/" in w["tags"]
    assert w["cache-from"].startswith("type=gha") and w["cache-to"].startswith("type=gha")
    assert w["target"] == "${{ matrix.image.target }}"
assert "beta-images.sh" in jobs["plan"]["steps"][-1]["run"]
text = open(sys.argv[1]).read()
assert "CI_LINUX_RUNNER" not in text and "fly-linux" not in text, "Fly runner clause remains"
for job in jobs.values():
    assert "ubuntu" in str(job["runs-on"]), "every job must run on a GitHub-hosted ubuntu label by default"
PY
    else
        fail "chain_source_paths requires PyYAML; install it for python3 before running this test"
        grep -q 'matrix:' "$workflow" || fail "workflow has no matrix"
        grep -q 'packages: write' "$workflow" || fail "workflow lacks packages: write"
        grep -q 'docker/build-push-action@' "$workflow" || fail "workflow does not use docker/build-push-action"
        grep -q 'beta-images.sh' "$workflow" || fail "workflow does not read the image table"
        ! grep -q 'CI_LINUX_RUNNER' "$workflow" || fail "Fly runner clause remains"
    fi
}
chain_source_paths

bash -n services/platform/hosted/tests/beta-images.sh services/platform/hosted/tests/beta-cluster.sh || fail "shell syntax"

[ "$errors" -eq 0 ] || { printf 'publish-images.test: %d failure(s)\n' "$errors" >&2; exit 1; }
printf 'publish-images.test: ok (%d images)\n' "${#IMAGE_NAMES[@]}"
