#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"

python3 - "$root" <<'PY'
import contextlib
import fnmatch
import io
import json
import os
from pathlib import Path
import re
import shlex
import sys
import tempfile

ROOT = Path(sys.argv[1])
LAUNCHPAD = ROOT / "apps/launchpad"
WEB = LAUNCHPAD / "web"
DOCKERFILE = ROOT / "deploy/launchpad/Dockerfile.web"
IGNORE_FILE = DOCKERFILE.with_name("Dockerfile.web.dockerignore")

# Next filters these files in next/dist/lib/typescript/runTypeCheck.js.
NEXT_DIAGNOSTIC_EXCLUSION = re.compile(
    r"[\\/]__(?:tests|mocks)__[\\/]|(?<=[\\/.])(?:spec|test)\.[^\\/]+$"
)
TOKEN = re.compile(
    r"//[^\n]*|/\*[\s\S]*?\*/|"
    r"'(?:\\[\s\S]|[^'\\])*'|\"(?:\\[\s\S]|[^\"\\])*\"|"
    r"`(?:\\[\s\S]|[^`\\])*`|[\w$]+|[^\s]"
)


def fail(message):
    raise SystemExit(f"FAIL: {message}")


def ignore_rules():
    rules = []
    for line in IGNORE_FILE.read_text().splitlines():
        if line.startswith("#") or not line.strip():
            continue
        pattern = line.strip()
        include = pattern.startswith("!")
        pattern = os.path.normpath(pattern[1:] if include else pattern).strip("/")
        if pattern == ".":
            continue
        parts = pattern.split("/")
        expression = ""
        for index, part in enumerate(parts):
            last = index == len(parts) - 1
            if part == "**":
                expression += ".*" if last else "(?:[^/]+/)*"
            else:
                expression += fnmatch.translate(part)[4:-3]
                if not last:
                    expression += "/"
        rules.append((re.compile(expression), include))
    return rules


IGNORE_RULES = ignore_rules()


def context_excludes(path):
    relative = path.relative_to(ROOT)
    ancestors = [relative.as_posix()]
    ancestors.extend(parent.as_posix() for parent in relative.parents if parent != Path("."))
    excluded = False
    for pattern, include in IGNORE_RULES:
        if any(pattern.fullmatch(name) for name in ancestors):
            excluded = not include
    return excluded


def web_sources():
    sources = []
    for directory, dirs, files in os.walk(WEB):
        dirs[:] = sorted(
            name for name in dirs
            if name not in {"node_modules", ".next", ".next-build", "dist", "out"}
        )
        for name in sorted(files):
            path = Path(directory) / name
            if (
                path.suffix in {".ts", ".tsx"}
                and not context_excludes(path)
            ):
                sources.append(path)
    return sources


def literal(token):
    if len(token) < 2 or token[0] not in {"'", '"', "`"}:
        return None
    if token[0] == "`" and "${" in token:
        return None
    return re.sub(r"\\(['\"`\\])", r"\1", token[1:-1])


def relative_specifiers(path):
    tokens = [token for token in TOKEN.findall(path.read_text()) if not token.startswith(("//", "/*"))]
    for index, token in enumerate(tokens):
        if token not in {"import", "export"} or index + 1 == len(tokens):
            continue
        following = tokens[index + 1]
        value = None
        if token == "import" and literal(following) is not None:
            value = literal(following)
        elif token == "import" and following == "(" and index + 3 < len(tokens):
            if tokens[index + 3] in {")", ","}:
                value = literal(tokens[index + 2])
        elif following != "." and (token == "import" or following in {"*", "{", "type"}):
            for cursor in range(index + 1, len(tokens) - 1):
                if tokens[cursor] in {";", "import", "export"}:
                    break
                if tokens[cursor] == "from":
                    value = literal(tokens[cursor + 1])
                    if value is not None:
                        break
        if value is not None and value.startswith(("./", "../")):
            yield value


def resolve_import(source, specifier):
    base = Path(os.path.normpath(source.parent / specifier))
    if base.suffix in {".ts", ".tsx", ".json"}:
        candidates = [base]
    else:
        stem = base.with_suffix("") if base.suffix in {".js", ".jsx"} else base
        candidates = [Path(str(stem) + suffix) for suffix in (".ts", ".tsx", ".d.ts")]
        candidates.extend([base, Path(str(base) + ".json")])
        candidates.extend(base / ("index" + suffix) for suffix in (".ts", ".tsx", ".d.ts", ".json"))
    return next((candidate for candidate in candidates if candidate.is_file()), base)


def build_copies(dockerfile):
    copies = []
    in_build = False
    for line in dockerfile.read_text().replace("\\\n", " ").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        instruction, _, arguments = line.strip().partition(" ")
        if instruction.upper() == "FROM":
            parts = shlex.split(arguments)
            in_build = len(parts) >= 3 and parts[-2].upper() == "AS" and parts[-1].lower() == "build"
        elif in_build and instruction.upper() == "COPY":
            if arguments.startswith("--from="):
                continue
            while arguments.startswith("--"):
                _, _, arguments = arguments.partition(" ")
                arguments = arguments.lstrip()
            parts = json.loads(arguments) if arguments.startswith("[") else shlex.split(arguments)
            copies.extend(ROOT / source.strip("/") for source in parts[:-1])
    return copies


def check_copies(dockerfile, sources):
    copies = build_copies(dockerfile)
    missing = set()
    for source in sources:
        for specifier in relative_specifiers(source):
            target = resolve_import(source, specifier)
            if target.is_relative_to(LAUNCHPAD):
                continue
            if (
                not target.is_relative_to(ROOT)
                or not target.is_file()
                or context_excludes(target)
                or not any(target == copy or (copy.is_dir() and copy in target.parents) for copy in copies)
            ):
                missing.add((source.relative_to(ROOT).as_posix(), specifier))
    for source, specifier in sorted(missing):
        print(f"{source}: {specifier}")
    return 1 if missing else 0


def test_case_files_are_excluded(context_sources, sources):
    recovery = WEB / "src/widgets/create-chain/chain-launch-recovery.test.ts"
    vectors = "../../../../../../tests/vectors/chains/abi-vectors.json"
    blocks = "../../../../../../services/platform/hosted/indexer/tests/fixtures/chains_blocks.json"
    if not NEXT_DIAGNOSTIC_EXCLUSION.search(recovery.as_posix()) or recovery in sources:
        fail("chain-launch-recovery.test.ts must follow Next's diagnostic exclusion")
    if not {vectors, blocks}.issubset(set(relative_specifiers(recovery))):
        fail("chain-launch-recovery.test.ts must exercise both external JSON imports")
    vector_cases = set()
    block_cases = set()
    for path in context_sources:
        if not NEXT_DIAGNOSTIC_EXCLUSION.search(path.as_posix()):
            continue
        specifiers = set(relative_specifiers(path))
        if vectors in specifiers:
            vector_cases.add(path)
        if blocks in specifiers:
            block_cases.add(path)
    if len(vector_cases) != 5 or block_cases != {recovery}:
        fail("external JSON case inventory changed")
    if (vector_cases | block_cases).intersection(sources):
        fail("Next's excluded JSON case files must be absent from the checked sources")
    print("ok: chain-launch-recovery.test.ts and external JSON cases follow Next's diagnostic exclusion")


def test_wallet_copy_is_required(sources):
    wallet = "services/wallet/gateway/src/auth/did.ts"
    copy_line = f"COPY {wallet} /src/{wallet}"
    lines = DOCKERFILE.read_text().splitlines(keepends=True)
    if sum(line.rstrip("\n") == copy_line for line in lines) != 1:
        fail("wallet DID source must have exactly one explicit COPY")
    fixture = "apps/launchpad/web/src/widgets/agents/agents-gateway.fixture.ts"
    specifier = "../../../../../../services/wallet/gateway/src/auth/did"
    if specifier not in (ROOT / fixture).read_text().splitlines()[10]:
        fail("agents gateway fixture must import the wallet DID source at line 11")
    with tempfile.TemporaryDirectory() as directory:
        dockerfile = Path(directory) / "Dockerfile.web"
        dockerfile.write_text("".join(line for line in lines if line.rstrip("\n") != copy_line))
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            status = check_copies(dockerfile, sources)
        if status != 1 or output.getvalue() != f"{fixture}: {specifier}\n":
            fail(f"removing the wallet COPY must report only its fixture import, got {status}: {output.getvalue()}")
    print("ok: removing the wallet COPY reports only the agents gateway fixture import")


context_sources = web_sources()
sources = [path for path in context_sources if not NEXT_DIAGNOSTIC_EXCLUSION.search(path.as_posix())]
if not sources:
    fail("web source inventory is empty")
if check_copies(DOCKERFILE, sources):
    sys.exit(1)
print("ok: web relative imports outside apps/launchpad are copied into the build stage")
test_case_files_are_excluded(context_sources, sources)
test_wallet_copy_is_required(sources)
PY
