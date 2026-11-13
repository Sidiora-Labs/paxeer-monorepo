"""Resolve documented shell commands against this checkout without running them.

Decision 2809 supplies the command inventory contract. Following decision 2943
and the Jev command page's refusal output (gotcha 3650), a dollar-prompt block
contains commands only at its prompts, including continuations and heredocs.
Following the public-node endpoint producers in decisions 2810 and 2944,
JSON-RPC and WebSocket upgrades use only the root of the alias and api1–api14;
the retired /rpc/ws route has no upgrade.
"""

from dataclasses import dataclass
from fnmatch import fnmatchcase
from functools import lru_cache
import glob
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import textwrap
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[2]
DATA_PATH = ROOT / "tests/ci/data/docs_command_roots.json"
SHELL_LANGUAGES = {"", "sh", "bash", "shell", "console", "zsh"}
VENDORED = (
    "tools/hyperfide/", "programs/vendor/", "tools/uci/", "core/deps/",
    "tools/wasm-tools/",
)
FENCE = re.compile(
    r"^ {0,3}(?P<ticks>`{3,}|~{3,})(?P<language>[^\n]*)\n"
    r"(?P<body>.*?)^ {0,3}(?P=ticks)[ \t]*$", re.MULTILINE | re.DOTALL,
)
SHELL_START = re.compile(
    r"^(?:[A-Za-z_][A-Za-z_0-9]*=|[./~$]|\(|"
    r"(?:make|gmake|npm|pnpm|yarn|cargo|go|cd|git|bash|sh|zsh|python3?|node|"
    r"source|export|env|sudo|timeout|curl|wget|mkdir|cp|mv|rm|echo|printf|cat|"
    r"chmod|pip3?|uv|docker|kubectl|helm|corepack|rustup|npx|pnpx|eval|"
    r"hyperfide|cg|paxd|hpx|seictl|flyctl|read|set|test|if|for|while|done|fi)\b)"
)
PUBLIC_HOST = re.compile(
    r"(?:api-mainnet-beta\.paxeer\.network|api(?:[1-9]|1[0-4])"
    r"\.mainnet-beta\.paxeer\.network)\Z"
)
URL = re.compile(r"(?:https?|wss?)://[^\s`\"'<>\[\]()]+")
ASSIGNMENT = re.compile(r"^[A-Za-z_][A-Za-z_0-9]*=")


@lru_cache(maxsize=1)
def _data():
    return json.loads(DATA_PATH.read_text(encoding="utf-8"))


@lru_cache(maxsize=1)
def _tracked():
    return tuple(subprocess.run(
        ["git", "ls-files", "-z"], cwd=ROOT, check=True,
        capture_output=True, text=True,
    ).stdout.rstrip("\0").split("\0"))


@lru_cache(maxsize=1)
def _pages():
    return tuple(ROOT / name for name in _tracked() if (
        Path(name).name == "README.md" or
        name.startswith("docs/") and name.endswith(".md")
    ) and not name.startswith(VENDORED) and not (
        {"node_modules", "vendor"} & set(Path(name).parts)
    ) and not name.startswith("spec/"))


def _shell_blocks(text):
    for match in FENCE.finditer(text):
        language = match["language"].strip().lower()
        if language in SHELL_LANGUAGES:
            yield text.count("\n", 0, match.start()) + 2, language, textwrap.dedent(match["body"])


def _logical_commands(body, language="sh", first_line=1):
    """Keep output out of prompt-led blocks before attempting shell tokenization."""
    lines = body.splitlines()
    prompt_led = any(line.lstrip().startswith("$ ") for line in lines)
    index = 0
    while index < len(lines):
        number = first_line + index
        line = lines[index].strip()
        index += 1
        prompted = line.startswith("$ ")
        if prompt_led and not prompted:
            continue
        if prompted:
            line = line[2:]
        if not line or line.startswith("#"):
            continue
        if not prompt_led and language in {"", "console"} and not SHELL_START.match(line):
            continue
        if not prompt_led and language in {"", "console"}:
            # Untagged source/documentation maps are output, including rows
            # such as '../ARCHITECTURE.md how the pieces fit together'.
            first = line.split()[0]
            if first.endswith("/") or Path(first).suffix in {".md", ".go", ".rs", ".c", ".h"}:
                continue
        command = line
        while True:
            continued = command.endswith("\\")
            if continued:
                command = command[:-1].rstrip() + " "
            # A heredoc's body is data even when it contains shell commands/prompts.
            heredocs = re.findall(r"<<(-?)[ \t]*['\"]?([A-Za-z_][A-Za-z_0-9]*)['\"]?", command)
            try:
                shlex.split(command, comments=True)
                complete = True
            except ValueError:
                complete = False
            if not continued and complete:
                break
            assert index < len(lines), f"incomplete shell command at line {number}: {command}"
            command += ("" if continued else "\n") + lines[index].strip()
            index += 1
        for strip_tabs, delimiter in heredocs:
            while index < len(lines) and (
                lines[index].lstrip("\t") if strip_tabs else lines[index]
            ) != delimiter:
                index += 1
            assert index < len(lines), f"unclosed heredoc {delimiter} at line {number}"
            index += 1
        yield number, command


def _tokens(command):
    # Documentation placeholders are arguments, not shell input redirections.
    command = re.sub(r"<[^<>\n]+>", lambda m: shlex.quote(m[0]), command)
    # Command substitutions are opaque argument values; never execute them.
    command = re.sub(r"\$\((?:[^()]|\([^()]*\))*\)", "DOC_SUBSTITUTION", command)
    lexer = shlex.shlex(command, posix=True, punctuation_chars=";&|()<>")
    lexer.whitespace_split = True
    return list(lexer)


@dataclass(frozen=True)
class Command:
    page: Path
    line: int
    cwd: Path
    words: tuple[str, ...]
    text: str

    @property
    def location(self):
        return f"{self.page.relative_to(ROOT)}:{self.line}"


def _page_root(page, preceding=""):
    name = page.relative_to(ROOT).as_posix()
    entries = _data()["page_roots"]
    prefix = max((p for p in entries if name.startswith(p)), key=len)
    directory = ROOT / entries[prefix]
    # Only the prose since the previous fence supplies a block-local override.
    # A path used in an earlier example is not a working-directory directive.
    previous_fences = list(FENCE.finditer(preceding))
    if previous_fences:
        preceding = preceding[previous_fences[-1].end():]
    sentences = list(re.finditer(
        r"(?:^|[\n.!?,]\s*)(?:Run|run|Start|start)[^\n.]{0,120}?from\s+"
        r"(?:the\s+)?(?:directory\s+)?`([^`]+)`", preceding,
    ))
    if sentences:
        reference = sentences[-1][1].removeprefix("paxeer-monorepo/")
        directory = ROOT / reference
    elif re.search(r"(?:From|from) the (?:monorepo|repository|repo) root\b", preceding):
        directory = ROOT / entries[prefix] if name.startswith("docs/hyperfide/") else ROOT
    local = re.search(r"(?:^|[\n.!?]\s*)(?:From|from)\s+`([^`]+)`[^\n]*:\s*$", preceding)
    if local:
        directory = ROOT / local[1]
    if re.search(r"From this SDK directory[^\n]*:\s*$", preceding):
        directory = page.parent
    if re.search(r"(?:^|[\n.!?]\s*)(?:From|from|Run it from|run from) this directory[^\n]*:\s*$", preceding):
        directory = page.parent
    if "(run from this directory)" in preceding:
        directory = page.parent
    return directory.resolve()


def _path(reference, cwd):
    reference = reference.replace("${PWD}", str(cwd)).replace("$PWD", str(cwd))
    if reference == "paxeer-monorepo":
        return ROOT
    if reference.startswith("paxeer-monorepo/"):
        return ROOT / reference.removeprefix("paxeer-monorepo/")
    if reference.startswith("/path/to/paxeer-monorepo/"):
        return ROOT / reference.removeprefix("/path/to/paxeer-monorepo/")
    return Path(reference) if reference.startswith("/") else cwd / reference


def _allowed(reference, cwd):
    if any(marker in reference for marker in ("$", "<", "DOC_SUBSTITUTION")):
        # Environment values and angle-bracket inputs are supplied by the reader.
        return True
    path = _path(reference, cwd)
    try:
        name = path.resolve().relative_to(ROOT).as_posix()
    except ValueError:
        name = str(path)
    return any(fnmatchcase(name, item["pattern"]) for item in _data()["allowed_inputs_and_outputs"])


def _exists(reference, cwd, directory=False):
    path = _path(reference, cwd)
    if directory and path.is_dir() or not directory and path.is_file():
        return True
    if any(character in reference for character in "*?["):
        matches = [Path(name) for name in glob.glob(str(path), recursive=True)]
        if any(p.is_dir() if directory else p.is_file() for p in matches):
            return True
    return _allowed(reference, cwd)


def _segments(words):
    segment = []
    for word in words:
        if word in {";", "&&", "||", "|", "&", "(", ")"}:
            if segment:
                yield segment
                segment = []
            if word in {"(", ")"}:
                yield [word]
        else:
            segment.append(word)
    if segment:
        yield segment


def _unwrap(words):
    words = list(words)
    while words:
        if ASSIGNMENT.match(words[0]) or words[0] in {"then", "do", "!"}:
            words.pop(0)
        elif words[0] in {"sudo", "env", "timeout", "command", "nohup"}:
            wrapper = words.pop(0)
            while words and (words[0].startswith("-") or ASSIGNMENT.match(words[0])):
                option = words.pop(0)
                if option in {"-u", "--unset", "-S", "--signal", "-k", "--kill-after"} and words:
                    words.pop(0)
            if wrapper == "timeout" and words:
                words.pop(0)
        else:
            break
    return words


@lru_cache(maxsize=1)
def _commands():
    commands = []
    for page in _pages():
        text = page.read_text(encoding="utf-8")
        previous_cwd = None
        for first, language, body in _shell_blocks(text):
            preceding = "\n".join(text.splitlines()[:first - 2])
            cwd = _page_root(page, preceding)
            if (page.relative_to(ROOT).as_posix() in _data()["carry_cwd_pages"]
                    and previous_cwd is not None and cwd == _page_root(page)):
                cwd = previous_cwd
            # Entering a mapped project by its monorepo path starts at the
            # monorepo root; following blocks keep their mapped project root.
            first_cd = re.match(r"\s*(?:\$ )?\(?cd ([^\s;&|]+)", body)
            if first_cd and (ROOT / first_cd[1]).is_dir() and first_cd[1].split("/")[0] in {
                "tools", "apps", "services", "programs", "contracts", "tests", "wasm",
            }:
                cwd = ROOT
            if re.search(r"(?im)^\s*#\s*From the repository root\b", body):
                cwd = ROOT
            stack = []
            for number, line in _logical_commands(body, language, first):
                for words in _segments(_tokens(line)):
                    if words == ["("]:
                        stack.append(cwd)
                        continue
                    if words == [")"]:
                        if stack:
                            cwd = stack.pop()
                        continue
                    words = _unwrap(words)
                    if not words:
                        continue
                    command = Command(page, number, cwd, tuple(words), line)
                    commands.append(command)
                    if words[0] == "cd" and len(words) > 1:
                        cwd = _path(words[1], cwd).resolve()
            previous_cwd = cwd
    return tuple(commands)


def _options(words, directory, names):
    """Remove options with values while collecting repeated selections."""
    result = []
    values = {name: [] for name in names}
    index = 0
    while index < len(words):
        word = words[index]
        key, equals, value = word.partition("=")
        if key in names:
            if not equals:
                index += 1
                assert index < len(words), f"{word} requires a value"
                value = words[index]
            values[key].append(value)
        else:
            result.append(word)
        index += 1
    for key in ("-C", "--directory", "--dir", "--cwd", "--prefix"):
        for value in values.get(key, []):
            directory = _path(value, directory).resolve()
    return result, directory, values


def _expand_make(text, variables):
    for _ in range(12):
        expanded = re.sub(r"\$\(([^()]+)\)|\$\{([^{}]+)\}",
                          lambda m: variables.get(m[1] or m[2], m[0]), text)
        if expanded == text:
            break
        text = expanded
    return text


@lru_cache(maxsize=None)
def _make_targets(makefile):
    assert makefile.is_file(), f"Missing Makefile: {makefile}"
    variables = {"CURDIR": str(makefile.parent), "PAXEER_DIR": str(ROOT)}
    targets = set()
    visited = set()

    def read(path):
        if path in visited:
            return
        assert path.is_file(), f"Missing make include: {path}"
        visited.add(path)
        content = path.read_text(encoding="utf-8").replace("\\\n", " ")
        lines = content.splitlines()
        for line in lines:
            if line.startswith("\t"):
                continue
            match = re.match(r"\s*([A-Za-z_][A-Za-z_0-9]*)\s*([:+?]?=)\s*(.*?)\s*(?:#.*)?$", line)
            if match:
                key, operator, value = match.groups()
                if operator == "+=":
                    variables[key] = variables.get(key, "") + " " + value
                elif operator != "?=" or key not in variables:
                    variables[key] = value
        for line in lines:
            if line.startswith("\t"):
                continue
            include = re.match(r"\s*(-?include|sinclude)\s+(.+)", line)
            if include:
                for name in _expand_make(include[2], variables).split():
                    if "$" in name:
                        continue  # Conditional, computed compiler dependency includes.
                    files = glob.glob(str(_path(name, makefile.parent)))
                    if include[1] == "include":
                        assert files, f"Missing make include {name} in {path}"
                    for filename in files:
                        read(Path(filename))
            rule = re.match(r"([^\s#][^:=]*):(?![=])", line)
            if rule:
                targets.update(_expand_make(rule[1], variables).split())
        if path == ROOT / "Makefile":
            forwarded = _expand_make(variables["PAXEER_FORWARDED_TARGETS"], variables).split()
            chain = _make_targets(ROOT / "chain.mk")
            for target in forwarded:
                assert _target_exists(target, chain), f"Forwarded target {target} missing from chain.mk"
            targets.update(forwarded)

    read(makefile)
    return frozenset(targets)


def _target_exists(target, declared):
    return any(re.fullmatch(re.escape(pattern).replace("%", ".*"), target) for pattern in declared)


def test_doc_make_targets_exist():
    checked = 0
    failures = []
    for command in _commands():
        if command.words[0] not in {"make", "gmake"}:
            continue
        words, directory, options = _options(command.words[1:], command.cwd,
            {"-C", "--directory", "-f", "--file", "--makefile", "-j", "--jobs"})
        files = options["-f"] + options["--file"] + options["--makefile"]
        if not files:
            files = [next((name for name in ("GNUmakefile", "makefile", "Makefile")
                           if (directory / name).is_file()), "Makefile")]
        targets = set()
        for name in files:
            targets.update(_make_targets(_path(name, directory)))
        for target in words:
            if target.startswith("-") or "=" in target or target.isdigit():
                continue
            checked += 1
            if not _target_exists(target, targets):
                failures.append(f"{command.location}: make {target} from {directory}")
    assert checked > 0, "No documented make targets checked"
    assert not failures, "Missing make targets:\n" + "\n".join(failures)


def _script_references(words):
    for index, executable in enumerate(words):
        if index != 0 and words[index - 1] != "--":
            continue
        if executable in {"bash", "sh", "zsh", "source", ".", "python", "python3", "node"}:
            args = list(words[index + 1:])
            while args and args[0].startswith("-"):
                option = args.pop(0)
                if option in {"-c", "-e", "--eval", "-p", "--print", "-m"}:
                    args = []
                    break
                if option in {"--import", "--require", "-r", "--loader"} and args:
                    args.pop(0)
            if args and args[0] not in {"-", "<", "<<", "<<-", ">", "2", "&"}:
                yield args[0]
        elif index == 0 and executable.startswith(("./", "../")):
            yield executable


def test_doc_script_paths_exist():
    checked = 0
    failures = []
    for command in _commands():
        for reference in _script_references(command.words):
            checked += 1
            if not _exists(reference, command.cwd):
                failures.append(f"{command.location}: {reference} from {command.cwd}")
    assert checked > 0, "No documented script paths checked"
    assert not failures, "Missing script paths:\n" + "\n".join(failures)


@lru_cache(maxsize=None)
def _json(path):
    return json.loads(path.read_text(encoding="utf-8"))


@lru_cache(maxsize=1)
def _package_manifests():
    return tuple(ROOT / name for name in _tracked() if name.endswith("/package.json")
                 and not {"node_modules", "vendor"} & set(Path(name).parts)) + (ROOT / "package.json",)


def _workspace_root(directory):
    for parent in (directory, *directory.parents):
        if (parent / "pnpm-workspace.yaml").is_file():
            return parent
        manifest = parent / "package.json"
        if manifest.is_file() and _json(manifest).get("workspaces"):
            return parent
        if parent == ROOT:
            break
    return directory


def _selected_packages(directory, selectors):
    workspace = _workspace_root(directory)
    workspace_file = workspace / "pnpm-workspace.yaml"
    if workspace_file.is_file():
        content = workspace_file.read_text(encoding="utf-8")
        packages = re.search(r"(?m)^packages:\s*\n((?:[ \t]+[^\n]*\n|\n)+)", content)
        assert packages, f"Missing package patterns in {workspace_file}"
        patterns = [m[1].strip().strip("\"'") for m in re.finditer(
            r"(?m)^\s+-\s+([^\n#]+)", packages[1])]
    else:
        package = workspace / "package.json"
        patterns = _json(package).get("workspaces", []) if package.is_file() else []
        if isinstance(patterns, dict):
            patterns = patterns["packages"]
    members = {workspace / "package.json"}
    for pattern in patterns:
        if not pattern.startswith("!"):
            members.update((Path(p) / "package.json").resolve()
                           for p in glob.glob(str(workspace / pattern)))
    for pattern in patterns:
        if pattern.startswith("!"):
            members.difference_update((Path(p) / "package.json").resolve()
                                      for p in glob.glob(str(workspace / pattern[1:])))
    selected = set()
    for selector in selectors:
        selector = selector.removeprefix("...").removesuffix("...").removesuffix("^")
        for manifest in _package_manifests():
            if manifest not in members:
                continue
            name = _json(manifest).get("name", "")
            if fnmatchcase(name, selector) or (
                selector.startswith(".") and fnmatchcase(
                    Path(os.path.relpath(manifest.parent, directory)).as_posix(),
                    selector.removeprefix("./"),
                )
            ):
                selected.add(manifest)
    assert selected, f"Package selectors {selectors} match no packages from {directory}"
    return selected


def test_doc_package_scripts_exist():
    checked = 0
    failures = []
    builtins = {
        "install", "i", "ci", "add", "remove", "uninstall", "update", "up", "exec", "dlx",
        "create", "init", "publish", "pack", "link", "unlink", "list", "ls", "info", "view",
        "audit", "config", "cache", "dedupe", "why", "outdated", "rebuild", "version",
    }
    for command in _commands():
        if command.words[0] not in {"pnpm", "npm", "yarn"}:
            continue
        words, directory, options = _options(command.words[1:], command.cwd,
            {"-C", "--dir", "--cwd", "--prefix", "--filter", "-F", "--workspace", "-w"})
        words = [word for word in words if not word.startswith("-")]
        if not words or words[0] in builtins:
            continue
        if words[0] in {"run", "run-script"}:
            words.pop(0)
        if not words:
            continue
        script = words[0]
        selectors = options["--filter"] + options["-F"] + options["--workspace"] + options["-w"]
        manifests = _selected_packages(directory, selectors) if selectors else {directory / "package.json"}
        for manifest in manifests:
            checked += 1
            if not manifest.is_file() or script not in _json(manifest).get("scripts", {}):
                failures.append(f"{command.location}: {script} missing from {manifest}")
    assert checked > 0, "No documented package scripts checked"
    assert not failures, "Undefined package scripts:\n" + "\n".join(failures)


@lru_cache(maxsize=None)
def _toml(path):
    return tomllib.loads(path.read_text(encoding="utf-8"))


def _cargo_manifests(manifest):
    assert manifest.is_file(), f"Missing Cargo manifest: {manifest}"
    data = _toml(manifest)
    workspace = data.get("workspace")
    if workspace is None:
        for parent in manifest.parent.parents:
            candidate = parent / "Cargo.toml"
            if candidate.is_file() and "workspace" in _toml(candidate):
                manifest = candidate
                workspace = _toml(candidate)["workspace"]
                break
            if parent == ROOT:
                break
    paths = {manifest}
    if workspace:
        for pattern in workspace.get("members", []):
            paths.update(path / "Cargo.toml" for path in manifest.parent.glob(pattern))
        excluded = {path / "Cargo.toml" for pattern in workspace.get("exclude", [])
                    for path in manifest.parent.glob(pattern)}
        paths -= excluded
    return {path: _toml(path) for path in paths if path.is_file() and "package" in _toml(path)}


def _cargo_bins(manifest, data):
    result = {item["name"] for item in data.get("bin", [])}
    if data.get("package", {}).get("autobins", True):
        if (manifest.parent / "src/main.rs").is_file():
            result.add(data["package"]["name"])
        for source in (manifest.parent / "src/bin").glob("*"):
            if source.suffix == ".rs" or source.is_dir() and (source / "main.rs").is_file():
                result.add(source.stem if source.is_file() else source.name)
    return result


def _reader_created_cargo_bins(page):
    result = set()
    for entry in _data()["allowed_inputs_and_outputs"]:
        if entry.get("page") != page.relative_to(ROOT).as_posix() or "cargo_bin" not in entry:
            continue
        content = page.read_text(encoding="utf-8")
        source = entry["source"]
        assert f"Create `{source}`" in content, f"Missing source creation instruction in {page}"
        assert Path(source).stem == entry["cargo_bin"]
        assert any(match["language"].strip() == "rust" and "fn main()" in match["body"]
                   for match in FENCE.finditer(content)), f"Missing user-created binary source in {page}"
        result.add(entry["cargo_bin"])
    return result


def test_doc_cargo_packages_exist():
    checked = 0
    failures = []
    for command in _commands():
        if "cargo" not in command.words:
            continue
        cargo_index = command.words.index("cargo")
        words = list(command.words[cargo_index + 1:])
        if "--" in words:
            words = words[:words.index("--")]
        _, directory, options = _options(words, command.cwd,
            {"-C", "--manifest-path", "--package", "-p", "--bin"})
        packages = options["--package"] + options["-p"]
        bins = options["--bin"]
        if not packages and not bins:
            continue
        if options["--manifest-path"]:
            manifest = _path(options["--manifest-path"][-1], directory)
        else:
            manifest = next((p / "Cargo.toml" for p in (directory, *directory.parents)
                             if (p / "Cargo.toml").is_file()), directory / "Cargo.toml")
        manifests = _cargo_manifests(manifest)
        selected = {p: data for p, data in manifests.items() if not packages or any(
            fnmatchcase(data["package"]["name"], package) for package in packages)}
        for package in packages:
            checked += 1
            if not any(fnmatchcase(data["package"]["name"], package) for data in selected.values()):
                failures.append(f"{command.location}: unknown Cargo package {package} in {manifest}")
        for binary in bins:
            checked += 1
            declared_bins = _reader_created_cargo_bins(command.page)
            for path, data in selected.items():
                declared_bins.update(_cargo_bins(path, data))
            if binary not in declared_bins:
                failures.append(f"{command.location}: unknown Cargo binary {binary} in {manifest}")
    assert checked > 0, "No documented Cargo packages or binaries checked"
    assert not failures, "Missing Cargo selections:\n" + "\n".join(failures)


def test_doc_go_paths_exist():
    checked = 0
    failures = []
    for command in _commands():
        if command.words[0] != "go":
            continue
        words, directory, _ = _options(command.words[1:], command.cwd, {"-C"})
        if not words or words[0] not in {"run", "test", "build", "vet", "list", "install"}:
            continue
        # Only Go's package list precedes a run program's flags and arguments.
        options_with_value = {"-o", "-p", "-run", "-skip", "-tags", "-count", "-timeout",
                              "-ldflags", "-gcflags", "-modfile", "-coverprofile", "-fuzz"}
        index = 1
        while index < len(words):
            word = words[index]
            if word in options_with_value:
                index += 2
                continue
            index += 1
            if word.startswith("-"):
                continue
            if word.startswith("."):
                checked += 1
                path = word.removesuffix("/...")
                exists = _exists(path, directory, directory=not path.endswith(".go"))
                if not exists:
                    failures.append(f"{command.location}: go {words[0]} {word} from {directory}")
            if words[0] == "run":
                break
    assert checked > 0, "No documented Go paths checked"
    assert not failures, "Missing Go paths:\n" + "\n".join(failures)


def test_doc_cd_targets_exist():
    checked = 0
    failures = []
    for command in _commands():
        if command.words[0] == "cd" and len(command.words) > 1:
            checked += 1
            reference = command.words[1]
            if reference != "-" and not _exists(reference, command.cwd, directory=True):
                failures.append(f"{command.location}: cd {reference} from {command.cwd}")
    assert checked > 0, "No documented cd targets checked"
    assert not failures, "Missing cd directories:\n" + "\n".join(failures)


def test_doc_clone_urls_name_this_repository():
    checked = 0
    failures = []
    for command in _commands():
        if command.words[:2] != ("git", "clone"):
            continue
        for word in command.words[2:]:
            if ("://" in word or word.startswith("git@")) and not any(
                marker in word for marker in ("<", "$", "DOC_SUBSTITUTION")
            ):
                checked += 1
                repository = word.rstrip("/").rsplit("/", 1)[-1].removesuffix(".git")
                if repository != "paxeer-monorepo":
                    failures.append(f"{command.location}: {word}")
    assert checked > 0, "No documented repository clone URLs checked"
    assert not failures, "Incorrect repository clone URLs:\n" + "\n".join(failures)


def test_docs_command_roots_map_entries_exist():
    names = {p.relative_to(ROOT).as_posix() for p in _pages()}
    assert "README.md" in names
    assert any(name.startswith("docs/hyperfide/") for name in names)
    assert not any(name.startswith(VENDORED) for name in names)
    for prefix, directory in _data()["page_roots"].items():
        assert any(name.startswith(prefix) for name in names), f"Unused page prefix {prefix}"
        assert (ROOT / directory).is_dir(), f"Missing mapped project root {directory}"
    for entry in _data()["allowed_inputs_and_outputs"]:
        assert entry["pattern"] and entry["reason"], "Every exception needs an explicit reason"
        if "producer" in entry:
            assert (ROOT / entry["producer"]).is_file(), f"Missing output producer {entry['producer']}"
    for page in _data()["carry_cwd_pages"]:
        assert page in names, f"Missing page carrying its working directory: {page}"


def _public_url_error(url):
    parsed = urlsplit(url)
    host = parsed.hostname or ""
    if host == "api-mainnet-beta.paxeer.network" or re.fullmatch(r"api\d+\.mainnet-beta\.paxeer\.network", host):
        if not PUBLIC_HOST.fullmatch(host) or parsed.path not in {"", "/"}:
            return f"public endpoint must use api1–api14 or the alias at the root: {url}"
    return None


def test_doc_public_rpc_hosts_use_root_path():
    checked = 0
    failures = []
    for page in _pages():
        for number, line in enumerate(page.read_text(encoding="utf-8").splitlines(), 1):
            for match in URL.finditer(line):
                for endpoint in match[0].split(","):
                    url = endpoint.rstrip(".,;)")
                    if PUBLIC_HOST.fullmatch(urlsplit(url).hostname or ""):
                        checked += 1
                    error = _public_url_error(url)
                    if error:
                        failures.append(f"{page.relative_to(ROOT)}:{number}: {error}")
    assert checked > 0, "No public RPC endpoint URLs checked"
    assert not failures, "Invalid public RPC endpoints:\n" + "\n".join(failures)
    for host in ["api-mainnet-beta.paxeer.network", *(
        f"api{number}.mainnet-beta.paxeer.network" for number in range(1, 15)
    )]:
        for scheme in ("https", "wss"):
            assert _public_url_error(f"{scheme}://{host}") is None
            assert _public_url_error(f"{scheme}://{host}/") is None
            for path in ("/rpc", "/rpc/ws", "/ws"):
                assert _public_url_error(f"{scheme}://{host}{path}") is not None
    assert _public_url_error("https://api15.mainnet-beta.paxeer.network") is not None


def test_shell_command_extraction():
    # Jev's prompt-led refusal at line 89: output beginning with a shell builtin
    # and an apostrophe must not become a command or reach shlex.
    body = "$ jev export --format json\nexport refused: this isn't an enrolled workspace\n"
    commands = list(_logical_commands(body))
    assert commands == [(1, "jev export --format json")]
    assert len(commands) == 1
    jev = ROOT / "docs/hyperfide/jev.md"
    refusal_blocks = [body for _, _, body in _shell_blocks(jev.read_text(encoding="utf-8"))
                      if "$ hyperfide jev ask --state hello" in body]
    assert len(refusal_blocks) == 1
    assert list(_logical_commands(refusal_blocks[0])) == [
        (1, 'hyperfide jev ask --state hello --noul greeting "Is it a greeting?"'),
    ]
    for language in SHELL_LANGUAGES:
        assert list(_logical_commands(body, language)) == commands
    assert list(_logical_commands("output isn't shell\n$ make build\nexport can't run\n")) == [(2, "make build")]
    continued = "$ python3 \\\n  tools/check.py\nexport isn't a command\n$ cat <<'EOF'\ncd missing\n$ make missing\nEOF\nignored isn't shell\n$ make test\n"
    assert list(_logical_commands(continued)) == [
        (1, "python3 tools/check.py"), (4, "cat <<'EOF'"), (9, "make test"),
    ]
    assert list(_logical_commands("export VALUE=one\nmake build\n")) == [
        (1, "export VALUE=one"), (2, "make build"),
    ]
    assert list(_logical_commands("plain output\nmake build\n", "console")) == [(2, "make build")]
    assert list(_logical_commands("../ARCHITECTURE.md how it fits\n./check.sh\n", "")) == [(2, "./check.sh")]
    assert list(_script_references(("seictl", "node", "watch"))) == []
    assert _page_root(ROOT / "apps/explorer/README.md", "To build from source, use `.nvmrc`:\n") == ROOT / "apps/explorer"
    assert _page_root(ROOT / "sdk/README.md", "The targets also run from `sdk/`:\n") == ROOT
    assert _page_root(ROOT / "deploy/launchpad/production/topology/README.md",
                      "Commands, from the monorepo root (not from this directory):\n") == ROOT
    assert _page_root(ROOT / "docs/site/paxeer-docs/README.md", "From this directory:\n") == ROOT / "docs/site/paxeer-docs"
    assert _public_url_error("https://api1.mainnet-beta.paxeer.network/rpc") is not None
    assert [match[0] for match in URL.finditer(
        "[http://localhost:3000](http://localhost:3000)."
    )] == ["http://localhost:3000", "http://localhost:3000"]
    text = "```python\nmake missing\n```\n~~~bash\nmake build\n~~~\n```\n$ make test\n```\n"
    assert [(language, list(_logical_commands(body, language)))
            for _, language, body in _shell_blocks(text)] == [
        ("bash", [(1, "make build")]), ("", [(1, "make test")]),
    ]
