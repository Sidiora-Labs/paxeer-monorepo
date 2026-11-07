import ast
from functools import lru_cache
import json
from pathlib import Path
import re
import shlex
import tomllib

from test_websearch_context import admitted, rules


ROOT = Path(__file__).resolve().parents[2]
LAYERX = ROOT / "docker/layerx/Dockerfile"
SETTLEMENT = "contracts/config/checkpoint-settlement.json"
MARKDOWN = {
    "services/agent/crates/layerx-crypto/src/api-contract.md",
    "programs/crates/layerx-programs-runtime/src/runtime_contract.md",
}
RUST_TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|(?:br|r)(?P<hashes>\#*)".*?"(?P=hashes)'
    r'|b?"(?:\\.|[^"\\])*"|b?\'(?:\\.|[^\'\\])\''
    r'|[A-Za-z_][A-Za-z_0-9]*|[^\s]',
    re.DOTALL,
)


def instructions(dockerfile):
    text = re.sub(r"\\\r?\n", " ", dockerfile.read_text())
    for line in text.splitlines():
        parts = line.strip().split(maxsplit=1)
        if parts and not parts[0].startswith("#"):
            yield parts[0].upper(), parts[1] if len(parts) > 1 else ""


def copy_arguments(arguments):
    options = {}
    while arguments.startswith("--"):
        option, arguments = arguments.split(maxsplit=1)
        key, _, value = option[2:].partition("=")
        options[key] = value
    paths = json.loads(arguments) if arguments.startswith("[") else shlex.split(arguments)
    assert len(paths) >= 2, arguments
    return options, paths[:-1], paths[-1]


@lru_cache(maxsize=None)
def document(path):
    return tomllib.loads(path.read_text())


@lru_cache(maxsize=None)
def workspace(manifest):
    package = document(manifest).get("package", {})
    if "workspace" in package:
        return (manifest.parent / package["workspace"] / "Cargo.toml").resolve()
    for parent in (manifest.parent, *manifest.parent.parents):
        candidate = parent / "Cargo.toml"
        if candidate.is_file() and "workspace" in document(candidate):
            return candidate
    return manifest


def packages(manifest):
    data = document(manifest)
    result = {}
    if "package" in data:
        result[data["package"]["name"]] = manifest
    for member in data.get("workspace", {}).get("members", []):
        matches = list(manifest.parent.glob(member + "/Cargo.toml"))
        assert matches, f"Missing workspace member: {manifest}: {member}"
        for path in matches:
            result[document(path)["package"]["name"]] = path.resolve()
    return result


@lru_cache(maxsize=None)
def closure(manifest):
    visited = set()
    pending = [manifest]
    while pending:
        path = pending.pop().resolve()
        if path in visited:
            continue
        assert path.is_file(), path
        visited.add(path)
        data = document(path)
        inherited = workspace(path)
        for kind in ("dependencies", "build-dependencies"):
            tables = [data.get(kind, {})]
            tables.extend(target.get(kind, {}) for target in data.get("target", {}).values())
            for table in tables:
                for name, options in table.items():
                    if not isinstance(options, dict):
                        continue
                    base = path.parent
                    if options.get("workspace"):
                        options = document(inherited)["workspace"]["dependencies"][name]
                        base = inherited.parent
                    if isinstance(options, dict) and "path" in options:
                        pending.append((base / options["path"] / "Cargo.toml").resolve())
    return frozenset(visited)


@lru_cache(maxsize=None)
def image_builds():
    result = []
    for dockerfile in sorted((ROOT / "docker").rglob("Dockerfile*")):
        if dockerfile.name.endswith(".dockerignore"):
            continue
        copy_all = False
        stage = ""
        for instruction, arguments in instructions(dockerfile):
            if instruction == "FROM":
                stage = arguments.split()[-1]
                copy_all = False
            elif instruction == "COPY":
                options, sources, _ = copy_arguments(arguments)
                copy_all |= "from" not in options and "." in sources
            elif instruction == "RUN" and copy_all:
                for match in re.finditer(r"\bcargo\s+build\b([^;&|]*)", arguments):
                    argv = shlex.split(match[1])
                    manifests = [argv[i + 1] for i, arg in enumerate(argv) if arg == "--manifest-path"]
                    manifest = (ROOT / (manifests[0] if manifests else "Cargo.toml")).resolve()
                    assert manifest.is_file(), f"Unknown local Cargo build: {dockerfile}: {arguments}"
                    available = packages(manifest)
                    names = [argv[i + 1] for i, arg in enumerate(argv) if arg in ("-p", "--package")]
                    if not names:
                        names = [document(manifest)["package"]["name"]] if "package" in document(manifest) else list(available)
                    assert names, f"Empty image closure: {dockerfile}"
                    roots = frozenset(available[name] for name in names)
                    ignore = dockerfile.with_name(dockerfile.name + ".dockerignore")
                    if not ignore.is_file():
                        ignore = ROOT / ".dockerignore"
                    policy = rules(ignore.read_text()) if ignore.is_file() else []
                    result.append((dockerfile, stage, roots, policy))
    assert result, "No COPY-all Rust image builds discovered"
    return result


def closing(tokens, start):
    pairs = {"(": ")", "[": "]", "{": "}"}
    stack = [pairs[tokens[start]]]
    for index in range(start + 1, len(tokens)):
        token = tokens[index]
        if token in pairs:
            stack.append(pairs[token])
        elif token == stack[-1]:
            stack.pop()
            if not stack:
                return index
    raise AssertionError(f"Unclosed Rust delimiter: {tokens[start:start + 15]}")


def production_tokens(source):
    tokens = [match.group() for match in RUST_TOKEN.finditer(source)
              if not match.group().startswith(("//", "/*"))]
    result = []
    index = 0
    while index < len(tokens):
        if tokens[index] == "#":
            inner = tokens[index + 1] == "!"
            start = index + 2 if inner else index + 1
            if tokens[start] == "[":
                end = closing(tokens, start)
                attribute = tokens[start + 1:end]
                if attribute in (["cfg", "(", "test", ")"], ["test"]):
                    if inner:
                        return []
                    index = end + 1
                    while tokens[index] not in ("{", ";"):
                        if tokens[index] in ("[", "("):
                            index = closing(tokens, index)
                        index += 1
                    index = closing(tokens, index) + 1 if tokens[index] == "{" else index + 1
                    continue
        result.append(tokens[index])
        index += 1
    return result


def string_value(tokens, manifest):
    while tokens and tokens[-1] == ",":
        tokens = tokens[:-1]
    if len(tokens) == 1:
        raw = re.fullmatch(r'r(\#*)"(.*)"\1', tokens[0], re.DOTALL)
        return raw[2] if raw else ast.literal_eval(tokens[0])
    if tokens[:3] == ["env", "!", "("]:
        assert tokens[3:-1] == ['"CARGO_MANIFEST_DIR"'], tokens
        return str(manifest.parent)
    assert tokens[:3] == ["concat", "!", "("], f"Unsupported include expression in {manifest}: {tokens}"
    values = []
    index = 3
    while index < len(tokens) - 1:
        end = index + 1
        if end < len(tokens) and tokens[end] == "!":
            end = closing(tokens, end + 1) + 1
        values.append(string_value(tokens[index:end], manifest))
        index = end + 1
    return "".join(values)


def source_roots(manifest):
    data = document(manifest)
    base = manifest.parent
    candidates = [base / data.get("lib", {}).get("path", "src/lib.rs")]
    candidates.append(base / "src/main.rs")
    candidates.extend(base.glob("src/bin/*.rs"))
    candidates.extend(base.glob("src/bin/*/main.rs"))
    candidates.extend(base / target["path"] for target in data.get("bin", []) if "path" in target)
    build = data.get("package", {}).get("build", "build.rs")
    if isinstance(build, str):
        candidates.append(base / build)
    roots = {path.resolve() for path in candidates if path.is_file()}
    assert roots, f"No Rust targets in {manifest}"
    return roots


@lru_cache(maxsize=None)
def include_targets(manifest):
    targets = set()
    visited = set()

    def scan(path, module_dir=None):
        path = path.resolve()
        if path in visited:
            return
        assert path.is_file(), path
        visited.add(path)
        tokens = production_tokens(path.read_text())
        directory = module_dir or (path.parent if path.name in ("lib.rs", "main.rs", "mod.rs", "build.rs") else path.with_suffix(""))

        def walk(start, end, directory):
            index = start
            explicit_path = None
            while index < end:
                token = tokens[index]
                if token == "#" and tokens[index + 1] == "[":
                    attr_end = closing(tokens, index + 1)
                    attr = tokens[index + 2:attr_end]
                    if attr[:2] == ["path", "="]:
                        explicit_path = string_value(attr[2:], manifest)
                if token in ("include_str", "include_bytes") and tokens[index + 1:index + 3] == ["!", "("]:
                    last = closing(tokens, index + 2)
                    target = (path.parent / string_value(tokens[index + 3:last], manifest)).resolve()
                    assert target.is_relative_to(ROOT), f"Include escapes repository: {path}: {target}"
                    assert target.is_file(), f"Missing {token} target: {path}: {target}"
                    targets.add((path.relative_to(ROOT).as_posix(), target.relative_to(ROOT).as_posix()))
                    index = last + 1
                    continue
                if token == "mod" and index + 2 < end:
                    name, delimiter = tokens[index + 1:index + 3]
                    if delimiter == ";":
                        candidates = [path.parent / explicit_path] if explicit_path else [directory / (name + ".rs"), directory / name / "mod.rs"]
                        existing = [candidate for candidate in candidates if candidate.is_file()]
                        assert len(existing) == 1, f"Unresolved module: {path}: {name}: {candidates}"
                        scan(existing[0])
                        explicit_path = None
                    elif delimiter == "{":
                        last = closing(tokens, index + 2)
                        nested_directory = directory / (
                            explicit_path if explicit_path is not None else name
                        )
                        explicit_path = None
                        walk(index + 3, last, nested_directory)
                        index = last + 1
                        continue
                index += 1

        walk(0, len(tokens), directory)

    for source in source_roots(manifest):
        scan(source)
    return frozenset(targets)


def image_inputs(roots):
    manifests = set().union(*(closure(root) for root in roots))
    return manifests, set().union(*(include_targets(manifest) for manifest in manifests))


def test_layerx_image_includes_reachable():
    builds = [build for build in image_builds() if build[0] == LAYERX and build[1] == "x-websearch"]
    assert len(builds) == 1
    _, _, roots, policy = builds[0]
    manifests, inputs = image_inputs(roots)
    assert {"layerx-crypto", "layerx-programs-runtime", "layerx-proof"} <= {
        document(manifest)["package"]["name"] for manifest in manifests
    }
    targets = {target for _, target in inputs}
    assert MARKDOWN | {SETTLEMENT} <= targets
    for source, target in sorted(inputs):
        assert admitted(source, policy), f"LayerX source excluded: {source}"
        assert admitted(target, policy), f"LayerX include excluded: {source} -> {target}"
    for target in MARKDOWN:
        assert not admitted(str(Path(target).with_name("unrelated.md")), policy)
    native = False
    copied = False
    for instruction, arguments in instructions(LAYERX):
        if instruction == "FROM":
            native = arguments.endswith(" AS native")
        elif instruction == "COPY" and native:
            options, sources, destination = copy_arguments(arguments)
            if SETTLEMENT in sources:
                assert "from" not in options
                assert destination.rstrip("/") == "contracts/config"
                copied = True
    assert copied, "Native builder lacks its settlement compile input"


def test_rust_image_include_targets_not_ignored():
    checked = set()
    failures = []
    for dockerfile, stage, roots, policy in image_builds():
        manifests, inputs = image_inputs(roots)
        assert manifests, f"Empty dependency closure: {dockerfile}: {stage}"
        for source, target in sorted(inputs):
            checked.add((dockerfile, stage, source, target))
            if not admitted(target, policy):
                failures.append(f"{dockerfile.relative_to(ROOT)}:{stage}: {source} -> {target}")
    assert checked, "No production include targets checked"
    assert not failures, "Image include targets excluded:\n" + "\n".join(failures)


def test_checkpoint_settlement_absent_from_runtime_layers():
    stages = {}
    current = None
    for instruction, arguments in instructions(LAYERX):
        if instruction == "FROM":
            words = arguments.split()
            current = words[-1]
            stages[current] = (words[0], [])
        elif current:
            stages[current][1].append((instruction, arguments))
    assert {"runtime", "release", "source"} <= stages.keys()
    allowed_sources = {
        "release/linux/${TARGETARCH}/paxd", "release/linux/${TARGETARCH}/*.so",
        "/src/.x-websearch-target/release/x-websearch", "/usr/bin/paxctl",
        "/go/bin/paxd", "/go/lib/*.so",
    }
    for name in ("runtime", "release", "source"):
        parent, commands = stages[name]
        assert parent == "runtime" if name != "runtime" else parent.startswith("docker.io/ubuntu:")
        for instruction, arguments in commands:
            assert SETTLEMENT not in arguments
            assert instruction != "ADD", f"Unreviewed runtime input: {name}: {arguments}"
            if instruction == "COPY":
                options, sources, destination = copy_arguments(arguments)
                assert set(sources) <= allowed_sources, f"Runtime copies source/configuration: {name}: {sources}"
                assert destination in ("/usr/bin/", "/usr/bin/paxd", "/usr/bin/x-websearch", "/usr/lib/")
                if "from" in options:
                    assert options["from"] in ("builder", "x-websearch", "paxctl")
    assert any(instruction == "COPY" for instruction, _ in stages["release"][1])
    assert any(instruction == "COPY" for instruction, _ in stages["source"][1])
