import fnmatch
import json
import posixpath
import re
import shlex
import tomllib
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[4]
SITE = ROOT / "docs/site/paxeer-docs"
ASSISTANT = SITE.parent / "docs-assistant"
DEFINITION = ROOT / "deploy/projects/sites/docs.kvx"
JS_TOKEN = re.compile(
    r"(?P<comment>//[^\n]*|/\*[\s\S]*?\*/)"
    r"|(?P<string>'(?:\\.|[^'\\])*'|\"(?:\\.|[^\"\\])*\"|`(?:\\.|[^`\\])*`)"
    r"|[A-Za-z_$][\w$]*|\d+(?:\.\d+)?|===|!==|==|!=|\|\||\?\?|&&|[^\s]"
)


def definition():
    return tomllib.loads(DEFINITION.read_text())


def docker_stages():
    stages = {}
    stage = None
    text = (SITE / "Dockerfile").read_text().replace("\\\n", " ")
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        operation, value = line.split(None, 1)
        if operation == "FROM":
            stage = {"workdir": "/", "copies": [], "env": {}, "args": set()}
            stages[value.split()[-1]] = stage
        elif operation == "WORKDIR":
            stage["workdir"] = posixpath.join(stage["workdir"], value)
        elif operation == "COPY":
            words = shlex.split(value)
            origin = None
            if words[0].startswith("--from="):
                origin = words.pop(0).split("=", 1)[1]
            destination = posixpath.normpath(
                posixpath.join(stage["workdir"], words[-1])
            )
            for source in words[:-1]:
                stage["copies"].append((origin, source, destination, words[-1]))
        elif operation == "ENV":
            stage["env"].update(word.split("=", 1) for word in shlex.split(value))
        elif operation == "ARG":
            stage["args"].add(value.split("=", 1)[0])
        elif operation in {"CMD", "EXPOSE", "USER", "HEALTHCHECK", "RUN"}:
            stage.setdefault(operation, []).append(value)
    return stages


def source_files():
    directories = (
        SITE / "src",
        SITE / "scripts",
        ASSISTANT / "src",
        ASSISTANT / "server",
    )
    files = {path for directory in directories for path in directory.rglob("*")}
    files.update(SITE.glob("*.js"))
    files.update(SITE.glob("*.mjs"))
    return sorted(
        path for path in files if path.suffix in {".js", ".mjs", ".ts", ".tsx"}
    )


def js_tokens(text):
    tokens = []
    for match in JS_TOKEN.finditer(text):
        if match.lastgroup == "comment":
            continue
        value = match.group()
        tokens.append((value, match.start()))
        if value.startswith("`"):
            for expression in re.finditer(r"\$\{([^{}]*)\}", value):
                tokens.extend(
                    (token, match.start() + expression.start(1) + offset)
                    for token, offset in js_tokens(expression.group(1))
                )
    return tokens


def env_at(tokens, index):
    values = [value for value, _ in tokens[index : index + 6]]
    if values[:3] != ["process", ".", "env"]:
        return None
    if len(values) >= 5 and values[3] == ".":
        assert re.fullmatch(r"[A-Z_][A-Z0-9_]*", values[4])
        return values[4], index + 5
    assert len(values) == 6 and values[3] == "[" and values[5] == "]"
    assert re.fullmatch(r"['\"][A-Z_][A-Z0-9_]*['\"]", values[4])
    return values[4][1:-1], index + 6


def environment_contract():
    reads, required = set(), set()
    for path in source_files():
        text = path.read_text()
        tokens = js_tokens(text)
        index = 0
        while index < len(tokens):
            read = env_at(tokens, index)
            if read is None:
                index += 1
                continue
            start = tokens[index][1]
            names = set()
            while read:
                name, index = read
                names.add(name)
                if index < len(tokens) and tokens[index][0] in {"||", "??"}:
                    read = env_at(tokens, index + 1)
                    if read:
                        index += 1
                        continue
                break
            reads.update(names)
            suffix = [value for value, _ in tokens[index : index + 2]]
            has_default = (
                len(suffix) == 2
                and suffix[0] in {"||", "??"}
                and (
                    suffix[1].startswith(("'", '"', "`"))
                    or re.fullmatch(r"\d+(?:\.\d+)?|true|false|null", suffix[1])
                    or suffix[1] in {"[", "{"}
                )
            )
            line_start = text.rfind("\n", 0, start) + 1
            line_end = text.find("\n", start)
            line = text[line_start : line_end if line_end >= 0 else len(text)]
            prefix = text[line_start:start]
            conditional = re.search(r"\bif\s*\(", prefix) and "throw" not in line
            comparison = bool(suffix and suffix[0] in {"===", "!==", "==", "!="})
            cli_option = "options[" in prefix
            if not (has_default or conditional or comparison or cli_option):
                required.add(frozenset(names))
    return reads, required


def test_service_sources_and_watch_patterns():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == {"docs"}
    service = data["service"]["docs"]
    assert set(service) == {
        "kind",
        "dockerfile",
        "root",
        "watch",
        "health",
        "restart",
        "replicas",
        "public",
        "order",
        "needs",
    }
    assert service["kind"] == "repo"
    assert service["root"] == "/"
    assert ROOT / service["dockerfile"] == SITE / "Dockerfile"
    assert (ROOT / service["dockerfile"]).is_file()
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["order"] == 1
    assert service["needs"] == []
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())
    if "docs" in rollout["service"]:
        assert rollout["service"]["docs"]["railway_service"] == "docs"
        assert service["order"] == rollout["service"]["docs"]["order"]
        assert service["health"] == rollout["service"]["docs"]["health"]
    patterns = [pattern.lstrip("/") for pattern in service["watch"]]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert ".." not in PurePosixPath(pattern).parts
        assert list(ROOT.glob(pattern)), f"Watch pattern has no source: {pattern}"
    paths = {SITE / "Dockerfile"}
    for stage in docker_stages().values():
        for origin, source, _, _ in stage["copies"]:
            if origin is not None:
                continue
            path = ROOT / source
            assert path.exists(), f"Missing COPY source: {source}"
            assert path.is_relative_to(ROOT)
            if path.is_dir():
                paths.update(item for item in path.rglob("*") if item.is_file())
            else:
                paths.add(path)
    for path in paths:
        relative = path.relative_to(ROOT).as_posix()
        assert any(
            fnmatch.fnmatchcase(relative, pattern) for pattern in patterns
        ), relative


def test_variables_follow_source_reads_and_build_arguments():
    data = definition()
    assert set(data["vars"]) == {"docs"}
    variables = data["vars"]["docs"]
    reads, required = environment_contract()
    stages = docker_stages()
    arguments = set().union(*(stage["args"] for stage in stages.values()))
    container_settings = set(stages["runtime"]["env"]) & reads
    assert set(variables) <= reads | arguments
    assert arguments | container_settings <= set(variables)
    for alternatives in required:
        assert alternatives & set(variables), (
            f"Required source input: {sorted(alternatives)}"
        )
    necessary = arguments | container_settings | set().union(*required)
    assert set(variables) <= necessary
    for name, value in variables.items():
        assert isinstance(value, str) and value
        assert re.fullmatch(r"[A-Z_][A-Z0-9_]*", name)
        if name in container_settings:
            assert value == stages["runtime"]["env"][name]
        else:
            assert value.startswith("env:")
    runtime = (ASSISTANT / "server/assistant.mjs").read_text()
    proxy = re.search(r"process\.env\.([A-Z_]+)\s*===\s*'([^']+)'", runtime)
    assert proxy and variables[proxy[1]] == proxy[2]
    for argument in arguments:
        assert argument in reads
        assert any(
            f"process.env.{argument}" in path.read_text() for path in source_files()
        )


def test_inputs_have_names_categories_and_format_comments():
    data = definition()
    assert set(data["inputs"]) == {"docs"}
    inputs = data["inputs"]["docs"]
    external = {}
    prefix = f"{DEFINITION.parent.name}_docs".upper().replace("-", "_")
    for variable, value in data["vars"]["docs"].items():
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            assert name == f"{prefix}_{variable}"
            external[name] = variable
    assert set(inputs) == set(external)
    lines = DEFINITION.read_text().splitlines()
    for name, category in inputs.items():
        assert category in {"generated", "provider", "chain", "operator"}
        index = next(
            i for i, line in enumerate(lines) if line.startswith(name + " =")
        )
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6
        assert re.search(
            r"URL|token|secret|key|integer|JSON|hex|base64", lines[index - 1], re.I
        )


def test_port_and_health_match_the_server():
    data = definition()
    service, variables = data["service"]["docs"], data["vars"]["docs"]
    server = (ASSISTANT / "server/index.mjs").read_text()
    listener = re.search(
        r"const port = Number\(process\.env\.([A-Z_]+) \|\| (\d+)\)", server
    )
    assert listener
    assert "server.listen(port,host," in server
    port_name, port_default = listener.groups()
    assert variables[port_name] == port_default
    runtime = docker_stages()["runtime"]
    assert runtime["EXPOSE"] == [variables[port_name]]
    routes = re.findall(r"url\.pathname === '([^']+)'", server)
    assert service["health"] in routes
    handler = server.split(f"url.pathname === '{service['health']}'", 1)[1]
    assert "outgoing.writeHead(200" in handler.split("\n      }", 1)[0]
    assert service["health"] in runtime["HEALTHCHECK"][0]
    assert f"process.env.{port_name}" in runtime["HEALTHCHECK"][0]
    assert runtime["USER"] == ["node"]
    assert json.loads(runtime["CMD"][0]) == ["node", "scripts/serve.mjs"]


def test_container_includes_build_inventory_and_runtime_outputs():
    stages = docker_stages()
    build, runtime = stages["build"], stages["runtime"]
    manifest = json.loads((SITE / "package.json").read_text())
    dependency = json.loads((ASSISTANT / "package.json").read_text())["name"]
    package_path = manifest["dependencies"][dependency].removeprefix("file:")
    dependency_destination = posixpath.normpath(
        posixpath.join(build["workdir"], package_path)
    )
    assert (
        None,
        "docs/site/docs-assistant",
        dependency_destination,
        dependency_destination,
    ) in build["copies"]
    assert build["RUN"] == ["npm ci", "npm run build"]
    assert "prepare:docs" in manifest["scripts"]["prebuild"]
    generator = SITE / "scripts/generate-systems.mjs"
    inventory_relative = re.search(
        r"const inventory = path.resolve\(root, '([^']+)'\)", generator.read_text()
    )
    assert inventory_relative
    inventory = (SITE / inventory_relative[1]).resolve()
    assert inventory.is_file()
    destination = posixpath.normpath(
        posixpath.join(build["workdir"], inventory_relative[1])
    )
    assert (
        None,
        inventory.relative_to(ROOT).as_posix(),
        destination,
        destination,
    ) in build["copies"]
    generated = (SITE / "scripts/generate-llms.mjs").read_text()
    assert "'.generated/assistant-docs.json'" in generated
    assert "'export'" in (SITE / "next.config.js").read_text()
    for output in ("out", ".generated/assistant-docs.json"):
        assert (
            "build",
            posixpath.join(build["workdir"], output),
            posixpath.join(runtime["workdir"], output),
            "./" + output,
        ) in runtime["copies"]
    entry = SITE / "scripts/serve.mjs"
    imported = re.search(r"from '([^']*docs-assistant[^']+)'", entry.read_text())
    assert imported and (entry.parent / imported[1]).resolve().is_file()
    entry_destination = posixpath.join(runtime["workdir"], "scripts/serve.mjs")
    server_destination = posixpath.normpath(
        posixpath.join(posixpath.dirname(entry_destination), imported[1])
    )
    assert (
        None,
        "docs/site/paxeer-docs/scripts/serve.mjs",
        entry_destination,
        "./scripts/serve.mjs",
    ) in runtime["copies"]
    assert (
        None,
        "docs/site/docs-assistant/server",
        posixpath.dirname(server_destination),
        posixpath.dirname(server_destination),
    ) in runtime["copies"]
    assert "kind: 'static'" in entry.read_text()
