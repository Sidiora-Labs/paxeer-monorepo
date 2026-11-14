import json
import os
import posixpath
import re
import shlex
import subprocess
from dataclasses import dataclass, field
from fnmatch import translate
from pathlib import Path, PurePosixPath

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
BETA_TABLE = ROOT / "services/platform/hosted/tests/beta-images.sh"
OFFCHAIN_TABLE = ROOT / "tools/deploy/offchain-images.sh"
EXPLORER_WORKFLOW = ROOT / ".github/workflows/explorer-images.yml"


def bash_output(script, *arguments, env=None):
    try:
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", script, "image-context", *arguments],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=True,
        )
    except subprocess.CalledProcessError as error:
        pytest.fail(
            f"Image inventory command failed (exit {error.returncode}): "
            f"{error.cmd!r}\n{error.stdout}\n{error.stderr}",
            pytrace=False,
        )
    return result.stdout


@pytest.fixture(scope="session")
def image_rows(tmp_path_factory):
    work = tmp_path_factory.mktemp("image-contexts")
    env = {**os.environ, "REPO_ROOT": str(ROOT), "WORK_DIR": str(work)}
    beta = bash_output('source "$1"; image_table_rows', str(BETA_TABLE), env=env)
    rows = []
    for line in beta.splitlines():
        name, context, dockerfile, target = line.split("\t")
        if context == "@registry-builder":
            context = bash_output(
                'source "$1"; registry_builder_context_prepare',
                str(BETA_TABLE),
                env=env,
            ).strip()
            dockerfile = str(Path(context) / "Dockerfile")
        rows.append((f"beta/{name}", ROOT / context, ROOT / dockerfile, target, {}))
    assert rows, "Beta image table is empty"

    offchain = bash_output(
        'source "$1" --matrix >/dev/null; printf \'%s\\n\' "${rows[@]}"',
        str(OFFCHAIN_TABLE),
        env=env,
    )
    offchain_rows = []
    for line in offchain.splitlines():
        name, context, dockerfile, target, arguments, required = line.split("|")
        assert required in {"0", "1"}, line
        args = dict(argument.split("=", 1) for argument in shlex.split(arguments))
        args = {name: value for name, value in args.items() if value}
        offchain_rows.append(
            (f"offchain/{name}", ROOT / context, ROOT / dockerfile, target, args)
        )
    assert offchain_rows, "Offchain image table is empty"
    rows.extend(offchain_rows)

    workflow = yaml.safe_load(EXPLORER_WORKFLOW.read_text(encoding="utf-8"))
    explorer = workflow["jobs"]["publish"]["strategy"]["matrix"]["include"]
    assert explorer, "Explorer image matrix is empty"
    for row in explorer:
        args = dict(
            line.split("=", 1)
            for line in row.get("build_args", "").splitlines()
            if line.strip()
        )
        rows.append(
            (
                f"explorer/{row['image']}",
                ROOT / row["context"],
                ROOT / row["dockerfile"],
                row.get("target", ""),
                args,
            )
        )
    assert len({row[0] for row in rows}) == len(rows), "Duplicate image rows"
    return rows


def instructions(dockerfile):
    pending = ""
    heredocs = []
    for number, raw in enumerate(dockerfile.read_text(encoding="utf-8").splitlines(), 1):
        if heredocs:
            if raw.strip() == heredocs[0]:
                heredocs.pop(0)
            continue
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if not pending:
            start = number
        if line.endswith("\\"):
            pending += line[:-1] + " "
            continue
        line = pending + line
        pending = ""
        parts = line.split(maxsplit=1)
        assert len(parts) == 2, f"{dockerfile}:{start}: invalid instruction {line}"
        heredocs = re.findall(r"<<-?['\"]?(\w+)['\"]?", parts[1])
        yield start, parts[0].upper(), parts[1]
    assert not pending and not heredocs, f"{dockerfile}: unfinished instruction"


def expand(value, variables, strict=True):
    def replace(match):
        name = match[1] or match[2]
        if name in variables:
            return variables[name]
        assert not strict, f"Unresolved Docker variable {name} in {value}"
        return match[0]

    return re.sub(r"\$\{(\w+)\}|\$(\w+)", replace, value)


@dataclass
class Stage:
    variables: dict
    dependencies: set = field(default_factory=set)
    sources: list = field(default_factory=list)


def target_copy_sources(dockerfile, target, build_args):
    global_args = {
        "TARGETARCH": "amd64",
        "TARGETOS": "linux",
        "TARGETPLATFORM": "linux/amd64",
        "BUILDPLATFORM": "linux/amd64",
    }
    stages = []
    aliases = {}
    current = None

    def dependency(reference):
        if reference.lower() in aliases:
            current.dependencies.add(aliases[reference.lower()])
        else:
            assert not reference.isdigit(), f"Unknown stage {reference} in {dockerfile}"

    for number, operation, arguments in instructions(dockerfile):
        if operation == "ARG":
            name, separator, default = arguments.partition("=")
            variables = global_args if current is None else current.variables
            if name in build_args:
                variables[name] = build_args[name]
            elif separator:
                variables[name] = expand(default, variables, strict=False)
            elif name in global_args:
                variables[name] = global_args[name]
        elif operation == "FROM":
            words = shlex.split(arguments)
            while words[0].startswith("--"):
                words.pop(0)
            base = expand(words[0], global_args, strict=False)
            parent = aliases.get(base.lower())
            current = Stage(dict(stages[parent].variables) if parent is not None else {})
            if parent is not None:
                current.dependencies.add(parent)
            assert len(words) == 1 or (len(words) == 3 and words[1].upper() == "AS")
            index = len(stages)
            if len(words) == 3:
                alias = words[2].lower()
                assert alias not in aliases, f"Duplicate stage {alias} in {dockerfile}"
                aliases[alias] = index
            aliases[str(index)] = index
            stages.append(current)
        elif operation in {"COPY", "ADD"}:
            assert current is not None, f"{dockerfile}:{number}: COPY before FROM"
            options = {}
            while arguments.startswith("--"):
                option, arguments = arguments.split(maxsplit=1)
                key, _, value = option[2:].partition("=")
                options[key] = value
            if "from" in options:
                dependency(expand(options["from"], current.variables))
                continue
            paths = (
                json.loads(arguments)
                if arguments.startswith("[")
                else shlex.split(arguments)
            )
            assert len(paths) >= 2, (
                f"{dockerfile}:{number}: COPY needs source and destination"
            )
            for source in paths[:-1]:
                if operation == "ADD" and re.match(r"(?:https?|git|ssh)://", source):
                    continue
                assert not source.startswith("<<"), "Inline COPY needs explicit parsing"
                current.sources.append((number, expand(source, current.variables)))
        elif operation == "ENV":
            words = shlex.split(arguments)
            if "=" not in words[0]:
                current.variables[words[0]] = expand(
                    " ".join(words[1:]), current.variables, False
                )
            else:
                updates = dict(word.split("=", 1) for word in words)
                current.variables.update(
                    {
                        key: expand(value, current.variables, False)
                        for key, value in updates.items()
                    }
                )
        elif operation == "RUN":
            for mount in re.findall(r"--mount=([^\s]+)", arguments):
                options = dict(
                    item.split("=", 1) for item in mount.split(",") if "=" in item
                )
                if "from" in options:
                    dependency(expand(options["from"], current.variables))

    assert stages, f"No stages in {dockerfile}"
    assert not target or target.lower() in aliases, (
        f"Unknown target {target} in {dockerfile}"
    )
    pending = [aliases[target.lower()] if target else len(stages) - 1]
    visited = set()
    sources = []
    while pending:
        index = pending.pop()
        if index in visited:
            continue
        visited.add(index)
        sources.extend(stages[index].sources)
        pending.extend(stages[index].dependencies)
    return sources


def ignore_pattern(pattern):
    parts = pattern.split("/")
    expression = ""
    for index, part in enumerate(parts):
        if part == "**":
            expression += ".*" if index == len(parts) - 1 else "(?:[^/]+/)*"
        else:
            expression += translate(part)[:-2]
            if index != len(parts) - 1:
                expression += "/"
    return re.compile(expression)


def ignore_rules(context, dockerfile):
    ignore = Path(str(dockerfile) + ".dockerignore")
    if not ignore.is_file():
        ignore = context / ".dockerignore"
    if not ignore.is_file():
        return []
    rules = []
    for raw in ignore.read_text(encoding="utf-8-sig").splitlines():
        if raw.startswith("#") or not raw.strip():
            continue
        pattern = raw.strip()
        negated = pattern.startswith("!")
        if negated:
            pattern = pattern[1:].strip()
        assert pattern, f"Empty exclusion in {ignore}"
        pattern = posixpath.normpath(pattern).strip("/")
        if pattern != ".":
            rules.append((negated, ignore_pattern(pattern)))
    return rules


def ignored(path, rules):
    path = PurePosixPath(path)
    candidates = [
        candidate.as_posix()
        for candidate in [path, *path.parents]
        if str(candidate) != "."
    ]
    excluded = False
    for negated, pattern in rules:
        if any(pattern.fullmatch(candidate) for candidate in candidates):
            excluded = not negated
    return excluded


def source_paths(context, source):
    source = posixpath.normpath(source.lstrip("/"))
    assert ".." not in PurePosixPath(source).parts, f"COPY escapes context: {source}"
    if any(char in source for char in "*?["):
        return list(context.glob(source))
    return [context / source]


def source_included(path, context, rules):
    if not ignored(path.relative_to(context).as_posix(), rules):
        return True
    return path.is_dir() and any(
        not ignored(child.relative_to(context).as_posix(), rules)
        for child in path.rglob("*")
        if child.is_file() or child.is_symlink()
    )


def test_registry_chain_trust_not_ignored():
    dockerfile = ROOT / "docker/platform-registry/Dockerfile"
    source = "docker/platform-registry/chain-trust.py"
    copies = target_copy_sources(dockerfile, "env-init", {})
    assert source in {path for _, path in copies}, "env-init lost its chain trust script"
    assert (ROOT / source).is_file()
    assert not ignored(source, ignore_rules(ROOT, dockerfile))


def test_table_image_copy_sources_exist(image_rows):
    failures = []
    checked = 0
    for name, context, dockerfile, target, args in image_rows:
        assert context.is_dir(), f"{name}: missing context {context}"
        assert dockerfile.is_file(), f"{name}: missing Dockerfile {dockerfile}"
        for number, source in target_copy_sources(dockerfile, target, args):
            checked += 1
            paths = source_paths(context, source)
            if not paths or not all(path.exists() or path.is_symlink() for path in paths):
                failures.append(
                    f"{name}: {dockerfile}:{number}: missing COPY source {source}"
                )
    assert checked, "No local COPY sources checked"
    assert not failures, "\n".join(failures)


def test_table_image_copy_sources_not_ignored(image_rows):
    failures = []
    checked = 0
    for name, context, dockerfile, target, args in image_rows:
        assert context.is_dir(), f"{name}: missing context {context}"
        assert dockerfile.is_file(), f"{name}: missing Dockerfile {dockerfile}"
        rules = ignore_rules(context, dockerfile)
        for number, source in target_copy_sources(dockerfile, target, args):
            checked += 1
            paths = source_paths(context, source)
            if not paths or not any(
                source_included(path, context, rules) for path in paths
            ):
                failures.append(
                    f"{name}: {dockerfile}:{number}: excluded COPY source {source}"
                )
    assert checked, "No local COPY sources checked"
    assert not failures, "\n".join(failures)
