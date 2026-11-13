"""Read committed schema pins and route recordings, without inventing owner bundles."""

import hashlib
import importlib.util
import json
import pathlib
import re
import shutil


ROOT = pathlib.Path(__file__).resolve().parents[5]
OWNER = pathlib.Path("sdk/launchpad/protocol/contracts")
RECORDED = pathlib.Path("services/launchpad/api/internal/rest/testdata/contracts/v2/openapi.json")
GENERATOR = pathlib.Path("tools/launchpad/production/interfaces/codegen.py")


def load_module(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def package_path(major):
    if major not in (1, 2):
        raise ValueError(f"Unsupported schema major {major}")
    return pathlib.Path("sdk/launchpad/packages") / ("sdk" if major == 1 else "sdk-v2")


def schema_input(major, root=ROOT):
    package = root / package_path(major)
    release = json.loads((package / "release-manifest.json").read_bytes())
    assert release["apiMajor"] == release["streamMajor"] == major
    for name, digest in release["outputs"].items():
        assert hashlib.sha256((package / "src" / name).read_bytes()).hexdigest() == digest, name
    types = (package / "src/types.gen.ts").read_text()
    document = json.loads((package / "src/schemas.gen.json").read_bytes())
    # The emitter appends lifted union definitions to its output schema. Its
    # ContractTypes inventory preserves exactly the original input definitions.
    names = re.findall(r'^"([^"]+)":', types.split("export interface ContractTypes {", 1)[1], re.M)
    assert names and len(names) == len(set(names))
    document["$defs"] = {name: document["$defs"][name] for name in names}
    data = (json.dumps(document, sort_keys=True, indent=2) + "\n").encode()
    assert hashlib.sha256(data).hexdigest() == release["inputs"]["schema-definitions.json"]
    return data


def copy_bundle(major, destination):
    package = package_path(major)
    for relative in (
        GENERATOR, GENERATOR.with_name("validator.go.txt"),
        package / "src/types.gen.ts", package / "src/schemas.gen.json",
        package / "release-manifest.json",
    ):
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / relative, target)
    target = destination / OWNER / f"v{major}"
    target.mkdir(parents=True)
    (target / "schema-definitions.json").write_bytes(schema_input(major))
    assert (target / "schema-definitions.json").read_bytes() == schema_input(major, destination)
    return target


def operation_catalog(path=None):
    document = json.loads((path or ROOT / RECORDED).read_bytes())
    operations = {}
    for path, methods in document["paths"].items():
        for method, operation in methods.items():
            if method in ("get", "post", "patch", "put", "delete"):
                name = operation["operationId"]
                assert name not in operations, name
                operations[name] = (method.upper(), path)
    for name, expected in {
        "IF_TRADES": ("GET", "/v2/pools/{poolAddress}/trades"),
        "IF_TOPIC_SNAPSHOT_READ": ("GET", "/v2/snapshots"),
    }.items():
        assert operations[name] == expected, name
    return operations
