from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
import os
from pathlib import Path
import re
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[4]
HEX = re.compile(r"^[0-9a-f]{64}$")


def canonical(value):
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False, allow_nan=False).encode()


def digest(value):
    return hashlib.sha256(value).hexdigest()


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate input key")
        result[key] = value
    return result


def bound_document(path, expected, *, maximum_bytes=4 * 1024 * 1024):
    path = Path(path)
    if not isinstance(expected, str) or not HEX.fullmatch(expected) or path.is_symlink() or not path.is_file() or path.stat().st_size > maximum_bytes:
        raise ValueError("bounded source-bound document required")
    raw = path.read_bytes()
    if digest(raw) != expected:
        raise ValueError("document digest differs")
    return json.loads(raw, object_pairs_hook=unique)


def origin(value, schemes=("https",)):
    parsed = urlsplit(value)
    if parsed.scheme not in schemes or not parsed.hostname or parsed.username or parsed.password or parsed.path not in ("", "/") or parsed.query or parsed.fragment or value.rstrip("/") != value:
        raise ValueError("exact permitted origin required")
    return parsed


class ContractAuthority:
    """Reviewed documents bound to the same candidate and interface as the run."""

    def __init__(self, config, bindings):
        from dataclasses import asdict
        from jsonschema import Draft202012Validator

        if not config.get("contract_authority_path") or not config.get("contract_authority_sha256"):
            raise ValueError("required source-bound contract authority missing")
        authority = bound_document(config["contract_authority_path"], config["contract_authority_sha256"])
        if authority["bindings"] != asdict(bindings):
            raise ValueError("contract authority candidate/interface bindings differ")
        documents = {}
        for name in ("schemas", "openapi"):
            entry = authority[name]
            path = Path(entry["path"])
            if not path.is_absolute():
                path = ROOT / path
            documents[name] = bound_document(path, entry["sha256"])
        self.definitions = documents["schemas"]
        self.openapi = documents["openapi"]
        if not self.definitions.get("$defs") or self.definitions.get("$schema") != "https://json-schema.org/draft/2020-12/schema":
            raise ValueError("reviewed v2 schema definitions required")
        Draft202012Validator.check_schema(self.definitions)
        if self.openapi.get("openapi") != "3.1.0" or not self.openapi.get("paths"):
            raise ValueError("reviewed OpenAPI3.1 authority required")
        self.interface = bindings.interface
        self.bindings = bindings

        def local_references(value):
            if isinstance(value, dict):
                if "$ref" in value and not value["$ref"].startswith("#/"):
                    raise ValueError("contract references must remain inside the reviewed bundle")
                for child in value.values():
                    local_references(child)
            elif isinstance(value, list):
                for child in value:
                    local_references(child)

        local_references(self.definitions)
        local_references(self.openapi)


def validate_v2(name, value, authority):
    from jsonschema import Draft202012Validator, FormatChecker
    if not isinstance(authority, ContractAuthority):
        raise ValueError("source-bound contract authority required")
    definitions = authority.definitions
    if name not in definitions["$defs"]:
        raise ValueError("unknown v2 contract")
    schema = {**definitions, "$ref": "#/$defs/" + name}
    Draft202012Validator(schema, format_checker=FormatChecker()).validate(value)
    return value


def source_digest():
    directory = Path(__file__).parent
    files = sorted(directory.glob("*.py")) + sorted(directory.glob("phase2_browser.*")) + sorted(directory.glob("phase-*-scenario-manifest.json")) + sorted(directory.glob("phase-*-evidence-policy.json")) + sorted(directory.glob("final-*.json")) + sorted(directory.glob("eventdriver/*.go"))
    return digest(canonical({p.name: digest(p.read_bytes()) for p in files}))


@dataclass(frozen=True)
class Bindings:
    candidate: str
    interface: str
    topology: str
    generator: str

    def __post_init__(self):
        if any(not isinstance(v, str) or not HEX.fullmatch(v) for v in self.__dict__.values()):
            raise ValueError("exact candidate/interface/topology/generator hashes required")


class Artifacts:
    def __init__(self, directory, *, phase=2):
        if type(phase) is not int or phase not in (2, 3, 4):
            raise ValueError("exact lane evidence phase required")
        self.directory = Path(directory).resolve()
        allowed = (ROOT / ("launchpad/evidence/production-system/L48/phase" + str(phase))).resolve()
        if not self.directory.is_relative_to(allowed):
            raise ValueError("lane-owned evidence directory required")
        self.directory.mkdir(parents=True, exist_ok=False)
        self.files = {}

    def rows(self, name):
        if not re.fullmatch(r"[a-z0-9-]+\.ndjson", name) or name in self.files:
            raise ValueError("unique bounded artifact name required")
        output = (self.directory / name).open("x", encoding="utf-8")
        self.files[name] = output
        return output

    @staticmethod
    def append(output, value):
        output.write(canonical(value).decode() + "\n")

    def json(self, name, value):
        if not re.fullmatch(r"[a-z0-9-]+\.json", name):
            raise ValueError("bounded artifact name required")
        with (self.directory / name).open("x") as output:
            output.write(json.dumps(value, indent=2, allow_nan=False) + "\n")

    def close(self):
        for output in self.files.values():
            output.flush()
            os.fsync(output.fileno())
            output.close()

    def checksums(self):
        result = {}
        for path in sorted(self.directory.iterdir()):
            if path.is_file() and path.name != "artifact-sha256.json":
                hasher = hashlib.sha256()
                with path.open("rb") as source:
                    for block in iter(lambda: source.read(1048576), b""):
                        hasher.update(block)
                result[path.name] = hasher.hexdigest()
        return result
