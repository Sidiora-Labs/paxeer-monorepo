from __future__ import annotations

import re
import shlex
import tomllib
from collections.abc import Mapping
from pathlib import Path, PurePosixPath

from .kvx import KvxValue

ROOT = Path(__file__).resolve().parents[3]


class ParityError(ValueError):
    pass


def _rust_target(path: PurePosixPath) -> tuple[PurePosixPath, str, str]:
    if path.parent.name != "tests":
        raise ParityError(f"Rust selector must name an integration test: {path}")
    crate = path.parent.parent
    manifest = ROOT / crate / "Cargo.toml"
    try:
        with manifest.open("rb") as source:
            package = tomllib.load(source)["package"]
        name = package["name"]
        if not isinstance(name, str) or not name:
            raise ParityError(f"missing Rust crate name: {manifest}")
        if "workspace" in package:
            workspace_manifest = manifest.parent / package["workspace"] / "Cargo.toml"
            with workspace_manifest.open("rb") as source:
                if "workspace" not in tomllib.load(source):
                    raise ParityError(f"missing Rust workspace: {workspace_manifest}")
            manifest = workspace_manifest.resolve()
        else:
            for parent in manifest.parents:
                candidate = parent / "Cargo.toml"
                if candidate.is_file():
                    with candidate.open("rb") as source:
                        if "workspace" in tomllib.load(source):
                            manifest = candidate
                            break
                if parent == ROOT:
                    break
        return PurePosixPath(manifest.relative_to(ROOT).as_posix()), name, path.stem
    except (OSError, KeyError, TypeError, ValueError) as error:
        raise ParityError(f"cannot resolve Rust selector {path}: {error}") from error


def command_for(row: Mapping[str, KvxValue]) -> str:
    selectors = row.get("selectors")
    if not isinstance(selectors, list) or not selectors:
        raise ParityError("selectors must be a nonempty list")
    parsed: list[tuple[PurePosixPath, str]] = []
    for selector in selectors:
        if not isinstance(selector, str):
            raise ParityError("each selector must be a path::test string")
        path_text, separator, name = selector.partition("::")
        if (
            not separator
            or not re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*", path_text)
            or {".", ".."}.intersection(path_text.split("/"))
            or not re.fullmatch(
                r"[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*", name
            )
        ):
            raise ParityError(f"invalid exact selector: {selector}")
        parsed.append((PurePosixPath(path_text), name))

    suffixes = {path.suffix for path, _ in parsed}
    if len(suffixes) != 1:
        raise ParityError("selectors must use one toolchain")
    suffix = parsed[0][0].suffix
    if suffix == ".go":
        packages = {path.parent for path, _ in parsed}
        if len(packages) != 1:
            raise ParityError("Go selectors span two packages")
        if any(
            not path.name.endswith("_test.go")
            or not re.fullmatch(r"Test[A-Za-z0-9_]+", name)
            for path, name in parsed
        ):
            raise ParityError("Go selectors must name declared Test functions")
        package = parsed[0][0].parent.as_posix()
        target = "./" if package == "." else f"./{package}/"
        pattern = "^(" + "|".join(dict.fromkeys(name for _, name in parsed)) + ")$"
        return shlex.join(["go", "test", "-count=1", target, "-run", pattern])
    if suffix == ".py":
        return shlex.join(["python3", "-m", "pytest", *selectors, "-q"])
    if suffix == ".rs":
        targets = [(_rust_target(path), name) for path, name in parsed]
        crates = {(manifest, crate) for (manifest, crate, _), _ in targets}
        if len(crates) != 1:
            raise ParityError("Rust selectors span two crates")
        manifest, crate = targets[0][0][:2]
        tests: dict[str, list[str]] = {}
        for (_, _, file), name in targets:
            tests.setdefault(file, []).append(name)
        return " && ".join(
            shlex.join(
                [
                    "cargo",
                    "test",
                    "--locked",
                    "--manifest-path",
                    str(manifest),
                    "-p",
                    crate,
                    "--test",
                    file,
                    "--",
                    "--exact",
                    *dict.fromkeys(names),
                ]
            )
            for file, names in tests.items()
        )
    raise ParityError(f"unsupported selector toolchain: {suffix}")
