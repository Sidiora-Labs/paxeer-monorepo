"""Publication startup document for the metadata service."""

from __future__ import annotations

import json
import re
from pathlib import Path

from common import DocumentError


def documents(service: str, values: str | Path) -> dict[str, dict[str, object]]:
    if service != "metadata":
        raise DocumentError(f"metadata: unsupported service {service}")
    directory = Path(values)
    path = directory / "metadata.json"
    if not path.is_file():
        path = directory / "metadata.example.json"
    try:
        settings = json.loads(path.read_bytes())
    except (OSError, ValueError) as exc:
        raise DocumentError("metadata: cannot read values key metadata.json") from exc
    if not isinstance(settings, dict):
        raise DocumentError("metadata: metadata.json must be an object")
    unknown = sorted(settings.keys() - {"environment"})
    if unknown:
        raise DocumentError(f"metadata: unknown values key {unknown[0]}")
    environment = settings.get("environment")
    if not isinstance(environment, str) or re.fullmatch(r"[a-zA-Z0-9_.:-]{1,128}", environment) is None:
        raise DocumentError("metadata: environment must match [a-zA-Z0-9_.:-]{1,128}")
    return {
        "METADATA_PUBLICATION_CONFIG_DOCUMENT": {
            "schemaVersion": 3,
            "environment": environment,
            "envelopeHeader": "X-Kindle-Operation",
            "signatureHeader": "X-Kindle-Operation-Signature",
        }
    }
