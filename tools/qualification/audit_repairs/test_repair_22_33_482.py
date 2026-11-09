"""Offline source proofs for the Compose and runtime-environment audit repairs.

Parse the real deployment assets and trace their settings into the production
Dockerfile and candle route. No Docker, Node, shell, network or credentials are
needed; runtime deployment and image build behavior are explicitly review-only.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit

import yaml


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "apps/perps/deploy"
RECORD = Path(__file__).with_name("repair_22_33_482.kvx")
COMPOSE_PREFIX = [
    "docker", "compose", "--env-file", ".env.production", "-f",
    "deploy/docker-compose.yml",
]
RECREATE = [
    "up", "-d", "--no-build", "--pull", "never", "--force-recreate",
    "--wait", "app",
]


class UniqueKeyLoader(yaml.SafeLoader):
    """Reject duplicate mapping keys instead of silently dropping a setting."""


def unique_mapping(loader, node):
    pairs = loader.construct_pairs(node, deep=True)
    result = {}
    for key, value in pairs:
        assert key not in result, f"Duplicate YAML key: {key}"
        result[key] = value
    return result


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, unique_mapping
)


def source(relative):
    return (ROOT / relative).read_text(encoding="utf-8")


def compose():
    return yaml.load(
        (DEPLOY / "docker-compose.yml").read_text(encoding="utf-8"),
        Loader=UniqueKeyLoader,
    )


def readme():
    return (DEPLOY / "README.md").read_text(encoding="utf-8")


def section(heading, level=3):
    marker = "#" * level + " " + heading + "\n"
    assert readme().count(marker) == 1, heading
    return re.split(
        r"^#{1," + str(level) + r"} ",
        readme().split(marker, 1)[1], maxsplit=1, flags=re.M,
    )[0]


def commands(text):
    """Tokenize shell examples in-process, preserving quoted multiline JS."""
    result = []
    for script in re.findall(r"```bash\n(.*?)\n```", text, re.S):
        lexer = shlex.shlex(script, posix=True, punctuation_chars="\n")
        lexer.whitespace = " \t\r"
        lexer.whitespace_split = True
        current = []
        for token in lexer:
            if token and not token.strip("\n"):
                if current:
                    result.append(current)
                    current = []
            else:
                current.append(token)
        if current:
            result.append(current)
    return result


def binding(value, variable, operator):
    match = re.fullmatch(r"\$\{([A-Z_][A-Z_0-9]*)(:\?|:-)([^{}]*)\}", value)
    assert match, value
    assert match.group(1, 2) == (variable, operator)
    return match.group(3)


def test_compose_app_build_context_and_public_inputs():
    app = compose()["services"]["app"]
    build = app["build"]
    context = (DEPLOY / build["context"]).resolve()
    assert context == ROOT / "apps/perps"
    assert build["dockerfile"] == "Dockerfile"
    dockerfile = (context / build["dockerfile"]).read_text(encoding="utf-8")
    declared = set(re.findall(r"^ARG (NEXT_PUBLIC_\w+)=", dockerfile, re.M))
    assert declared and set(build["args"]) == declared
    for name, value in build["args"].items():
        if name == "NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA":
            assert binding(value, "BUILD_COMMIT_SHA", ":?")
        elif name == "NEXT_PUBLIC_PERPS_GATEWAY_URL":
            assert binding(value, name, ":?")
        else:
            assert binding(value, name, ":-") == ""
        assert f"{name}=${name}" in dockerfile
    assert "MARKET_PROXY_ORIGIN" not in build["args"]
    assert "MARKET_PROXY_HOST" not in build["args"]
    assert "COPY package.json pnpm-lock.yaml ./" in dockerfile
    assert "RUN pnpm install --frozen-lockfile" in dockerfile
    assert "pnpm build" in dockerfile
    ignore = (context / ".dockerignore").read_text().splitlines()
    assert ".env" in ignore and ".env.*" in ignore
    identity = app["environment"]["VERCEL_GIT_COMMIT_SHA"]
    assert identity == build["args"]["NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA"]
    assert app["image"] == "perps-ui:" + identity
    assert "latest" not in app["image"]


def test_compose_runtime_origin_reaches_production_candle_headers():
    env = compose()["services"]["app"]["environment"]
    assert binding(env["MARKET_PROXY_ORIGIN"], "MARKET_PROXY_ORIGIN", ":?")
    assert binding(env["MARKET_PROXY_HOST"], "MARKET_PROXY_HOST", ":-") == (
        "data.feed.chainflowtrading.com"
    )
    assert env["NODE_ENV"] == "production"
    assert env["PORT"] == "3000" and env["HOSTNAME"] == "0.0.0.0"
    assert not any(key.startswith("NEXT_PUBLIC_") for key in env)
    assert "env_file" not in compose()["services"]["app"]
    dockerfile = source("apps/perps/Dockerfile")
    runner = dockerfile.split(" AS runner\n", 1)[1]
    assert "MARKET_PROXY_ORIGIN" not in runner
    assert "MARKET_PROXY_HOST" not in runner
    assert 'ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0' in runner
    command = re.search(r"^CMD (.*)$", runner, re.M)
    assert json.loads(command[1]) == ["node", "server.js"]
    route = source("apps/perps/src/pages/api/candle/[...path].ts")
    parser = route.split("export function parseMarketProxyOrigin(", 1)[1].split(
        "\n// A set but malformed", 1
    )[0]
    assert 'if (!raw) throw new Error(`${ORIGIN_ENV} is not set`)' in parser
    assert 'url.protocol !== "http:" && url.protocol !== "https:"' in parser
    assert "if (url.origin !== raw) throw new Error(" in parser
    assert "return raw;" in parser
    origin = route.split("function upstreamOrigin()", 1)[1].split(
        "\n// Rolling", 1
    )[0]
    assert "return { origin: parseMarketProxyOrigin(process.env.MARKET_PROXY_ORIGIN) };" in origin
    handler = route.split("export default async function handler(", 1)[1]
    assert "const proxyOrigin = upstreamOrigin();" in handler
    assert 'if (!("origin" in proxyOrigin))' in handler
    assert 'return res.status(503).json({ error: "MARKET_PROXY_ORIGIN is not configured" });' in handler
    assert "const origin = proxyOrigin.origin;" in handler
    fetch = handler.split("const upstreamRes = await fetch(upstream, {", 1)[1].split(
        "\n    });", 1
    )[0]
    assert re.search(r"headers:\s*\{\s*Accept: \"application/json\",\s*Origin: origin,\s*\}", fetch)
    assert "method," in fetch
    assert '"config",' in route
    assert 'process.env.MARKET_PROXY_HOST || "data.feed.chainflowtrading.com"' in route
    assert "const base = getUpstream(symbolParam);" in handler
    assert "const upstream = `${base}/${endpoint}" in handler
    assert "return res.status(upstreamRes.status).json(data);" in handler


def test_compose_private_ingress_and_inherited_image_health():
    config = compose()
    assert set(config["services"]) == {"app", "nginx"}
    app, nginx = config["services"]["app"], config["services"]["nginx"]
    assert app["container_name"] == "hyperpax-app"
    assert "ports" not in app and "network_mode" not in app
    assert app["networks"] == nginx["networks"] == ["internal"]
    assert config["networks"]["internal"] == {"driver": "bridge"}
    assert nginx["ports"] == ["80:80", "443:443"]
    assert nginx["depends_on"] == {"app": {"condition": "service_healthy"}}
    assert "healthcheck" not in app  # Do not disable the real image's probe.
    for service in (app, nginx):
        assert service["restart"] == "unless-stopped"
        assert service["logging"] == {
            "driver": "json-file", "options": {"max-size": "50m", "max-file": "5"}
        }
    assert app["deploy"]["resources"]["limits"] == {"memory": "2g", "cpus": "4.0"}
    dockerfile = source("apps/perps/Dockerfile")
    health = dockerfile.split("HEALTHCHECK ", 1)[1].replace("\\\n", " ")
    assert shlex.split(health) == [
        "--interval=15s", "--timeout=5s", "--start-period=20s", "--retries=3",
        "CMD", "wget", "-q", "-O", "/dev/null",
        "http://127.0.0.1:${PORT}/api/build-id", "||", "exit", "1",
    ]
    identity = source("apps/perps/src/pages/api/build-id.ts")
    assert "process.env.VERCEL_GIT_COMMIT_SHA" in identity
    assert "res.status(200).json({ buildId });" in identity
    architecture = section("Architecture", 2)
    nginx_examples = re.findall(r"```nginx\n(.*?)\n```", architecture, re.S)
    assert len(nginx_examples) == 1
    directives = [shlex.split(line.removesuffix(";")) for line in nginx_examples[0].splitlines()]
    assert ["resolver", "192.0.2.53", "valid=5s", "ipv6=off"] in directives
    assert ["set", "$perps_upstream", "app:" + app["environment"]["PORT"]] in directives
    assert ["proxy_pass", "http://$perps_upstream"] in directives
    assert ["proxy_set_header", "Host", "$host"] in directives
    assert ["proxy_set_header", "X-Forwarded-Proto", "$scheme"] in directives
    assert ["proxy_set_header", "X-Forwarded-For", "$proxy_add_x_forwarded_for"] in directives
    assert "./nginx/nginx.conf:/etc/nginx/nginx.conf:ro" in nginx["volumes"]
    assert "./nginx/conf.d:/etc/nginx/conf.d:ro" in nginx["volumes"]
    assert "an operator must supply" in readme()


def test_readme_start_upgrade_rollback_and_service_commands():
    assert commands(section("3. Build the frontend image")) == [COMPOSE_PREFIX + ["build", "app"]]
    assert commands(section("4. Start the stack")) == [
        COMPOSE_PREFIX + ["up", "-d", "--no-build", "--pull", "never", "--wait"]
    ]
    assert commands(section("Deploy a new version")) == [COMPOSE_PREFIX + RECREATE]
    rollback = section("Roll back")
    assert commands(rollback) == [COMPOSE_PREFIX + RECREATE]
    for fact in ("Restore the retained environment file", "BUILD_COMMIT_SHA",
                 "MARKET_PROXY_ORIGIN", "MARKET_PROXY_HOST", "do not rebuild",
                 "build-identity, proxy-origin and HTTPS checks"):
        assert fact in rollback
    assert commands(section("Tail logs")) == [COMPOSE_PREFIX + ["logs", "-f", "--tail", "100", "app"]]
    all_commands = commands(readme())
    compose_commands = [argv for argv in all_commands if argv[:2] == ["docker", "compose"]]
    assert len(compose_commands) == 10
    for argv in compose_commands:
        assert argv[:len(COMPOSE_PREFIX)] == COMPOSE_PREFIX
        assert "restart" not in argv and "run" not in argv
    assert not any(argv[:2] == ["docker", "run"] for argv in all_commands)
    assert "Shell variables override the env file" in readme()
    assert "changing them requires a rebuild" in readme().lower()
    assert "NEXT_PUBLIC_*" in readme()


def test_runtime_env_update_recreates_same_image_and_checks_proxy_origin():
    updates = section("Apply server-only environment changes")
    assert commands(updates) == [COMPOSE_PREFIX + RECREATE]
    for fact in ("MARKET_PROXY_ORIGIN", "MARKET_PROXY_HOST", "BUILD_COMMIT_SHA",
                 "unchanged", "**same image**", "new expected origin",
                 "restart", "retains the existing container environment",
                 "NEXT_PUBLIC_*", "require a rebuild"):
        assert fact in updates
    assert "it does not load edited values" in readme()
    checks = commands(section("5. Verify before routing traffic"))
    assert checks[0] == COMPOSE_PREFIX + ["ps"]
    assert checks[1] == COMPOSE_PREFIX + ["logs", "--tail", "100", "app"]
    assert checks[2] == COMPOSE_PREFIX + [
        "exec", "-T", "app", "wget", "-qO-", "http://127.0.0.1:3000/api/build-id"
    ]
    origin_check = checks[3]
    assert origin_check[:len(COMPOSE_PREFIX) + 5] == COMPOSE_PREFIX + ["exec", "-T", "app", "node", "-e"]
    assert len(origin_check) == len(COMPOSE_PREFIX) + 7
    script, expected = origin_check[-2:]
    assert 'process.env.MARKET_PROXY_ORIGIN !== process.argv[1]' in script
    assert 'throw new Error("Running app has the wrong MARKET_PROXY_ORIGIN")' in script
    assert 'fetch("http://127.0.0.1:3000/api/candle/config")' in script
    assert 'if (!response.ok) throw new Error(' in script
    assert 'await response.json()' in script
    assert '!Array.isArray(config.supported_resolutions)' in script
    assert '!config.supported_resolutions.length' in script
    assert 'process.exit(1)' in script
    url = urlsplit(expected)
    assert url.scheme == "https" and url.netloc == "perps.example.com"
    assert not url.path and not url.query and not url.fragment
    assert checks[4:] == [
        ["curl", "--fail-with-body", expected + "/api/build-id"],
        ["curl", "--fail-with-body", expected + "/api/candle/config"],
    ]
    assert "exact intended `MARKET_PROXY_ORIGIN`" in readme()
    assert "startup, environment updates\nand rollback" in readme()


def test_environment_template_matches_compose_lifecycle():
    template = (DEPLOY / ".env.production.example").read_text(encoding="utf-8")
    values = dict(
        line.split("=", 1) for line in template.splitlines()
        if line and not line.startswith("#")
    )
    for name in ("MARKET_PROXY_ORIGIN", "MARKET_PROXY_HOST", "BUILD_COMMIT_SHA",
                 "NEXT_PUBLIC_PERPS_GATEWAY_URL"):
        assert name in values
    assert values["MARKET_PROXY_ORIGIN"] == values["BUILD_COMMIT_SHA"] == ""
    assert values["MARKET_PROXY_HOST"] == "data.feed.chainflowtrading.com"
    assert "requires Compose app recreation, not a rebuild" in template
    assert "a fresh Compose app build and image replacement" in template
    assert "application root (apps/perps)" in template
    assert "Keep it unchanged for server-only" in template
    assert "deploy.sh" not in template
    assert "only requires a container restart" not in template


def test_repair_record_maps_both_findings_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["482"]
    assert task["base_revision"] == "5fc0c55eaaa6d43868c4a2872366299b8b7e2007"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_482.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "hf-done" in task["evidence"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "No tests are claimed to have failed" in task["baseline"]
    assert "apps/perps/Dockerfile" in task["wiring_requests"][0]
    assert "domain 3" in task["wiring_requests"][0]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.15.kvx"))
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    expected = {
        "compose-app-service-absent": ("apps/perps/deploy/docker-compose.yml", "config-drift"),
        "runtime-env-restart-instructions": ("apps/perps/deploy/README.md", "doc-drift"),
    }
    assert set(record["finding"]) == set(expected)
    covered = set()
    for finding_id, (path, kind) in expected.items():
        finding = record["finding"][finding_id]
        original = audit["finding"][finding_id]
        assert finding["audit"] == "22.30.15"
        assert finding["file"] == original["file"] == path
        assert finding["class"] == original["class"] == kind
        assert finding["severity"] == original["severity"] == "medium"
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert all(callable(globals()[name]) for name in finding["tests"])
        covered.update(finding["tests"])
    assert covered == declared
