"""Reviewed split-repository qualification inputs. Missing authority fails closed.

The retained protocol fixtures are an inventory, not a replacement semantic
catalogue or scheduler DAG. Only the canonical registry can approve that migration.
"""

import hashlib
import importlib
import importlib.util
import json
from pathlib import Path
import re
import sys
import tomllib
from types import SimpleNamespace

REGISTRY_PATH = "tools/launchpad/production/bootstrap/check-registry.json"
DISPATCHER_PATH = "tools/launchpad/production/dispatcher"
RETAINED_SPEC_PATH = "spec/layerx-merge/spec.kvx"
RUNNER_PATHS = (
    "tools/launchpad/production/qualify.py",
    "tools/launchpad/production/control_plane.py",
    "tools/launchpad/production/serial_execution.py",
)
FIXTURE_ROOT = "sdk/launchpad/protocol/contracts/v1"
FIXTURE_FILES = (
    "accounting-cases.json",
    "capabilities.json",
    "chain/abi-catalog.json",
    "chain/client-deployment.json",
    "chain/event-authorities.json",
    "chain/golden-fixtures.json",
)
RETAINED_FILES = (
    *(f"{FIXTURE_ROOT}/{name}" for name in FIXTURE_FILES),
    "sdk/launchpad/packages/sdk/src/operations.gen.ts",
    "sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json",
)
_UNPINNED = object()


class ControlPlaneError(ValueError):
    pass


def digest(value):
    return hashlib.sha256(
        json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    ).hexdigest()


def source_file(root, relative, expected=_UNPINNED):
    """Read exactly a reviewed local file, rejecting links and path aliases."""
    root = Path(root).resolve()
    if (
        not isinstance(relative, str)
        or not relative
        or "\\" in relative
        or "\x00" in relative
        or any(part in ("", ".", "..") for part in relative.split("/"))
    ):
        raise ControlPlaneError("unsafe reviewed input path")
    path = root / relative
    if (
        Path(relative).is_absolute()
        or not path.resolve().is_relative_to(root)
        or any(item.is_symlink() for item in (path, *path.parents))
        or not path.is_file()
        or path.stat().st_nlink != 1
    ):
        raise ControlPlaneError(f"reviewed input missing or unsafe: {relative}")
    data = path.read_bytes()
    observed = hashlib.sha256(data).hexdigest()
    if expected is not _UNPINNED and (
        not isinstance(expected, str)
        or not re.fullmatch("[a-f0-9]{64}", expected)
        or observed != expected
    ):
        raise ControlPlaneError(f"reviewed input digest differs: {relative}")
    return path, data, observed


def retained_inventory(root, registry):
    inventory = registry.get("retained_inputs")
    if (
        not isinstance(inventory, dict)
        or set(inventory) != {"authority", "files", "sha256"}
        or inventory["authority"] != RETAINED_SPEC_PATH
        or not isinstance(inventory["files"], dict)
        or set(inventory["files"]) != set(RETAINED_FILES)
    ):
        raise ControlPlaneError("exact retained protocol and SDK inventory required")
    # This is the programme authority, not the production scheduler graph. Read
    # its identity header without projecting its unrelated multi-domain tasks.
    text = source_file(root, RETAINED_SPEC_PATH)[1].decode()
    header = re.search(r"(?ms)^\[meta\]\s*\n.*?(?=^\[|\Z)", text)
    if header is None:
        raise ControlPlaneError("retained specification identity absent")
    authority = tomllib.loads(header.group())
    if authority.get("meta", {}).get("slug") != "layerx-merge":
        raise ControlPlaneError("wrong retained specification authority")
    observed = {
        name: source_file(root, name, expected)[2]
        for name, expected in inventory["files"].items()
    }
    if digest(observed) != inventory["sha256"]:
        raise ControlPlaneError("retained inventory digest differs")
    return {"files": observed, "sha256": inventory["sha256"]}


def reviewed_inputs(root, registry, spec_path=None, require_catalog=False):
    retained = retained_inventory(root, registry)
    review = registry.get("control_plane")
    if not isinstance(review, dict) or review.get("schema_version") != 1:
        raise ControlPlaneError("explicit control-plane migration review required")
    if review.get("status") != "approved":
        raise ControlPlaneError(
            "production graph/catalog migration requires owning domain 4 review; "
            "retained fixtures do not supply those authorities"
        )
    bindings = review.get("bindings")
    approval = review.get("approval")
    if (
        not isinstance(bindings, dict)
        or not isinstance(approval, dict)
        or not isinstance(approval.get("reference"), str)
        or not approval["reference"].strip()
        or approval.get("bindings_sha256") != digest(bindings)
        or bindings.get("retained_sha256") != retained["sha256"]
    ):
        raise ControlPlaneError("migration approval does not bind current inputs")
    selected = {"retained": retained, "bindings": bindings}
    runners = bindings.get("runner_sources")
    if not isinstance(runners, dict) or set(runners) != set(RUNNER_PATHS):
        raise ControlPlaneError("exact qualification runner source identities required")
    for path, expected in runners.items():
        source_file(root, path, expected)
    names = ["spec", "plan"]
    if require_catalog:
        names += ["catalog", "catalog_manifest"]
    for name in names:
        identity = bindings.get(name)
        if not isinstance(identity, dict) or set(identity) != {"path", "sha256"}:
            raise ControlPlaneError(f"reviewed {name} identity required")
        selected[name] = source_file(root, identity["path"], identity["sha256"])[0]
    if spec_path is not None and Path(spec_path).resolve() != selected["spec"]:
        raise ControlPlaneError("qualification requires the canonical reviewed specification")
    if require_catalog:
        catalog = json.loads(selected["catalog"].read_bytes())
        sections = catalog.get("sections") if isinstance(catalog, dict) else None
        if (
            not isinstance(catalog, dict)
            or catalog.get("algorithm") != "interface-catalog-v1"
            or not isinstance(sections, dict)
            or set(sections) != {"schema", "endpoint", "stream", "error", "interface_freeze", "req"}
            or any(not isinstance(value, dict) for value in sections.values())
            or not sections["schema"]
            or not sections["endpoint"]
        ):
            raise ControlPlaneError("authentic nonempty semantic catalogue required")
        semantic = digest(catalog)
        manifest = json.loads(selected["catalog_manifest"].read_bytes())
        inputs = manifest.get("inputs", {}) if isinstance(manifest, dict) else {}
        if (
            not isinstance(inputs, dict)
            or inputs.get("semanticDigestAlgorithm") != "interface-catalog-v1"
            or inputs.get("catalogSha256") != semantic
        ):
            raise ControlPlaneError("catalogue manifest semantic identity differs")
        selected["semantic_input"] = {
            "algorithm": "interface-catalog-v1",
            "sha256": semantic,
            "catalog_manifest_sha256": bindings["catalog_manifest"]["sha256"],
            "catalog_source_sha256": bindings["catalog"]["sha256"],
        }
    return selected


def load_dispatcher(root, bindings):
    """Load the reviewed package from this source root, independent of sys.path."""
    package = Path(root).resolve() / DISPATCHER_PATH
    identities = bindings.get("dispatcher_sources")
    files = {path.name for path in package.glob("*.py")}
    if not files or not isinstance(identities, dict) or set(identities) != files:
        raise ControlPlaneError("complete dispatcher source identities required")
    for name, expected in identities.items():
        source_file(root, f"{DISPATCHER_PATH}/{name}", expected)
    namespace = "_reviewed_launchpad_dispatcher_" + hashlib.sha256(
        (str(package) + digest(identities)).encode()
    ).hexdigest()
    if namespace not in sys.modules:
        loader = importlib.util.spec_from_file_location(
            namespace, package / "__init__.py", submodule_search_locations=[str(package)]
        )
        module = importlib.util.module_from_spec(loader)
        sys.modules[namespace] = module
        try:
            loader.loader.exec_module(module)
        except BaseException:
            sys.modules.pop(namespace, None)
            raise
    return SimpleNamespace(**{
        name: importlib.import_module(f"{namespace}.{name}")
        for name in ("graph", "acceptance", "evidence", "patches", "store")
    })


def reviewed_graph(root, registry=None, spec_path=None, spec=None, require_catalog=False):
    if registry is None:
        registry = json.loads(source_file(root, REGISTRY_PATH)[1])
    selected = reviewed_inputs(root, registry, spec_path, require_catalog)
    dispatcher = load_dispatcher(root, selected["bindings"])
    graph = dispatcher.graph.Graph(selected["spec"], selected["plan"])
    document = tomllib.loads(selected["spec"].read_text())
    if document.get("meta", {}).get("feature") != "production-system":
        raise ControlPlaneError("reviewed qualification graph must be production-system")
    if graph.layout_digest != selected["bindings"].get("layout_sha256"):
        raise ControlPlaneError("reviewed graph layout differs")
    if spec is not None and document != spec:
        raise ControlPlaneError("caller specification differs from canonical graph")
    selected.update(graph=graph, dispatcher=dispatcher, document=document)
    return selected


def tokens(selected, root):
    values = {
        "{reviewed_spec}": str(selected["spec"].relative_to(root)),
        "{reviewed_plan}": str(selected["plan"].relative_to(root)),
    }
    if "catalog" in selected:
        values.update({
            "{interface_catalog}": str(selected["catalog"].relative_to(root)),
            "{interface_manifest}": str(selected["catalog_manifest"].relative_to(root)),
            "{interface_root}": str(selected["catalog_manifest"].parent.relative_to(root)),
        })
    return values


def bind(value, selected, root):
    for token, replacement in tokens(selected, root).items():
        value = value.replace(token, replacement)
    if re.search(r"\{(?:reviewed_|interface_)[a-z_]+\}", value):
        raise ControlPlaneError("unbound reviewed qualification input")
    return value


def verify_inputs(root, selected):
    for name in ("spec", "plan", "catalog", "catalog_manifest"):
        if name in selected:
            identity = selected["bindings"][name]
            source_file(root, identity["path"], identity["sha256"])
    for name, expected in selected["retained"]["files"].items():
        source_file(root, name, expected)
    for name, expected in selected["bindings"]["dispatcher_sources"].items():
        source_file(root, f"{DISPATCHER_PATH}/{name}", expected)
    for name, expected in selected["bindings"]["runner_sources"].items():
        source_file(root, name, expected)
