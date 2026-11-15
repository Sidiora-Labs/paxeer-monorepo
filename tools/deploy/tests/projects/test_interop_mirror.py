"""Checks the interop mirror project row against its container sources."""

from __future__ import annotations

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
PROJECT = ROOT / "deploy/projects/interop/mirror.kvx"
CONTAINER = ROOT / "docker/interop-mirror"


def parse_kvx(path: Path) -> dict[str, dict[str, object]]:
    sections: dict[str, dict[str, object]] = {}
    section: dict[str, object] | None = None
    multiline_key: str | None = None
    multiline_value: list[str] = []
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("[") and line.endswith("]"):
            section = sections.setdefault(line[1:-1], {})
            continue
        if section is None:
            continue
        if multiline_key is not None:
            multiline_value.append(line)
            if line.endswith("]"):
                section[multiline_key] = " ".join(multiline_value)
                multiline_key = None
                multiline_value = []
            continue
        if "=" not in line:
            continue
        key, value = (part.strip() for part in line.split("=", 1))
        if value.endswith("["):
            multiline_key = key
            multiline_value = [value]
        else:
            section[key] = value
    return sections


def quoted_list(value: object) -> list[str]:
    return re.findall(r'"([^"]+)"', str(value))


def source_variables() -> set[str]:
    """Return explicit deployment variables consumed by the image inputs."""

    table = CONTAINER / "files.tsv"
    names = set()
    for line in table.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        name = line.split("\t", 1)[0]
        if re.fullmatch(r"[A-Z][A-Z0-9_]*", name):
            names.add(name)
    for path in CONTAINER.glob("*.sh"):
        text = path.read_text()
        names.update(re.findall(r"\$\{([A-Z][A-Z0-9_]*)", text))
        names.update(re.findall(r"\$([A-Z][A-Z0-9_]*)", text))
    return names


def watch_patterns(sections: dict[str, dict[str, object]]) -> list[str]:
    return quoted_list(sections["service.mirror"]["watch"])


def path_is_watched(path: str, patterns: list[str]) -> bool:
    prefix = path.rstrip("/") + "/"
    return any(pattern.endswith("/**") and prefix.startswith(pattern[:-2]) for pattern in patterns)


def test_mirror_manifest_describes_the_publisher_container() -> None:
    sections = parse_kvx(PROJECT)
    service = sections["service.mirror"]
    assert service["kind"] == '"repo"'
    assert service["container"] == '"docker/interop-mirror/Dockerfile"'
    assert service["root"] == '"."'
    assert service["health"] == '"/readyz"'
    assert service["restart"] == '"ALWAYS"'
    assert service["replicas"] == "1"
    assert service["volume"] == '"/var/lib/layerx-mirror"'
    assert service["order"] == "11"

    patterns = watch_patterns(sections)
    assert path_is_watched("docker/interop-mirror/Dockerfile", patterns)
    assert path_is_watched("docker/common/env-files.sh", patterns)
    assert path_is_watched("services/interop/Cargo.toml", patterns)
    assert (ROOT / service["container"].strip('"')).is_file()
    assert (ROOT / "docker/common/env-files.sh").is_file()

    dockerfile = (CONTAINER / "Dockerfile").read_text()
    assert "COPY . ." in dockerfile
    assert "--manifest-path services/interop/Cargo.toml" in dockerfile
    assert "ENTRYPOINT [\"/usr/local/bin/layerx-env-files\"" in dockerfile
    assert "/etc/layerx-mirror/config.json" in dockerfile
    assert "GET /readyz" in (ROOT / "services/interop/crates/layerx-mirror/src/bin/layerx-mirror-publisher.rs").read_text()


def test_mirror_manifest_tracks_every_container_input() -> None:
    sections = parse_kvx(PROJECT)
    variables = sections["vars.mirror"]
    inputs = sections["inputs.mirror"]
    expected_prefix = "INTEROP_MIRROR_"
    names = source_variables()
    assert names
    assert all(name.startswith(expected_prefix) for name in names)
    assert names == set(variables) == set(inputs)
    assert all(value.startswith('"env:') for value in variables.values())
    assert all(value.strip('"').startswith("operator") for value in inputs.values())

    table_fields = [line.split("\t") for line in (CONTAINER / "files.tsv").read_text().splitlines() if line and not line.startswith("#")]
    assert len(table_fields) == 1
    name, guest, mode, roles = table_fields[0]
    assert name in names
    assert guest == "/etc/layerx-mirror/config.json"
    assert mode == "0600"
    assert roles == "*"

