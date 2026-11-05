"""Install authentic source peers in an explicitly isolated qualification root.

These test approvals exercise the registry codec; they are never published as
production approvals, catalogues, scheduler state or accepted execution evidence.
"""

import copy
import hashlib
import json
from pathlib import Path
import shutil

SOURCE = Path(__file__).resolve().parents[1]
ROOT = SOURCE.parents[2]


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, sort_keys=True, indent=2) + "\n")


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def identity(root, path):
    return {"path": str(path.relative_to(root)), "sha256": sha(path)}


def install_review(root, spec_path, plan_path, graph, control, *, catalog=None, manifest=None):
    canonical = json.loads((SOURCE / "bootstrap/check-registry.json").read_bytes())
    retained = copy.deepcopy(canonical["retained_inputs"])
    for relative in (*retained["files"], control.RETAINED_SPEC_PATH, *control.RUNNER_PATHS):
        target = root / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / relative, target)
    package = root / control.DISPATCHER_PATH
    if not package.exists():
        shutil.copytree(SOURCE / "dispatcher", package, ignore=shutil.ignore_patterns("__pycache__", "tests"))
    bindings = {
        "retained_sha256": retained["sha256"],
        "spec": identity(root, spec_path),
        "plan": identity(root, plan_path),
        "layout_sha256": graph.layout_digest,
        "dispatcher_sources": {path.name: sha(path) for path in package.glob("*.py")},
        "runner_sources": {name: sha(root / name) for name in control.RUNNER_PATHS},
    }
    if catalog is not None:
        bindings["catalog"] = identity(root, catalog)
        bindings["catalog_manifest"] = identity(root, manifest)
    registry = {
        "schema_version": 1,
        "tasks": {},
        "retained_inputs": retained,
        "control_plane": {
            "schema_version": 1,
            "status": "approved",
            "purpose": "isolated committed peer; no production migration approval",
            "approval": {
                "reference": "test-recording:22.33.973",
                "bindings_sha256": control.digest(bindings),
            },
            "bindings": bindings,
        },
    }
    write_json(root / control.REGISTRY_PATH, registry)
    return registry


def project_plan(path, graph, document, spec_path):
    # The real Graph reader produces task/ownership records; JSON is the actual
    # execution-plan transport, including the source byte digest and lane rows.
    write_json(path, {
        "spec_sha256": sha(spec_path),
        "tasks": graph.tasks,
        "lanes": [
            {
                "id": lane,
                "write_paths": definition["write_paths"],
                "tasks": [dict(id=task, **row) for task, row in graph.tasks.items() if row.get("lane") == lane],
            }
            for lane, definition in document["lane"].items()
        ],
    })
