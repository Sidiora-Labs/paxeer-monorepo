import fnmatch
import json
import re
import shlex
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/dashboard-web.kvx"
CONTAINER = ROOT / "docker/platform-dashboard-web"
APPLICATION = ROOT / "services/platform/hosted/dashboard/web"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def docker_instructions():
    text = (CONTAINER / "Dockerfile").read_text().replace("\\\n", " ")
    return [
        line.strip()
        for line in text.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


def docker_contract():
    copies = []
    arguments = set()
    runtime_env = {}
    stage = None
    for instruction in docker_instructions():
        operation, _, value = instruction.partition(" ")
        if operation == "FROM":
            stage = "runtime" if stage is not None else "build"
        elif operation == "ARG" and stage == "build":
            arguments.add(value.split("=", 1)[0])
        elif operation == "ENV" and stage == "runtime":
            runtime_env.update(
                assignment.split("=", 1)
                for assignment in shlex.split(value)
            )
        elif operation == "COPY" and stage == "build":
            words = [word for word in shlex.split(value) if not word.startswith("--")]
            copies.extend(words[:-1])
    return copies, arguments, runtime_env


def source_files():
    paths = []
    for path in APPLICATION.rglob("*"):
        if path.suffix not in {".ts", ".tsx", ".js", ".mjs", ".cjs"}:
            continue
        if any(part in {"node_modules", ".next", "tests", "fixtures"} for part in path.parts):
            continue
        if re.search(r"\.(?:test|spec)\.", path.name):
            continue
        paths.append(path)
    return paths


def source_environment():
    reads = set()
    for path in source_files():
        source = path.read_text()
        reads.update(
            re.findall(
                r"\bprocess\s*\.\s*env\s*\.\s*([A-Z][A-Z0-9_]*)", source
            )
        )
        reads.update(
            re.findall(
                r"\bprocess\s*\.\s*env\s*\[\s*['\"]([A-Z][A-Z0-9_]*)['\"]\s*\]",
                source,
            )
        )
    assert reads
    return reads


def watched(path, patterns):
    relative = "/" + path.relative_to(ROOT).as_posix()
    return any(fnmatch.fnmatchcase(relative, pattern) for pattern in patterns)


def test_dashboard_web_service_matches_rollout_and_container():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == {"dashboard-web"}
    service = data["service"]["dashboard-web"]
    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/platform-dashboard-web/Dockerfile"
    assert service["root"] == "/"
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 2
    assert service["public"] == 1
    assert service["order"] == 8
    assert service["needs"] == []
    assert "volume" not in service

    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert rollout["dashboard-web"]["railway_service"] == "dashboard-web"
    assert service["health"] == rollout["dashboard-web"]["health"]
    assert service["order"] == rollout["dashboard-web"]["order"]

    patterns = service["watch"]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert pattern.startswith("/")
        assert ".." not in Path(pattern).parts
        assert list(ROOT.glob(pattern.lstrip("/"))), pattern

    copies, _, _ = docker_contract()
    inputs = {CONTAINER / "Dockerfile"}
    for source in copies:
        path = ROOT / source
        assert path.exists(), source
        if path.is_dir():
            inputs.update(child for child in path.rglob("*") if child.is_file())
        else:
            inputs.add(path)
    assert all(watched(path, patterns) for path in inputs)


def test_dashboard_web_variables_follow_source_and_build_inputs():
    data = definition()
    variables = data["vars"]["dashboard-web"]
    copies, arguments, runtime_env = docker_contract()
    reads = source_environment()

    assert arguments == {"DASHBOARD_API_ORIGIN"}
    assert arguments <= variables.keys()
    assert variables.keys() == (reads | arguments | {"PORT"}) - {"NODE_ENV"}
    assert variables["PORT"] == runtime_env["PORT"] == "3000"
    assert variables["DASHBOARD_API_ORIGIN"] == (
        "env:PLATFORM_DASHBOARD_WEB_DASHBOARD_API_ORIGIN"
    )
    assert "NODE_ENV" in reads and variables.get("NODE_ENV") is None
    assert runtime_env["NODE_ENV"] == "production"
    assert runtime_env["HOSTNAME"] == "0.0.0.0"
    assert all(re.fullmatch(r"[A-Z][A-Z0-9_]*", name) for name in variables)
    assert all(isinstance(value, str) and value for value in variables.values())

    for argument in arguments:
        assert any(
            line.startswith("RUN ")
            and f'{argument}="${argument}"' in line
            and "npm run build" in line
            for line in docker_instructions()
        )


def test_dashboard_web_gateway_origin_is_runtime_input():
    data = definition()
    variables = data["vars"]["dashboard-web"]
    inputs = data["inputs"]["dashboard-web"]
    _, arguments, _ = docker_contract()

    assert variables["DASHBOARD_GATEWAY_ORIGIN"] == (
        "env:PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_ORIGIN"
    )
    assert inputs["PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_ORIGIN"] == "operator"
    assert variables["DASHBOARD_API_ORIGIN"] == (
        "env:PLATFORM_DASHBOARD_WEB_DASHBOARD_API_ORIGIN"
    )
    assert arguments == {"DASHBOARD_API_ORIGIN"}


def test_dashboard_web_gateway_tenant_is_runtime_input():
    data = definition()
    variables = data["vars"]["dashboard-web"]
    inputs = data["inputs"]["dashboard-web"]
    _, arguments, runtime_env = docker_contract()

    assert variables["DASHBOARD_GATEWAY_TENANT"] == (
        "env:PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_TENANT"
    )
    assert inputs["PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_TENANT"] == "operator"
    assert "DASHBOARD_GATEWAY_TENANT" in source_environment()
    assert "DASHBOARD_GATEWAY_TENANT" not in arguments
    assert "DASHBOARD_GATEWAY_TENANT" not in runtime_env
    page = (APPLICATION / "app/page.tsx").read_text()
    assert 'export const dynamic = "force-dynamic"' in page
    assert "readGatewayTenant(process.env.DASHBOARD_GATEWAY_TENANT)" in page

    lines = DEFINITION.read_text().splitlines()
    index = next(
        i
        for i, line in enumerate(lines)
        if line.startswith("PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_TENANT =")
    )
    assert index > 0
    assert lines[index - 1] == (
        "# Hosted gateway tenant: one to 95 characters from lower case letters, "
        "digits, dot, underscore and hyphen."
    )


def test_dashboard_web_inputs_are_named_classified_and_described():
    data = definition()
    variables = data["vars"]["dashboard-web"]
    inputs = data["inputs"]["dashboard-web"]
    expected = {
        value.removeprefix("env:")
        for value in variables.values()
        if value.startswith("env:")
    }
    assert set(inputs) == expected
    assert inputs == {
        "PLATFORM_DASHBOARD_WEB_DASHBOARD_API_ORIGIN": "operator",
        "PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_ORIGIN": "operator",
        "PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_TENANT": "operator",
    }
    lines = DEFINITION.read_text().splitlines()
    for name, category in inputs.items():
        assert category == "operator"
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6


def test_dashboard_web_health_and_runtime_contract():
    data = definition()
    service = data["service"]["dashboard-web"]
    variables = data["vars"]["dashboard-web"]
    recipe = "\n".join(docker_instructions())
    assert 'ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0' in recipe
    assert 'CMD ["node", "server.js"]' in recipe
    assert "USER 65532:65532" in recipe
    assert 'output: "standalone"' in (APPLICATION / "next.config.ts").read_text()
    route = APPLICATION / "app/healthz/route.ts"
    assert service["health"] == "/healthz"
    assert route.is_file() and 'new Response("ok"' in route.read_text()
    assert variables["PORT"] == "3000"


def test_dashboard_web_build_inventory_is_locked_and_static_checks_run():
    package = json.loads((ROOT / "apps/web/package.json").read_text())
    app_package = json.loads((APPLICATION / "package.json").read_text())
    assert package["name"]
    assert app_package["scripts"]["build"] == (
        "node scripts/clean-dev-output.mjs && next build && node tests/built-style.mjs"
    )
    recipe = "\n".join(docker_instructions())
    assert "npm --prefix /src/apps/web ci --ignore-scripts --no-audit --no-fund" in recipe
    assert "npm --prefix /src/apps/web run build:ui" in recipe
    assert "npm run test:static" in recipe
    assert (ROOT / "scripts/ci/developer-dashboard-lock.mjs").is_file()
