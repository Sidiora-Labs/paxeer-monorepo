import os
import re
import shlex
import subprocess
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
TABLE = ROOT / "services/platform/hosted/tests/beta-images.sh"
IMAGE_DOCKERFILES = {
    "explorer-frontend": "docker/explorer-frontend/Dockerfile",
    "hpx-registry": "docker/hpx-registry/Dockerfile",
    "localnode": "docker/localnode/Dockerfile",
    "layerx-dashboard-web": "docker/platform-dashboard-web/Dockerfile",
    "rpcnode": "docker/rpcnode/Dockerfile",
    "search-front": "docker/search-front/Dockerfile",
    "wallet-attestor": "docker/wallet-attestor/Dockerfile",
    "wallet-gateway": "docker/wallet-gateway/Dockerfile",
    "wallet-pwa": "docker/wallet-pwa/Dockerfile",
    "layerx-human-web": "docker/web/Dockerfile",
}
PINNED_IMAGE = re.compile(r"([^@\s]+):([^:@\s]+)@sha256:([0-9a-f]{64})")


@pytest.fixture(scope="module")
def table_images():
    result = subprocess.run(
        [
            "bash",
            "-euc",
            '''
source "$1"
shift
for name in "$@"; do
    [[ " ${IMAGE_NAMES[*]} " == *" $name "* ]]
    read -r canonical dockerfile <<<"$(image_source "$name")"
    bases=$(image_base_refs "$name")
    while IFS= read -r base; do
        printf '%s\\t%s\\t%s\\n' "$name" "$dockerfile" "$base"
    done <<<"$bases"
done
''',
            "table-image-bases",
            str(TABLE),
            *IMAGE_DOCKERFILES,
        ],
        cwd=ROOT,
        env={**os.environ, "REPO_ROOT": str(ROOT)},
        text=True,
        capture_output=True,
        check=True,
    )
    images = {}
    for row in result.stdout.splitlines():
        name, dockerfile, base = row.split("\t")
        assert dockerfile == IMAGE_DOCKERFILES[name], row
        assert base, f"{name}: no external base found in the shared table"
        images.setdefault(name, []).append(base)
    assert set(images) == set(IMAGE_DOCKERFILES)
    return images


def _instructions(path):
    contents = re.sub(r"\\\r?\n", " ", path.read_text(encoding="utf-8"))
    for line in contents.splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            instruction, arguments = line.split(None, 1)
            yield instruction.upper(), arguments


def _external_bases(path):
    stages = set()
    bases = []
    for instruction, arguments in _instructions(path):
        if instruction != "FROM":
            continue
        arguments = shlex.split(arguments, comments=True)
        base = next(arg for arg in arguments if not arg.startswith("--"))
        if base.lower() not in stages:
            bases.append(base)
        if len(arguments) >= 3 and arguments[-2].upper() == "AS":
            stages.add(arguments[-1].lower())
    assert bases, f"{path}: no external FROM instructions found"
    return bases


@pytest.mark.parametrize("name", IMAGE_DOCKERFILES)
def test_table_images_pin_base_digests(name, table_images):
    dockerfile = ROOT / IMAGE_DOCKERFILES[name]
    bases = _external_bases(dockerfile)
    assert table_images[name] == bases, f"{dockerfile}: table base inventory disagrees"
    for base in bases:
        match = PINNED_IMAGE.fullmatch(base)
        assert match, f"{dockerfile}: base requires an exact tag and sha256 digest: {base}"
        assert match.group(3) != "0" * 64, f"{dockerfile}: placeholder digest: {base}"


@pytest.mark.parametrize("name", IMAGE_DOCKERFILES)
def test_no_floating_base_tags(name, table_images):
    for base in table_images[name]:
        match = PINNED_IMAGE.fullmatch(base)
        assert match, f"{name}: unpinned image: {base}"
        repository, tag = match.group(1), match.group(2)
        assert not {"latest", "stable", "rolling", "edge"}.intersection(
            re.split(r"[._-]", tag.lower())
        ), f"{name}: floating tag: {base}"
        image = repository.rsplit("/", 1)[-1]
        if image == "ubuntu":
            assert re.fullmatch(r"\d{2}\.\d{2}(?:\.\d+)?", tag), base
        elif image == "debian":
            assert re.fullmatch(r"\d+\.\d+-slim", tag), base
        else:
            assert re.fullmatch(r"\d+\.\d+\.\d+(?:-[a-z0-9.-]+)?", tag), base
        if image in {"node", "golang", "rust"} and "alpine" in tag:
            assert re.search(r"-alpine\d+\.\d+$", tag), base
        if image == "node":
            assert int(tag.split(".", 1)[0]) >= 22, f"{name}: end-of-life Node: {base}"
    if name == "hpx-registry":
        tags = [base.split("@", 1)[0] for base in table_images[name]]
        assert tags == ["golang:1.25.8-alpine3.22", "alpine:3.22.2", "alpine:3.22.2"]
    if name == "localnode":
        instructions = list(_instructions(ROOT / IMAGE_DOCKERFILES[name]))
        copies = [shlex.split(args) for op, args in instructions if op == "COPY"]
        assert ["--from=node-dist", "/usr/local/bin/node", "/usr/local/bin/node"] in copies
        assert [
            "--from=node-dist",
            "/usr/local/lib/node_modules",
            "/usr/local/lib/node_modules",
        ] in copies


@pytest.mark.parametrize("name", IMAGE_DOCKERFILES)
def test_no_curl_pipe_shell_in_dockerfiles(name, table_images):
    assert table_images[name]
    dockerfile = ROOT / IMAGE_DOCKERFILES[name]
    for instruction, arguments in _instructions(dockerfile):
        if instruction != "RUN":
            continue
        for pipeline in re.split(r"&&|\|\||;", arguments):
            commands = pipeline.split("|")
            for index, command in enumerate(commands):
                if re.search(r"\b(?:curl|wget)\b", command):
                    for consumer in commands[index + 1 :]:
                        assert not re.search(
                            r"(?:^|\s|/)(?:sh|bash|dash|ash|zsh|ksh)(?:\s|$)",
                            consumer,
                        ), f"{dockerfile}: downloaded script piped to a shell: {pipeline}"
        assert "deb.nodesource.com/setup_16" not in arguments, (
            f"{dockerfile}: end-of-life Node installer"
        )
