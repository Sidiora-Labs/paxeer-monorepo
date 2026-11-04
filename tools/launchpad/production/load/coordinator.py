"""Source-bound root programme admission; no inferred replacement task numbers."""
from dataclasses import asdict
from pathlib import Path
import re

from phase2_types import ROOT, bound_document, digest
from programme import load_programme

SPEC_PATH = "spec/layerx-merge/spec.kvx"
REPAIR_TASK = "22.33.965"


def task_at(spec, identifier):
    if not isinstance(identifier, str) or not re.fullmatch(r"\d+(?:\.\d+)+", identifier):
        raise ValueError("exact programme task identifier required")
    task = spec.get("task", {})
    for part in identifier.split("."):
        task = task.get(part, {})
    if not isinstance(task, dict) or "status" not in task or "title" not in task:
        raise ValueError("coordinator task absent from current programme")
    return task


def validate_coordinator(config, bindings, runtime, *, root=ROOT):
    expected = config["coordinator_authority_sha256"]
    authority = bound_document(config["coordinator_authority_path"], expected)
    if authority["bindings"] != asdict(bindings) or runtime["coordinator_authority_sha256"] != expected:
        raise ValueError("reviewed coordinator authority bindings differ")
    entry = authority["spec"]
    if entry["path"] != SPEC_PATH:
        raise ValueError("current root programme authority required")
    path = root / entry["path"]
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 64 * 1024 * 1024:
        raise ValueError("bounded current programme required")
    raw = path.read_bytes()
    if digest(raw) != entry["sha256"]:
        raise ValueError("coordinator programme digest differs")
    spec, parsed_raw = load_programme(path)
    if parsed_raw != raw:
        raise ValueError("coordinator programme changed while reading admission")
    if spec.get("meta", {}).get("slug") != "layerx-merge":
        raise ValueError("coordinator programme differs")
    freeze, final = authority["freeze_task"], authority["final_task"]
    if freeze == final or task_at(spec, freeze)["status"] != "done" or task_at(spec, final)["status"] != "in_progress":
        raise ValueError("final execution requires a completed freeze and claimed final task")
    if freeze not in task_at(spec, final).get("requires", []):
        raise ValueError("final task must depend on reviewed source freeze")
    if runtime["final_task"] != final or runtime["coordinator_source_sha256"] != bindings.candidate:
        raise ValueError("final coordinator source freeze absent")
    return authority
