#!/usr/bin/env python3
"""Check resolved dependency versions against a JSON file of minimum versions."""

import argparse
import json
from pathlib import Path
import re
import shlex
import sys
import tomllib
import xml.etree.ElementTree as ET


def _version_parts(version):
    text = version.removeprefix("v").split("+", 1)[0]
    match = re.fullmatch(r"(\d+(?:\.\d+)*)(?:-?([A-Za-z0-9][A-Za-z0-9.-]*))?", text)
    if not match:
        raise ValueError(f"invalid version: {version!r}")
    numbers = tuple(int(part) for part in match[1].split("."))
    suffix = match[2]
    prerelease = (
        None if suffix is None else tuple(
            (0, int(part)) if part.isdigit() else (1, part)
            for part in suffix.split(".")
        )
    )
    return numbers, prerelease


def compare_versions(left, right):
    """Return -1, 0 or 1, ignoring v/build metadata and trailing numeric zeroes."""
    left_numbers, left_pre = _version_parts(left)
    right_numbers, right_pre = _version_parts(right)
    width = max(len(left_numbers), len(right_numbers))
    left_numbers += (0,) * (width - len(left_numbers))
    right_numbers += (0,) * (width - len(right_numbers))
    if left_numbers != right_numbers:
        return (left_numbers > right_numbers) - (left_numbers < right_numbers)
    if left_pre is None or right_pre is None:
        return (left_pre is None) - (right_pre is None)
    return (left_pre > right_pre) - (left_pre < right_pre)


def _add(versions, name, version, wanted):
    if name in wanted:
        if not isinstance(version, str) or not version:
            raise ValueError(f"no resolved version for {name}")
        versions.setdefault(name, []).append(version)


def _package_lock(path, wanted):
    data = json.loads(path.read_text())
    if not isinstance(data.get("packages"), dict):
        raise ValueError(f"{path}: package-lock needs a packages map")
    versions = {}
    for location, package in data["packages"].items():
        if location.startswith("node_modules/") or "/node_modules/" in location:
            name = location.rsplit("node_modules/", 1)[1]
            if not package.get("link"):
                _add(versions, name, package.get("version"), wanted)
    return versions


def _yarn_lock(path, wanted):
    versions = {}
    names = set()
    for line in path.read_text().splitlines():
        line = line.rstrip()
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if not line[0].isspace():
            names = set()
            if not line.endswith(":"):
                raise ValueError(f"{path}: invalid Yarn v1 header: {line}")
            for selector in re.finditer(r'"([^"\n]+)"|([^,\s]+)', line[:-1]):
                value = selector[1] or selector[2]
                match = re.match(r"^(@[^/]+/[^@]+|[^@]+)@", value)
                if match:
                    names.add(match[1])
        elif line.startswith("  version "):
            fields = shlex.split(line, comments=True)
            if fields and fields[0] == "version":
                if len(fields) != 2:
                    raise ValueError(f"{path}: invalid Yarn version: {line}")
                for name in sorted(names):
                    _add(versions, name, fields[1], wanted)
    return versions


def _pnpm_lock(path, wanted):
    """Read package/snapshot keys without needing a YAML dependency."""
    versions = {}
    section = None
    key_indent = None
    seen = set()
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        indent = len(line) - len(line.lstrip())
        if indent == 0:
            section = stripped if stripped in ("packages:", "snapshots:") else None
            key_indent = None
            continue
        if section is None:
            continue
        if key_indent is None:
            key_indent = indent
        if indent != key_indent:
            continue
        key_match = re.match(
            r'''(?:'([^']+)'|"([^"]+)"|([^\s]+?)):\s*(?:\{.*\})?\s*(?:#.*)?$''',
            stripped,
        )
        if not key_match:
            raise ValueError(f"{path}: invalid pnpm package key: {stripped}")
        key = next(part for part in key_match.groups() if part is not None)
        match = re.fullmatch(r"/?(@[^/]+/[^@/]+|[^@/]+)(?:@|/)(.+)", key)
        if not match:
            raise ValueError(f"{path}: unsupported pnpm package key: {key}")
        name, version = match.groups()
        version = re.split(r"[(_]", version, maxsplit=1)[0]
        if (name, version) not in seen:
            _add(versions, name, version, wanted)
            seen.add((name, version))
    return versions


def _go_mod(path, wanted):
    requires = []
    replacements = {}
    block = None
    for line in path.read_text().splitlines():
        fields = shlex.split(line.split("//", 1)[0])
        if not fields:
            continue
        if fields == [")"]:
            block = None
            continue
        if fields[0] in ("require", "replace"):
            directive, fields = fields[0], fields[1:]
            if fields == ["("]:
                block = directive
                continue
        elif block:
            directive = block
        else:
            continue
        if directive == "require":
            if len(fields) != 2:
                raise ValueError(f"{path}: invalid require: {line}")
            requires.append(tuple(fields))
        else:
            if "=>" not in fields:
                raise ValueError(f"{path}: invalid replace: {line}")
            arrow = fields.index("=>")
            old, new = fields[:arrow], fields[arrow + 1:]
            if len(old) not in (1, 2) or len(new) not in (1, 2):
                raise ValueError(f"{path}: invalid replace: {line}")
            replacements[(old[0], old[1] if len(old) == 2 else None)] = (
                new[1] if len(new) == 2 else None
            )
    versions = {}
    for name, version in requires:
        version = replacements.get(
            (name, version), replacements.get((name, None), version)
        )
        _add(versions, name, version, wanted)
    return versions


def _cargo_lock(path, wanted):
    with path.open("rb") as stream:
        data = tomllib.load(stream)
    versions = {}
    for package in data.get("package", []):
        _add(versions, package["name"], package.get("version"), wanted)
    return versions


def _pom(path, wanted):
    root = ET.parse(path).getroot()
    properties = {
        child.tag.rsplit("}", 1)[-1]: (child.text or "").strip()
        for block in root.findall("{*}properties")
        for child in block
    }

    def resolve(value, active=()):
        def substitute(match):
            key = match[1]
            if key in active or key not in properties:
                raise ValueError(f"{path}: unresolved or cyclic property ${{{key}}}")
            return resolve(properties[key], active + (key,))

        return re.sub(r"\$\{([^}]+)\}", substitute, value)

    versions = {}
    for dependency in root.findall(".//{*}dependency"):
        artifact = dependency.findtext("{*}artifactId", "").strip()
        group = dependency.findtext("{*}groupId", "").strip()
        names = {artifact, f"{group}:{artifact}"} & wanted
        if names:
            version = resolve(dependency.findtext("{*}version", "").strip())
            for name in sorted(names):
                _add(versions, name, version, wanted)
    return versions


def _python_name(name):
    return re.sub(r"[-_.]+", "-", name).lower()


def _requirements(path, wanted):
    versions = {}
    for line in path.read_text().splitlines():
        match = re.match(
            r"^\s*([A-Za-z0-9_.-]+)(?:\[[^]]+\])?\s*==\s*([^\s;#]+)",
            line,
        )
        if match:
            for name in wanted:
                if _python_name(name) == _python_name(match[1]):
                    _add(versions, name, match[2], wanted)
    return versions


READERS = {
    "package-lock.json": _package_lock,
    "yarn.lock": _yarn_lock,
    "pnpm-lock.yaml": _pnpm_lock,
    "go.mod": _go_mod,
    "Cargo.lock": _cargo_lock,
    "pom.xml": _pom,
    "requirements.txt": _requirements,
}


def _lock_path(manifest):
    if not manifest.is_file():
        raise ValueError(f"manifest does not exist: {manifest}")
    if manifest.name in READERS:
        return manifest
    if manifest.name == "package.json":
        names = ("package-lock.json", "yarn.lock", "pnpm-lock.yaml")
        manager = (
            json.loads(manifest.read_text())
            .get("packageManager", "")
            .split("@", 1)[0]
        )
        preferred = {
            "npm": "package-lock.json",
            "yarn": "yarn.lock",
            "pnpm": "pnpm-lock.yaml",
        }.get(manager)
        if preferred:
            names = (preferred,)
    elif manifest.name == "Cargo.toml":
        names = ("Cargo.lock",)
    else:
        raise ValueError(f"unsupported manifest: {manifest}")
    for directory in (manifest.parent, *manifest.parent.parents):
        candidates = [directory / name for name in names if (directory / name).is_file()]
        if len(candidates) > 1:
            raise ValueError(f"{manifest}: ambiguous lock files in {directory}")
        if candidates:
            return candidates[0]
    raise ValueError(f"no lock file for {manifest}")


def lockfloor(floors_file):
    """Print each manifest/package/resolved/floor violation and return its exit code."""
    data = json.loads(Path(floors_file).read_text())
    if not isinstance(data, dict) or not isinstance(data.get("manifests"), dict):
        raise ValueError("floors file must contain a manifests object")
    failed = False
    for manifest_name, floors in data["manifests"].items():
        if not isinstance(floors, dict):
            raise ValueError(f"{manifest_name}: floors must be an object")
        parsed_floors = {}
        for package, floor in floors.items():
            minimum = floor.get("minimum") if isinstance(floor, dict) else floor
            applies_from = floor.get("applies_from") if isinstance(floor, dict) else None
            if (
                not package
                or not isinstance(minimum, str)
                or (isinstance(floor, dict) and not isinstance(applies_from, str))
            ):
                raise ValueError(f"{manifest_name}: package floors must be version strings")
            _version_parts(minimum)
            if applies_from is not None:
                _version_parts(applies_from)
            parsed_floors[package] = (minimum, applies_from)
        try:
            path = _lock_path(Path(manifest_name).resolve())
            versions = READERS[path.name](path, set(floors))
            for package, (floor, applies_from) in parsed_floors.items():
                resolved = versions.get(package, [])
                if not resolved:
                    print(f"{manifest_name}\t{package}\t<unresolved>\t{floor}")
                    failed = True
                for version in resolved:
                    if (
                        applies_from is not None
                        and compare_versions(version, applies_from) < 0
                    ):
                        continue
                    if compare_versions(version, floor) < 0:
                        print(f"{manifest_name}\t{package}\t{version}\t{floor}")
                        failed = True
        except (OSError, ValueError, KeyError, TypeError, ET.ParseError) as error:
            print(f"{manifest_name}: {error}", file=sys.stderr)
            failed = True
    return int(failed)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("floors_file", type=Path)
    args = parser.parse_args()
    try:
        return lockfloor(args.floors_file)
    except (OSError, ValueError, TypeError) as error:
        print(f"lockfloor: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
