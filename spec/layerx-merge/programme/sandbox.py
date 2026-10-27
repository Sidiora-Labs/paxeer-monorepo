from __future__ import annotations

from collections.abc import Mapping
from os import PathLike
from pathlib import Path

from .kvx import KvxError, KvxValue, load

MANIFEST = Path(__file__).resolve().parent / "data/sandbox-profiles.kvx"


def profiles() -> dict[str, dict[str, KvxValue]]:
    return {
        name.removeprefix("profile."): profile
        for name, profile in load(MANIFEST).items()
        if name.startswith("profile.")
    }


def unknown_profiles(spec_path: str | PathLike[str]) -> list[str]:
    known = profiles()
    unknown = []
    for name, task in load(spec_path).items():
        if not name.startswith("task."):
            continue
        profile = task.get("sandbox_profile")
        if not isinstance(profile, str) or profile not in known:
            unknown.append(name.removeprefix("task."))
    return unknown


def prebuild_for(task: Mapping[str, KvxValue]) -> list[str]:
    profile = task.get("sandbox_profile")
    known = profiles()
    if not isinstance(profile, str) or profile not in known:
        raise KvxError(f"unknown sandbox_profile: {profile!r}")
    prebuild = known[profile]["prebuild"]
    if not isinstance(prebuild, list) or any(
        not command.strip() for command in prebuild
    ):
        raise KvxError(f"profile.{profile}: prebuild must be a list of commands")
    return prebuild
