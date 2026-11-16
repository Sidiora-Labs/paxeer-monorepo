"""Account event subjects for the launchpad user service."""

from __future__ import annotations

import json
import re
from pathlib import Path

from common import DocumentError


EVENT_TYPES = (
    "profile.updated",
    "preferences.updated",
    "watchlist.changed",
    "consent.updated",
    "endpoint.bound",
    "endpoint.revoked",
    "account.export.requested",
    "account.deletion.requested",
    "account.deletion.job",
    "account.backfill.completed",
)
SUBJECT_NAME = re.compile(r"[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*", re.ASCII)


def documents(service: str, values: str | Path) -> dict[str, dict[str, str]]:
    if service != "user":
        raise DocumentError("user: service must be user")
    directory = Path(values)
    path = next(
        (directory / name for name in ("user.json", "user.example.json")
         if (directory / name).is_file()),
        None,
    )
    if path is None:
        raise DocumentError("user: missing values file user.json")
    try:
        value = json.loads(path.read_bytes())
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise DocumentError("user: invalid values file user.json") from exc
    if not isinstance(value, dict):
        raise DocumentError("user: user.json must be an object")
    for key in value:
        if key != "subjects":
            raise DocumentError(f"user: unknown key {key}")
    subjects = value.get("subjects")
    if not isinstance(subjects, dict):
        raise DocumentError("user: subjects must be an object")
    for kind in EVENT_TYPES:
        if kind not in subjects:
            raise DocumentError(f"user: missing subjects.{kind}")
    seen: set[str] = set()
    for kind, subject in subjects.items():
        if not SUBJECT_NAME.fullmatch(kind):
            raise DocumentError(f"user: invalid subjects.{kind}")
        if (not isinstance(subject, str) or len(subject) > 256
                or not SUBJECT_NAME.fullmatch(subject)):
            raise DocumentError(f"user: invalid subjects.{kind}")
        if subject in seen:
            raise DocumentError(f"user: duplicate subject at subjects.{kind}")
        seen.add(subject)
    return {"ACCOUNT_SUBJECTS_DOCUMENT": subjects}
