import fnmatch
import http.client
import json
import os
from pathlib import Path
import re
import selectors
import shlex
import socket
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[4]
APP = ROOT / "apps/inference-peer"
DEFINITION = ROOT / "deploy/projects/sites/inference-peer.kvx"
NAME = DEFINITION.stem
MANIFEST = tomllib.loads(DEFINITION.read_text())
SERVICE = MANIFEST["service"][NAME]
VARIABLES = MANIFEST["vars"][NAME]
INPUTS = MANIFEST["inputs"][NAME]
RECIPE = ROOT / SERVICE["dockerfile"]
IMPORT = re.compile(
    r'''\b(?:from\s*|import\s*\(\s*|import\s*)["'](\.[^"']+)["']'''
)
ENV_READ = re.compile(
    r'''\bprocess\s*\.\s*env\s*(?:\.\s*([A-Za-z_$][\w$]*)|\[\s*["']([^"']+)["']\s*\])'''
)


def instructions():
    source = re.sub(r"\\\n\s*", " ", RECIPE.read_text())
    return [
        (parts[0].upper(), parts[1])
        for line in source.splitlines()
        if (parts := line.strip().split(None, 1)) and len(parts) == 2
        and not line.lstrip().startswith("#")
    ]


def web_sources():
    command = json.loads(dict(instructions())["CMD"])
    assert command[0] == "node"
    pending = [APP / command[1], APP / "index.html"]
    sources = {}
    while pending:
        path = pending.pop().resolve()
        if path in sources:
            continue
        assert path.is_relative_to(APP), path
        assert path.is_file(), path
        source = path.read_text()
        sources[path] = source
        pending.extend(path.parent / match for match in IMPORT.findall(source))
        if path.suffix == ".html":
            pending.extend(
                path.parent / match
                for match in re.findall(r'''\bsrc=["'](\.[^"']+)["']''', source)
            )
    return sources


def environment_reads():
    reads = set()
    required = set()
    for source in web_sources().values():
        for match in ENV_READ.finditer(source):
            name = match.group(1) or match.group(2)
            reads.add(name)
            if not re.match(r"\s*(?:\?\?|\|\|)", source[match.end():]):
                required.add(name)
        for match in re.finditer(
            r"\b(?:const|let|var)\s*\{([^}]+)\}\s*=\s*process\.env\b", source
        ):
            for item in match.group(1).split(","):
                name = re.split(r"\s*[:=]\s*", item.strip())[0]
                assert re.fullmatch(r"[A-Z_][A-Z0-9_]*", name), item
                reads.add(name)
                if "=" not in item:
                    required.add(name)
    for operation, value in instructions():
        if operation == "ARG":
            name = value.split("=", 1)[0]
            reads.add(name)
            required.add(name)
    return reads, required


def port_variable():
    source = (APP / "scripts/serve.mjs").read_text()
    match = re.search(
        r"const\s+port\s*=\s*Number\(process\.env\.([A-Z_][A-Z0-9_]*)", source
    )
    assert match
    return match.group(1)


def test_repo_service_and_watch_patterns_cover_container_inputs():
    assert set(MANIFEST) == {"service", "vars", "inputs"}
    assert set(MANIFEST["service"]) == {NAME}
    assert set(MANIFEST["vars"]) == {NAME}
    assert set(MANIFEST["inputs"]) == {NAME}
    assert SERVICE["kind"] == "repo"
    assert SERVICE["railway_service"] == NAME
    assert SERVICE["root"] == "/"
    assert SERVICE["restart"] == "ALWAYS"
    assert SERVICE["replicas"] == 1
    assert SERVICE["public"] == 1
    assert SERVICE["order"] == 1
    assert SERVICE["needs"] == []
    assert "volume" not in SERVICE
    assert RECIPE.is_file()
    patterns = SERVICE["watch"]
    for pattern in patterns:
        assert pattern.startswith("/") and pattern.endswith("/**"), pattern
        assert (ROOT / pattern[1:-3]).is_dir(), pattern
    paths = {RECIPE.relative_to(ROOT).as_posix()}
    for operation, value in instructions():
        if operation != "COPY":
            continue
        parts = shlex.split(value)
        assert not parts[0].startswith("--from=")
        for part in parts[:-1]:
            copied = ROOT / part
            assert copied.exists(), part
            paths.add(part)
            if copied.is_dir():
                paths.update(
                    p.relative_to(ROOT).as_posix()
                    for p in copied.rglob("*") if p.is_file()
                )
    for path in paths:
        candidate = "/" + path + ("/" if (ROOT / path).is_dir() else "")
        assert any(fnmatch.fnmatch(candidate, pattern) for pattern in patterns), path


def test_variables_are_derived_from_web_entry_points():
    reads, required = environment_reads()
    assert required <= VARIABLES.keys(), sorted(required - VARIABLES.keys())
    assert VARIABLES.keys() <= reads, sorted(VARIABLES.keys() - reads)
    server = (APP / "scripts/serve.mjs").read_text()
    port_name = port_variable()
    assert port_name in VARIABLES
    assert re.search(r"server\.listen\(port,\s*[\"']0\.0\.0\.0[\"']", server)
    exposed = int(dict(instructions())["EXPOSE"])
    assert int(VARIABLES[port_name]) == exposed
    defaults = dict(
        item.split("=", 1) for item in shlex.split(dict(instructions())["ENV"])
    )
    assert defaults[port_name] == VARIABLES[port_name]
    environment_names = set()
    prefix = "_".join((DEFINITION.parent.name.upper(), NAME.upper().replace("-", "_"))) + "_"
    lines = DEFINITION.read_text().splitlines()
    for variable, value in VARIABLES.items():
        assert re.fullmatch(r"[A-Z_][A-Z0-9_]*", variable)
        assert isinstance(value, str)
        if not value.startswith("env:"):
            continue
        name = value[4:]
        assert name == prefix + variable
        environment_names.add(name)
        assert INPUTS[name] in {"generated", "provider", "chain", "operator"}
        row = next(
            i for i, line in enumerate(lines)
            if re.match(re.escape(name) + r"\s*=", line)
        )
        assert row > 0 and lines[row - 1].startswith("# ") and len(lines[row - 1]) > 10
    assert INPUTS.keys() == environment_names


def test_container_uses_pinned_node_and_unprivileged_web_command():
    recipe = dict(instructions())
    assert re.fullmatch(r"node:[^\s@]+@sha256:[0-9a-f]{64}", recipe["FROM"])
    assert recipe["USER"] == "65532:65532"
    assert recipe["WORKDIR"] == "/app"
    command = json.loads(recipe["CMD"])
    package = json.loads((APP / "package.json").read_text())
    assert shlex.split(package["scripts"]["serve"]) == command
    assert not package.get("dependencies")
    inventory = subprocess.run(
        [
            "bash", "-c",
            'source services/platform/hosted/tests/beta-images.sh; '
            '[[ " ${IMAGE_NAMES[*]} " == *" inference-peer-web "* ]] && '
            'image_source inference-peer-web && '
            'printf "\\n%s\\n" "$(image_context inference-peer-web)"',
        ],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout.splitlines()
    assert inventory == [
        "ghcr.io/sidiora-labs/inference-peer-web:0.1.0 " + SERVICE["dockerfile"],
        ".",
    ]
    registration = "|".join(
        ("inference-peer-web", SERVICE["dockerfile"], SERVICE["dockerfile"], ".")
    )
    assert registration in (ROOT / "tools/containers/check.sh").read_text().splitlines()


def test_web_command_serves_health_and_modules_on_configured_port():
    command = json.loads(dict(instructions())["CMD"])
    with socket.socket() as available:
        available.bind(("0.0.0.0", 0))
        port = available.getsockname()[1]
    environment = dict(os.environ)
    environment[port_variable()] = str(port)
    process = subprocess.Popen(
        command, cwd=APP, env=environment, stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, text=True,
    )
    connection = None
    try:
        with selectors.DefaultSelector() as ready:
            ready.register(process.stdout, selectors.EVENT_READ)
            assert ready.select(timeout=10), "Web listener did not start"
        assert process.stdout.readline().strip() == f"Inference peer: http://0.0.0.0:{port}"
        connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
        routes = {SERVICE["health"]: (APP / "index.html").read_bytes()}
        routes.update(
            ("/" + path.relative_to(APP).as_posix(), source.encode())
            for path, source in web_sources().items()
            if path.is_relative_to(APP / "src") or path.is_relative_to(APP / "engine")
        )
        routes["/engine/engine/engine.js"] = (APP / "engine/engine/engine.js").read_bytes()
        for path, content in routes.items():
            connection.request("GET", path)
            response = connection.getresponse()
            assert response.status == 200, path
            assert response.getheader("cross-origin-opener-policy") == "same-origin"
            assert response.getheader("cross-origin-embedder-policy") == "require-corp"
            assert response.read() == content, path
        for path in ["/package.json", "/.env", "/test/page.test.mjs"]:
            connection.request("GET", path)
            response = connection.getresponse()
            assert response.status == 404, path
            response.read()
    finally:
        if connection:
            connection.close()
        process.terminate()
        try:
            process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.communicate(timeout=5)
    assert process.returncode == 0
