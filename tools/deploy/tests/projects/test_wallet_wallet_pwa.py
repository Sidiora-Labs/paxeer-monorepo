"""Check the wallet hosting definition against its container and application sources."""

import fnmatch
import json
import os
import re
import shlex
import subprocess
import tomllib
from pathlib import Path
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/wallet/wallet-pwa.kvx"
APP = ROOT / "apps/wallet"
CONTAINER = ROOT / "docker/wallet-pwa"
ENV_NAME = r"[A-Z][A-Z0-9_]*"
DIRECT_READ = re.compile(
    rf"process\.env(?:\.({ENV_NAME})|\[\s*['\"]({ENV_NAME})['\"]\s*\])"
)


def definition():
    with DEFINITION.open("rb") as source:
        return tomllib.load(source)


def service_name():
    return DEFINITION.stem


def docker_instructions():
    text = (CONTAINER / "Dockerfile").read_text().replace("\\\n", " ")
    return [shlex.split(line) for line in text.splitlines() if line and not line.startswith("#")]


def build_arguments():
    return {
        words[1].split("=", 1)[0]
        for words in docker_instructions()
        if words[0] == "ARG"
    }


def application_sources():
    for folder in (APP / "src", APP / "scripts", ROOT / "sdk/wallet/src", ROOT / "services/agent/sdk/typescript/src"):
        for path in sorted(folder.rglob("*")):
            if path.suffix not in {".ts", ".tsx", ".js", ".mjs"}:
                continue
            if any(part in {"node_modules", "__tests__", "test", "tests", "fixtures"} for part in path.parts):
                continue
            if any(marker in path.name for marker in (".test.", ".spec.", "fixture")):
                continue
            yield path
    yield from APP.glob("sentry.*.config.ts")
    yield APP / "next.config.mjs"


def source_variables():
    reads, required = set(), set()
    for path in application_sources():
        source = path.read_text()
        reads.update(first or second for first, second in DIRECT_READ.findall(source))

        # Required readers resolve a dynamic process.env[name] through their calls.
        functions = re.finditer(
            r"function\s+(\w+)\s*\(\s*(\w+)[^)]*\)[^{]*\{(.*?)\n\}",
            source,
            re.S,
        )
        for function in functions:
            name, parameter, body = function.groups()
            if re.search(rf"process\.env\[\s*{parameter}\s*\]", body):
                names = set(re.findall(rf"\b{name}\(\s*['\"]({ENV_NAME})['\"]", source))
                reads.update(names)
                if "throw " in body:
                    required.update(names)

        # Configuration interfaces distinguish required properties from optional ones.
        maps = {}
        for mapping in re.finditer(r"const\s+(\w+)\s*=\s*\{(.*?)\}\s*as const", source, re.S):
            for key, value in re.findall(rf"(\w+):\s*['\"]({ENV_NAME})['\"]", mapping[2]):
                maps[f"{mapping[1]}.{key}"] = value
        required_fields = set()
        for interface in re.finditer(r"interface\s+\w+Config\s*\{(.*?)\n\}", source, re.S):
            required_fields.update(re.findall(r"(?:readonly\s+)?(\w+)\s*:", interface[1]))
        validators = set()
        for function in re.finditer(r"function\s+(\w+)\([^)]*\)[^{]*\{(.*?)\n\}", source, re.S):
            if re.search(r"env\[\s*\w+\s*\]", function[2]) and "throw " in function[2]:
                validators.add(function[1])
        for field in required_fields:
            for validator in validators:
                for key in re.findall(rf"\b{field}:\s*{validator}\(env,\s*(\w+\.\w+)\)", source):
                    assert key in maps, (path, key)
                    required.add(maps[key])
        for key in re.findall(r"throw new \w+\(\s*(\w+\.\w+)\s*,", source):
            if key in maps:
                required.add(maps[key])

        for assignment in re.finditer(rf"(?:const|let)\s+(\w+)\s*=\s*process\.env\.({ENV_NAME})\s*;", source):
            local, variable = assignment.groups()
            following = source[assignment.end():]
            if re.match(rf"\s*if\s*\(\s*!{local}\s*(?:\|\|[^)]*)?\)\s*\{{?\s*(?:throw|return publicError)", following):
                required.add(variable)
        for guard in re.finditer(r"if\s*\(([^;{}]+)\)\s*\{\s*throw\b", source):
            required.update(re.findall(rf"!process\.env\.({ENV_NAME})", guard[1]))

    for path in (CONTAINER / "start-wallet.sh", CONTAINER / "render-nginx.cjs", APP / "deployment/render-edge-trust.sh"):
        source = path.read_text()
        names = {first or second for first, second in DIRECT_READ.findall(source)}
        names.update(re.findall(rf"\$\{{({ENV_NAME})(?::[-?+][^}}]*)?\}}", source))
        reads.update(names)
        required.update(names)
    return reads, required


def copy_sources():
    for words in docker_instructions():
        if words[0] != "COPY" or any(word.startswith("--from=") for word in words):
            continue
        yield from (word for word in words[1:-1] if not word.startswith("--"))


def watched(path, patterns):
    return any(
        fnmatch.fnmatchcase(path, pattern)
        or path == pattern.removesuffix("/**")
        for pattern in patterns
    )


def test_service_matches_container_and_deployment():
    data = definition()
    name = service_name()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == {name}
    service = data["service"][name]
    assert service["kind"] == "repo"
    assert service["dockerfile"] == (CONTAINER / "Dockerfile").relative_to(ROOT).as_posix()
    assert service["root"] == "."
    assert (ROOT / service["dockerfile"]).is_file()
    deployment = json.loads((APP / "railway.json").read_text())["deploy"]
    assert service["health"] == deployment["healthcheckPath"]
    assert service["restart"] == deployment["restartPolicyType"]
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert "target" not in service

    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert service["needs"]
    assert len(service["needs"]) == len(set(service["needs"]))
    for dependency in service["needs"]:
        assert rollout[dependency]["railway_service"] == dependency
        assert rollout[dependency]["order"] < service["order"]
    assert service["order"] == max(rollout[dep]["order"] for dep in service["needs"]) + 1


def test_watch_patterns_cover_every_container_input():
    service = definition()["service"][service_name()]
    patterns = service["watch"]
    assert patterns and len(patterns) == len(set(patterns))
    for pattern in patterns:
        base = pattern.split("*", 1)[0].rstrip("/")
        assert base not in {"", "."}
        assert (ROOT / base).exists(), pattern
    inputs = list(copy_sources())
    assert inputs
    for path in inputs:
        assert (ROOT / path).exists(), path
        assert watched(path, patterns), f"Container input needs a watch pattern: {path}"
    for path in CONTAINER.iterdir():
        assert watched(path.relative_to(ROOT).as_posix(), patterns), path

    ignore = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for path in inputs:
        included = True
        ancestors = [path, *(parent.as_posix() for parent in Path(path).parents if parent != Path("."))]
        for raw in ignore:
            if not raw or raw.startswith("#"):
                continue
            negated = raw.startswith("!")
            pattern = raw.removeprefix("!")
            if any(fnmatch.fnmatchcase(candidate, pattern) for candidate in ancestors):
                included = negated
        assert included, f"Container input is excluded: {path}"


def test_variables_match_source_reads_and_build_arguments():
    variables = definition()["vars"][service_name()]
    reads, required = source_variables()
    arguments = build_arguments()
    assert arguments
    assert required
    assert required | arguments <= variables.keys(), sorted((required | arguments) - variables.keys())
    assert variables.keys() <= reads | arguments, sorted(variables.keys() - reads - arguments)
    forwarded = {
        word.split("=", 1)[0]: word.split("=", 1)[1]
        for words in docker_instructions()
        if words[0] == "ENV"
        for word in words[1:]
        if "=" in word
    }
    for argument in arguments:
        assert forwarded[argument] in {f"${argument}", f"${{{argument}}}"}
    for variable in required:
        if variable.startswith("NEXT_PUBLIC_"):
            assert variable in arguments, f"Browser setting needs a build argument: {variable}"


def test_inputs_have_scoped_names_categories_and_format_comments():
    data = definition()
    name = service_name()
    prefix = f"{DEFINITION.parent.name}_{name}_".upper().replace("-", "_")
    inputs = data["inputs"][name]
    expected = set()
    for variable, value in data["vars"][name].items():
        assert re.fullmatch(ENV_NAME, variable)
        assert isinstance(value, str)
        if value.startswith("env:"):
            external = value.removeprefix("env:")
            assert external == prefix + variable
            expected.add(external)
        else:
            assert re.fullmatch(r"[0-9]+", value), f"Deployment values must be inputs: {variable}"
    assert set(inputs) == expected
    lines = DEFINITION.read_text().splitlines()
    start = lines.index(f"[inputs.{name}]")
    for external, category in inputs.items():
        assert category in {"generated", "provider", "chain", "operator"}
        row = next(index for index in range(start + 1, len(lines)) if lines[index].startswith(external + " ="))
        assert lines[row - 1].startswith("# ")
        description = lines[row - 1].removeprefix("# ")
        assert len(description) >= 20 and ";" in description


def test_health_route_and_persistent_state_match_definition():
    service = definition()["service"][service_name()]
    config = (APP / "next.config.mjs").read_text()
    base = re.search(r"basePath:\s*['\"]([^'\"]+)['\"]", config)[1]
    health = urlsplit(service["health"])
    route = APP / "src/app" / health.path.removeprefix(base).lstrip("/") / "route.ts"
    source = route.read_text()
    assert "export async function GET(" in source
    assert health.query == "mode=liveness"
    assert re.search(r"mode === ['\"]liveness['\"]", source)
    assert "NextResponse.json(" in source
    for path in (APP / "src/server/push-store.ts", APP / "src/lib/rateLimit.ts"):
        defaults = re.findall(r"process\.env\.\w+\s*\?\?\s*['\"]([^'\"]+)['\"]", path.read_text())
        assert defaults
        for default in defaults:
            assert Path(default).is_relative_to(service["volume"])
    docker = (CONTAINER / "Dockerfile").read_text()
    assert service["volume"] in docker
    assert "USER nextjs" in docker
    assert "chown -R nextjs:nodejs /data/paxport /tmp/nginx" in docker


@pytest.mark.parametrize("port", [None, "3000", "8080", "65535"])
def test_nginx_renderer_uses_public_port_and_separate_application_port(tmp_path, port):
    output = tmp_path / "nginx.conf"
    environment = {key: value for key, value in os.environ.items() if key != "PORT"}
    if port is not None:
        environment["PORT"] = port
    result = subprocess.run(
        ["node", str(CONTAINER / "render-nginx.cjs"), str(APP / "deployment/nginx.conf"), str(output)],
        env=environment,
        text=True,
        capture_output=True,
        check=True,
    )
    actual_port = port if port is not None else definition()["vars"][service_name()]["PORT"]
    rendered = output.read_text()
    assert re.findall(r"\blisten (\d+);", rendered) == [actual_port]
    assert re.findall(r"\bserver 127\.0\.0\.1:(\d+);", rendered) == [result.stdout]
    assert result.stdout != actual_port
    assert output.stat().st_mode & 0o777 == 0o600
    template = (APP / "deployment/nginx.conf").read_text()
    expected = template.replace("listen 3001;", f"listen {actual_port};").replace(
        "server 127.0.0.1:3000;", f"server 127.0.0.1:{result.stdout};"
    )
    assert rendered == expected


@pytest.mark.parametrize("port", ["", "0", "80", "1023", "65536", "-1", "3.5", "03001", "8080;", " 8080"])
def test_nginx_renderer_rejects_invalid_ports(tmp_path, port):
    output = tmp_path / "nginx.conf"
    result = subprocess.run(
        ["node", str(CONTAINER / "render-nginx.cjs"), str(APP / "deployment/nginx.conf"), str(output)],
        env={**os.environ, "PORT": port},
        text=True,
        capture_output=True,
    )
    assert result.returncode != 0
    assert "PORT must be an integer between 1024 and 65535" in result.stderr
    assert not output.exists()


def test_launcher_keeps_trust_rendering_and_supervises_both_processes():
    launcher = (CONTAINER / "start-wallet.sh").read_text()
    original = (APP / "deployment/start-wallet.sh").read_text()
    for fragment in (
        'secret="${TRUSTED_PROXY_SECRET:-}"',
        '*[!A-Za-z0-9._~+/=-]*)',
        "proxy_set_header x-paxport-proxy-secret",
        'sh "$(dirname -- "$0")/render-edge-trust.sh" /tmp/nginx/edge-trust.conf',
        "trap shutdown TERM INT EXIT",
        'kill -TERM "$nginx_pid"',
        'kill -TERM "$node_pid"',
        'kill -0 "$node_pid"',
        'kill -0 "$nginx_pid"',
    ):
        assert fragment in original
        assert fragment in launcher
    assert "umask 077" in launcher
    assert 'app_port="$(node /app/render-nginx.cjs /app/nginx.conf.template /tmp/nginx/nginx.conf)"' in launcher
    assert 'PORT="$app_port" HOSTNAME=127.0.0.1 node /app/apps/wallet/server.js' in launcher
    assert "nginx -e /dev/stderr -c /tmp/nginx/nginx.conf" in launcher
    docker = (CONTAINER / "Dockerfile").read_text()
    assert "docker/wallet-pwa/start-wallet.sh /app/start-wallet.sh" in docker
    assert "docker/wallet-pwa/render-nginx.cjs /app/render-nginx.cjs" in docker
    assert "apps/wallet/deployment/nginx.conf /app/nginx.conf.template" in docker
    assert "apps/wallet/deployment/render-edge-trust.sh /app/render-edge-trust.sh" in docker
    assert "process.env.PORT" in docker.split("HEALTHCHECK", 1)[1]
    assert 'ENTRYPOINT ["/sbin/tini", "-g", "--"]' in docker
    assert 'CMD ["/app/start-wallet.sh"]' in docker


def test_pwa_build_argument_preserves_same_origin_name_default():
    source = (APP / "src/pwa/config.ts").read_text()
    names = re.search(r"names:\s*['\"]([^'\"]+)['\"]", source)[1]
    arguments = {
        words[1].split("=", 1)[0]: words[1].partition("=")[2]
        for words in docker_instructions()
        if words[0] == "ARG"
    }
    assert names in arguments
    assert arguments[names] == ""
    assert "if (!raw) return null;" in source
    assert "parsed = new URL(raw);" in source
    assert "'/wallet/api/pns'" in (APP / "src/lib/pns.ts").read_text()
