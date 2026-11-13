from pathlib import Path
import re

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
PUBLISH = ".github/workflows/paxeer-docker-publish.yml"
EXPLORER = ".github/workflows/explorer-images.yml"


def workflow(path):
    return yaml.load((ROOT / path).read_text(), Loader=yaml.BaseLoader)


def step_with_id(steps, step_id):
    return next(step for step in steps if step.get("id") == step_id)


@pytest.mark.parametrize("path", [PUBLISH, EXPLORER])
def test_workflows_parse_as_yaml(path):
    document = workflow(path)
    assert document["name"]
    assert document["on"]
    assert document["jobs"]
    for job in document["jobs"].values():
        assert job["steps"]
        assert all("uses" in step or "run" in step for step in job["steps"])


def test_image_publish_waits_for_release_and_uses_its_tagged_commit():
    document = workflow(PUBLISH)
    trigger = document["on"]["workflow_run"]
    assert trigger["workflows"] == ["Paxeer / Release Publish"]
    assert trigger["types"] == ["completed"]
    job = document["jobs"]["pax-images"]
    assert "github.event.workflow_run.conclusion == 'success'" in job["if"]
    assert "github.event.workflow_run.event == 'push'" in job["if"]
    steps = job["steps"]
    checkout = next(step for step in steps if "actions/checkout@" in step.get("uses", ""))
    assert checkout["with"]["ref"] == "${{ github.event.workflow_run.head_sha }}"
    assert checkout["with"]["fetch-depth"] == "0"
    tag = step_with_id(steps, "tag")["run"]
    assert "jq -er '.version' tools/release/version.json" in tag
    assert 'RELEASE_TAG="paxeer-network/$release_version"' in tag
    assert 'test "$(git rev-parse "refs/tags/$RELEASE_TAG^{commit}")" = "$(git rev-parse HEAD)"' in tag
    assert 'VERSION=${RELEASE_TAG#paxeer-network/v}' in tag
    assert 'echo "RELEASE_TAG=$RELEASE_TAG" >> $GITHUB_ENV' in tag


def test_released_binary_and_libraries_are_verified_before_build():
    steps = workflow(PUBLISH)["jobs"]["pax-images"]["steps"]
    download = next(step for step in steps if 'gh release download "$RELEASE_TAG"' in step.get("run", ""))
    script = download["run"]
    assert "set -euo pipefail" in script
    assert download["env"]["GH_REPO"] == "${{ github.repository }}"
    assert download["env"]["GH_TOKEN"] == "${{ github.token }}"
    assert 'asset="paxd-v${VERSION}-linux-amd64"' in script
    assert '--pattern "$asset" --pattern SHA256SUMS' in script
    for library in (
        "libwasmvm.x86_64.so",
        "libwasmvm152.x86_64.so",
        "libwasmvm155.x86_64.so",
    ):
        assert f"--pattern {library}" in script
        assert library in script.split("libraries=(", 1)[1].split(")", 1)[0]
    assert 'for file in "$asset" "${libraries[@]}"; do' in script
    assert "'$2 == file { print }' SHA256SUMS" in script
    assert 'test "$(wc -l < "$file.sha256")" -eq 1' in script
    assert 'sha256sum --check --strict "$file.sha256"' in script
    assert script.index("sha256sum --check --strict") < script.index("install -m 0755")
    assert 'destination="$GITHUB_WORKSPACE/release/linux/amd64"' in script
    assert 'install -m 0755 "$asset" "$destination/paxd"' in script
    assert 'install -m 0644 "${libraries[@]}" "$destination/"' in script
    build = step_with_id(steps, "build_push_image")
    assert steps.index(step_with_id(steps, "tag")) < steps.index(download) < steps.index(build)
    assert build["uses"].startswith("docker/build-push-action@")
    assert build["with"]["context"] == "."
    assert build["with"]["file"] == "docker/layerx/Dockerfile"
    assert build["with"]["target"] == "release"
    assert build["with"]["platforms"] == "linux/amd64"
    assert build["with"]["push"] == "true"


def test_docker_release_target_copies_verified_binary_without_rebuilding_it():
    dockerfile = (ROOT / "docker/layerx/Dockerfile").read_text()
    release = dockerfile.split("FROM runtime AS release\n", 1)[1].split("\nFROM ", 1)[0]
    assert "ARG TARGETARCH" in release
    assert "COPY --chmod=0755 release/linux/${TARGETARCH}/paxd /usr/bin/paxd" in release
    assert "COPY release/linux/${TARGETARCH}/*.so /usr/lib/" in release
    assert "--from=builder" not in release
    assert "RUN " not in release
    runtime = dockerfile.split(" AS runtime\n", 1)[1].split("\nFROM ", 1)[0]
    assert 'ENTRYPOINT ["/usr/bin/paxd"]' in runtime
    assert "ca-certificates libssl3t64" in runtime
    source = dockerfile.split("FROM runtime AS source\n", 1)[1]
    assert "COPY --from=builder /go/bin/paxd /usr/bin/" in source


def test_published_image_digest_is_appended_to_existing_release_notes():
    job = workflow(PUBLISH)["jobs"]["pax-images"]
    assert job["permissions"]["contents"] == "write"
    steps = job["steps"]
    record = next(step for step in steps if 'gh release edit "$RELEASE_TAG"' in step.get("run", ""))
    assert steps.index(step_with_id(steps, "build_push_image")) < steps.index(record)
    assert record["env"]["IMAGE_DIGEST"] == "${{ steps.build_push_image.outputs.digest }}"
    assert record["env"]["GH_REPO"] == "${{ github.repository }}"
    script = record["run"]
    assert "set -euo pipefail" in script
    assert '[[ "$IMAGE_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]]' in script
    assert 'gh release view "$RELEASE_TAG" --json body --jq \'.body\' > "$notes"' in script
    assert '`%s:v%s@%s`' in script
    assert '"$IMAGE_NAME" "$VERSION" "$IMAGE_DIGEST" >> "$notes"' in script
    assert 'gh release edit "$RELEASE_TAG" --notes-file "$notes"' in script


def test_explorer_frontend_receives_resolved_release_tag():
    document = workflow(EXPLORER)
    assert "tools/release/version.json" in document["on"]["push"]["paths"]
    job = document["jobs"]["publish"]
    frontend = next(image for image in job["strategy"]["matrix"]["include"] if image["image"] == "paxeer-x-explorer-frontend")
    assert frontend["dockerfile"] == "docker/explorer-frontend/Dockerfile"
    assert "GIT_COMMIT_SHA=${{ github.sha }}" in frontend["build_args"]
    steps = job["steps"]
    version = step_with_id(steps, "version")
    script = version["run"]
    assert "set -euo pipefail" in script
    assert 'if [ "$GITHUB_REF_TYPE" = tag ]; then' in script
    assert 'tag="$GITHUB_REF_NAME"' in script
    assert "jq -er '.version | strings | select(length > 0)' tools/release/version.json" in script
    assert 'echo "tag=$tag" >> "$GITHUB_OUTPUT"' in script
    build = next(step for step in steps if "docker/build-push-action@" in step.get("uses", ""))
    assert steps.index(version) < steps.index(build)
    assert "${{ matrix.build_args }}" in build["with"]["build-args"]
    assert "GIT_TAG=${{ steps.version.outputs.tag }}" in build["with"]["build-args"]
    assert re.search(r"^ARG GIT_TAG$", (ROOT / frontend["dockerfile"]).read_text(), re.MULTILINE)
