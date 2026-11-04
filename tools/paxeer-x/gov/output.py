"""Confine governance output replacement to named generated files.

The selected output directory must be exclusively owned by the renderer while
it runs. Bridge generation happens in a fresh directory before publication, so
operator files in an existing bridge directory never need to be removed.
"""

import json
import os
from pathlib import Path
import re
import stat
import sys


TOP_LEVEL = (
    "01-software-upgrade.json",
    "02-fee-token-params.json",
    "03-xweb-fee.json",
)
BODY_FILES = ("01-register-chain.json", "02-set-attestors.json")
PROPOSAL_FILES = ("04-proposal-open-chain.json", "05-proposal-sidiora-cap.json")
SAFE_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]*", re.ASCII)
CAP_FILE = re.compile(r"03-set-cap-(?:0[1-9]|[1-9][0-9]+)-[0-9a-f]{40}\.json", re.ASCII)


def contained(root, path):
    resolved = path.resolve()
    if resolved == root or not resolved.is_relative_to(root):
        raise ValueError(f"output escapes selected directory: {path}")
    return path


def directory(root, path):
    contained(root, path)
    current = root
    for part in path.relative_to(root).parts:
        current /= part
        if current.is_symlink():
            raise ValueError(f"symlink output directory: {current}")
        if current.exists():
            if not current.is_dir():
                raise ValueError(f"output is not a directory: {current}")
        else:
            current.mkdir()
    return path


def regular_output(root, path):
    contained(root, path)
    if path.is_symlink():
        raise ValueError(f"symlink output file: {path}")
    try:
        mode = path.lstat().st_mode
    except FileNotFoundError:
        return path
    if not stat.S_ISREG(mode):
        raise ValueError(f"output is not a regular file: {path}")
    return path


def prepare_output(output):
    root = Path(output).resolve()
    root.mkdir(parents=True, exist_ok=True)
    files = [
        regular_output(root, root / (name + suffix))
        for name in TOP_LEVEL
        for suffix in ("", ".tx.json", ".err")
    ]
    for path in files:
        # Unlink instead of truncating an existing inode, which may be linked
        # to an operator file outside the output directory.
        path.unlink(missing_ok=True)
    return root


def bridge_directory(output, config):
    with Path(config).open() as stream:
        config = json.load(stream)
    name = config.get("name") if isinstance(config, dict) else None
    if not isinstance(name, str) or SAFE_NAME.fullmatch(name) is None:
        raise ValueError("bridge name must be one ASCII alphanumeric identifier with '-' or '_'")
    root = Path(output).resolve(strict=True)
    return directory(root, root / "bridge" / name)


def generated_file(section, name):
    if section == "bodies":
        return name in BODY_FILES or CAP_FILE.fullmatch(name) is not None
    if section == "proposals":
        return any(name == base + suffix for base in PROPOSAL_FILES
                   for suffix in ("", ".tx.json", ".err"))
    return False


def publish_bridge(output, destination, staging):
    root = Path(output).resolve(strict=True)
    destination, staging = Path(destination), Path(staging)
    relative = destination.relative_to(root)
    if (len(relative.parts) != 2 or relative.parts[0] != "bridge"
            or SAFE_NAME.fullmatch(relative.parts[1]) is None):
        raise ValueError("invalid bridge destination")
    directory(root, destination)
    if (staging.parent != root or not staging.name.startswith(".bridge-render.")
            or staging.is_symlink() or not staging.is_dir()):
        raise ValueError("invalid bridge staging directory")
    contained(root, staging)
    if {path.name for path in staging.iterdir()} != {"bodies", "proposals"}:
        raise ValueError("unexpected bridge staging contents")

    replacements, obsolete = [], []
    for section, required in (("bodies", BODY_FILES), ("proposals", PROPOSAL_FILES[:1])):
        source_dir = directory(root, staging / section)
        target_dir = directory(root, destination / section)
        sources = list(source_dir.iterdir())
        if not set(required).issubset(path.name for path in sources):
            raise ValueError(f"incomplete generated {section}")
        for source in sources:
            if (not generated_file(section, source.name)
                    or (section == "proposals" and source.name not in PROPOSAL_FILES)):
                raise ValueError(f"unexpected generated file: {source}")
            regular_output(root, source)
            target = regular_output(root, target_dir / source.name)
            replacements.append((source, target))
        names = {source.name for source in sources}
        for target in target_dir.iterdir():
            if generated_file(section, target.name):
                regular_output(root, target)
                if target.name not in names:
                    obsolete.append(target)

    # Validate every source, destination and stale sidecar before replacing any
    # files. Unknown operator files are deliberately absent from these lists.
    for source, target in replacements:
        os.replace(source, target)
    for path in obsolete:
        path.unlink()


def main(args):
    if len(args) == 2 and args[0] == "prepare":
        print(prepare_output(args[1]))
    elif len(args) == 3 and args[0] == "bridge":
        print(bridge_directory(args[1], args[2]))
    elif len(args) == 4 and args[0] == "publish":
        publish_bridge(*args[1:])
    else:
        raise ValueError("usage: output.py prepare OUT | bridge OUT CONFIG | publish OUT DIR STAGE")


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except (OSError, ValueError) as error:
        sys.exit(f"render.sh: {error}")
