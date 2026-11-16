#!/usr/bin/env python3
"""Compose launchpad startup-document environment files."""

from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import json
import os
import sys
from pathlib import Path
from types import ModuleType
from typing import Any


ROOT = Path(__file__).resolve().parent
CONTRACT_PATH = ROOT.parent / "railway" / "variables.json"


class ComposeError(ValueError):
    """The contract and values directory do not describe the same documents."""


def _ensure_module_path() -> None:
    """Make the document directory importable regardless of the caller's cwd."""

    root = str(ROOT)
    if root not in sys.path:
        sys.path.insert(0, root)


def _contract() -> dict[str, dict[str, Any]]:
    try:
        contract = json.loads(CONTRACT_PATH.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise ComposeError("cannot read launchpad variable contract") from exc
    services = {}
    for name, entry in contract.items():
        required = entry.get("required", [])
        if any(variable.endswith("_DOCUMENT") for variable in required) or entry.get("documentSources"):
            services[name] = entry
    return services


def _load_module(service: str) -> ModuleType | None:
    _ensure_module_path()
    module_path = ROOT / (service.replace("-", "_") + ".py")
    if not module_path.is_file() or module_path.name == "compose.py":
        return None
    name = service.replace("-", "_")
    loaded = sys.modules.get(name)
    if loaded is not None and Path(getattr(loaded, "__file__", "")).resolve() == module_path:
        return loaded
    spec = importlib.util.spec_from_file_location(name, module_path)
    if spec is None or spec.loader is None:
        raise ComposeError(f"{service}: cannot load document module")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def _module_documents(module: ModuleType, service: str, values: Path) -> dict[str, Any]:
    provider = next((getattr(module, name, None) for name in ("documents", "compose", "get_documents") if hasattr(module, name)), None)
    if provider is None:
        raise ComposeError(f"{service}: document module has no documents function")
    try:
        result = provider(service, values)
    except TypeError:
        result = provider(values)
    if not isinstance(result, dict):
        raise ComposeError(f"{service}: document module must return an object")
    return result


def _as_bytes(service: str, name: str, value: Any) -> bytes:
    if isinstance(value, bytes):
        data = value
    else:
        try:
            data = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
        except (TypeError, ValueError) as exc:
            raise ComposeError(f"{service}: document {name} is not JSON") from exc
    if not data:
        raise ComposeError(f"{service}: document {name} is empty")
    return data


def _validate_document_sources(service: str, supplied: dict[str, Any], sources: dict[str, str]) -> None:
    for document, path in sources.items():
        if document in supplied and path in supplied:
            raise ComposeError(f"{service}: {document} and {path} are both provided")
        if document not in supplied and path not in supplied:
            raise ComposeError(f"{service}: required document {document} or {path} is not provided")


def _compose_service(service: str, values: Path, entry: dict[str, Any]) -> tuple[dict[str, str], dict[str, str]]:
    _ensure_module_path()
    sources = entry.get("documentSources", {})
    if not isinstance(sources, dict) or any(
        not isinstance(name, str) or not name.endswith("_DOCUMENT")
        or path != name[:-len("_DOCUMENT")] + "_FILE"
        or name[:-len("_DOCUMENT")] + "_SHA256" not in entry.get("required", [])
        for name, path in sources.items()
    ):
        raise ComposeError(f"{service}: invalid documentSources contract")
    contract_documents = {name for name in entry.get("required", []) if name.endswith("_DOCUMENT")} | set(sources)
    if not contract_documents:
        raise ComposeError(f"{service}: contract has no document variables")
    from common import DocumentError, FileDocument, documents as common_documents

    try:
        supplied: dict[str, Any] = dict(common_documents(service, values))
    except DocumentError as exc:
        raise ComposeError(str(exc)) from exc
    module = _load_module(service)
    if module is not None:
        try:
            supplied.update(_module_documents(module, service, values))
        except ValueError as exc:
            raise ComposeError(str(exc)) from exc
    # The shared composer is released before the service-specific composers.
    # Until a service module exists, its contract's future document variables
    # have no producer and are deliberately left for that module. Once the
    # module is present, every contract document is mandatory.
    required = contract_documents if module is not None else {name for name in ("DB_BUDGET_DOCUMENT", "EXPECTED_SCHEMA_DOCUMENT") if name in contract_documents}
    unknown = sorted(set(supplied) - contract_documents - set(sources.values()))
    if unknown:
        raise ComposeError(f"{service}: document {unknown[0]} is not required by the contract")
    _validate_document_sources(service, supplied, sources)
    missing = sorted(name for name in required if name not in supplied and sources.get(name) not in supplied)
    if missing:
        raise ComposeError(f"{service}: required document {missing[0]} is not provided")

    values_out: dict[str, str] = {}
    digests: dict[str, str] = {}
    for name in sorted(supplied):
        value = supplied[name]
        if name in sources.values():
            if not isinstance(value, FileDocument) or not isinstance(value.path, str) or not value.path.strip() or not isinstance(value.data, bytes):
                raise ComposeError(f"{service}: document {name} must provide rendered bytes and a runtime path")
            data = _as_bytes(service, name, value.data)
            values_out[name] = value.path
            digest_name = name[:-len("_FILE")] + "_SHA256"
        else:
            data = _as_bytes(service, name, value)
            values_out[name] = base64.b64encode(data).decode("ascii")
            digest_name = name[:-len("_DOCUMENT")] + "_SHA256"
        digest = hashlib.sha256(data).hexdigest()
        values_out[digest_name] = digest
        digests[digest_name] = digest
    return values_out, digests


def _write(path: Path, value: dict[str, str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, "wb") as output:
            descriptor = -1
            output.write(payload)
    finally:
        if descriptor != -1:
            os.close(descriptor)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group()
    source.add_argument("--values", type=Path, help="values directory")
    source.add_argument(
        "--examples",
        nargs="?",
        const=ROOT / "values",
        type=Path,
        help="use the checked-in example values directory or a supplied examples directory",
    )
    parser.add_argument("--out", type=Path, required=True, help="output directory")
    parser.add_argument("--service", action="append", help="service to compose; may be repeated")
    args = parser.parse_args(argv)
    values = args.values if args.values is not None else args.examples
    if values is None:
        parser.error("one of --values or --examples is required")
    if not values.is_dir():
        raise ComposeError(f"missing values file {values}")
    contract = _contract()
    services = args.service or list(contract)
    for service in services:
        if service not in contract:
            raise ComposeError(f"unknown service {service}")
        files, digests = _compose_service(service, values, contract[service])
        _write(args.out / f"{service}.json", files)
        for name, digest in sorted(digests.items()):
            print(f"{service} {name} {digest}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ComposeError as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(2)

