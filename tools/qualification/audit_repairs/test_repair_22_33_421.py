"""Offline source qualification for the explorer deployment repairs.

Parse the actual manifests and guarded commands, and drive the image check's
real configuration consumer in-process. Docker, Next.js and bash -n execution
are operator checks, not runtime coverage claimed by this Python-only gate.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/explorer/deploy"
RECORD = Path(__file__).with_name("repair_22_33_421.kvx")


class OverridePorts(list):
    """Preserve Compose's explicit replacement tag when parsing the override."""


class ComposeLoader(yaml.SafeLoader):
    pass


def override_ports(loader, node):
    return OverridePorts(loader.construct_sequence(node, deep=True))


ComposeLoader.add_constructor("!override", override_ports)


def manifest(name="docker-compose.local.yml"):
    return yaml.load((DEPLOY / name).read_text(), Loader=ComposeLoader)


def script(name):
    text = (DEPLOY / "tests" / name).read_text()
    assert text.startswith("#!/usr/bin/env bash\n")
    assert "set -euo pipefail\n" in text
    return text


def argv(command):
    return shlex.split(command.replace("\\\n", " "))


def test_local_database_exposure_default_is_loopback_and_internal_url_matches():
    services = manifest()["services"]
    database = services["db"]
    assert database["image"] == "postgres:16"
    assert database["ports"] == [
        "127.0.0.1:${POSTGRES_PUBLISHED_PORT:-7432}:5432"
    ]
    assert database["volumes"] == [
        "explorer-db-data:/var/lib/postgresql/data"
    ]
    assert database["environment"] == {
        "POSTGRES_DB": "${POSTGRES_DB:-blockscout}",
        "POSTGRES_USER": "${POSTGRES_USER:-blockscout}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD:-blockscout}",
    }
    assert services["backend"]["environment"]["DATABASE_URL"] == (
        "postgresql://${POSTGRES_USER:-blockscout}:"
        "${POSTGRES_PASSWORD:-blockscout}@db:5432/${POSTGRES_DB:-blockscout}"
    )
    assert services["backend"]["depends_on"]["db"] == {
        "condition": "service_healthy"
    }
    assert database["healthcheck"]["test"] == [
        "CMD-SHELL",
        'pg_isready -U "${POSTGRES_USER:-blockscout}" '
        '-d "${POSTGRES_DB:-blockscout}"',
    ]


def test_local_database_exposure_config_check_guards_the_default_bind():
    check = script("config-check.sh")
    assert 'compose="$root/deploy/docker-compose.local.yml"' in check
    block = re.search(
        r"if ! (sed -n .*?) \| \\\n  (grep .*?); then\n(.*?)\nfi",
        check,
        re.S,
    )
    assert block is not None
    producer, consumer, failure = block.groups()
    assert argv(producer) == [
        "sed", "-n", "/^  db:/,/^  [a-z]/p", "$compose"
    ]
    guard = argv(consumer)
    assert guard[:2] == ["grep", "-qFx"]
    assert len(guard) == 3
    assert guard[2] == (
        '      - "127.0.0.1:${POSTGRES_PUBLISHED_PORT:-7432}:5432"'
    )
    compose = (DEPLOY / "docker-compose.local.yml").read_text()
    database = re.search(r"^  db:\n(.*?)(?=^  [a-z])", compose, re.M | re.S)
    assert database is not None
    assert guard[2] in database.group().splitlines()
    assert 'echo "FAIL docker-compose.local.yml must bind PostgreSQL to loopback"' in failure
    assert re.search(r"^  fail=1$", failure, re.M)
    assert check.rstrip().endswith('exit "$fail"')


def test_local_database_exposure_remote_override_requires_matching_credentials():
    services = manifest("docker-compose.remote-db.yml")["services"]
    assert set(services) == {"db", "backend"}
    database = services["db"]
    ports = database["ports"]
    assert isinstance(ports, OverridePorts)
    assert len(ports) == 1
    assert ports[0] == {
        "target": 5432,
        "published": "${POSTGRES_PUBLISHED_PORT:-7432}",
        "host_ip": "${POSTGRES_BIND_ADDRESS:?Set the private interface for remote database access}",
        "protocol": "tcp",
    }
    credentials = {
        "POSTGRES_USER": "${POSTGRES_USER:?Set an operator-provided database user}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD:?Set an operator-provided database password}",
    }
    assert database["environment"] == credentials
    assert services["backend"] == {
        "environment": {
            "DATABASE_URL": (
                "postgresql://" + credentials["POSTGRES_USER"] + ":"
                + credentials["POSTGRES_PASSWORD"]
                + "@db:5432/${POSTGRES_DB:-blockscout}"
            )
        }
    }
    # :? rejects both unset and empty values; no credential has a fallback.
    for name, value in credentials.items():
        assert re.fullmatch(r"\$\{" + name + r":\?[^}]+\}", value)
        assert "blockscout" not in value


def test_local_database_exposure_remote_access_documents_safe_transition():
    readme = (DEPLOY / "README.md").read_text()
    for fact in (
        "127.0.0.1", "db:5432", "Compose 2.24.4 or later",
        "POSTGRES_BIND_ADDRESS", "POSTGRES_USER", "POSTGRES_PASSWORD",
        "strong, unique operator-provided password", "protected environment",
        "unset\nor empty values", "URL-safe credentials", "firewall",
        "VPN or SSH tunnel", "does not configure PostgreSQL TLS",
        "only to an empty data directory", "rotate the role credentials",
        "before enabling\nremote publication",
        "changing environment variables does not rotate existing",
        "Do not expose a volume still using the example credentials",
    ):
        assert fact in readme, fact
    command = re.search(r"```sh\n(docker compose .*?)\n```", readme, re.S)
    assert command is not None
    assert argv(command[1]) == [
        "docker", "compose", "-f",
        "services/explorer/deploy/docker-compose.local.yml", "-f",
        "services/explorer/deploy/docker-compose.remote-db.yml", "up", "-d",
    ]


def test_frontend_health_base_path_matches_built_route_and_response():
    frontend = manifest()["services"]["frontend"]
    assert frontend["healthcheck"]["test"][0] == "CMD-SHELL"
    command = argv(frontend["healthcheck"]["test"][1])
    assert command == [
        "wget", "-q", "-O", "/dev/null",
        "http://localhost:3000/explorer/api/healthz",
    ]
    config = (ROOT / "apps/explorer/next.config.js").read_text()
    base_paths = re.findall(r"^  basePath: '([^']+)',", config, re.M)
    assert base_paths == ["/explorer"]
    assert "output: 'standalone'" in config
    route = ROOT / "apps/explorer/pages/api/healthz.tsx"
    mounted_path = base_paths[0] + "/" + str(
        route.relative_to(ROOT / "apps/explorer/pages").with_suffix("")
    )
    assert command[-1] == "http://localhost:3000" + mounted_path
    assert frontend["environment"]["PORT"] == "3000"
    assert frontend["ports"] == ["${FRONTEND_PUBLISHED_PORT:-3000}:3000"]
    handler = route.read_text()
    assert re.search(
        r"export default function handler\(req: NextApiRequest, res: NextApiResponse\)"
        r"\s*\{\s*res\.status\(200\)\.json\('ok'\);\s*\}", handler,
    )
    dockerfile = (ROOT / "docker/explorer-frontend/Dockerfile").read_text()
    instructions = [
        argv(line) for line in dockerfile.replace("\\\n", " ").splitlines()
        if line and not line.startswith("#") and not line.startswith(" ")
    ]
    for instruction in (
        ["COPY", "apps/explorer", "."],
        ["RUN", "yarn", "build"],
        ["COPY", "--from=builder", "--chown=nextjs:nodejs",
         "/app/.next/standalone", "./"],
        ["ENV", "PORT=3000"],
    ):
        assert instruction in instructions
    assert json.loads(re.search(r"^CMD (\[.*\])$", dockerfile, re.M)[1]) == [
        "node", "server.js"
    ]
    assert frontend["image"] == (
        "ghcr.io/sidiora-labs/paxeer-x-explorer-frontend:${EXPLORER_IMAGE_TAG:-latest}"
    )


def image_settings_consumer():
    text = script("frontend-health-check.sh")
    code = re.search(r"python3 -c '\n(.*?)\n'\)", text, re.S)
    assert code is not None
    parsed = ast.parse(code[1])
    functions = [node for node in parsed.body if isinstance(node, ast.FunctionDef)]
    assert len(functions) == 1 and functions[0].name == "frontend_settings"
    scope = {}
    exec(compile(ast.Module(body=functions, type_ignores=[]), str(DEPLOY / "tests/frontend-health-check.sh"), "exec"), scope)
    return scope["frontend_settings"], code[1]


def test_frontend_health_base_path_drives_real_image_check_configuration_consumer():
    consume, code = image_settings_consumer()
    # The exact Compose service document crosses JSON into the production
    # image check's consumer. No replacement frontend or transport is used.
    config = manifest()
    decoded = json.loads(json.dumps(config))
    assert consume(decoded) == (
        config["services"]["frontend"]["image"],
        "wget -q -O /dev/null http://localhost:3000/explorer/api/healthz",
    )
    assert 'print(*frontend_settings(json.load(sys.stdin)), sep="\\n")' in code
    text = script("frontend-health-check.sh")
    assert 'docker compose --file "$deploy_dir/docker-compose.local.yml" config --format json | python3 -c' in text
    assert 'mapfile -t fields <<< "$settings"' in text
    assert '[ "${#fields[@]}" -eq 2 ]' in text
    assert 'image="${fields[0]}"' in text
    assert 'probe="${fields[1]}"' in text


@pytest.mark.parametrize("probe", [
    ["CMD", "wget", "http://localhost:3000/explorer/api/healthz"],
    ["CMD-SHELL"],
    ["CMD-SHELL", "wget", "extra-command"],
])
def test_frontend_health_base_path_image_check_refuses_incompatible_probe(probe):
    consume, _ = image_settings_consumer()
    config = manifest()
    config["services"]["frontend"]["healthcheck"]["test"] = probe
    with pytest.raises(AssertionError):
        consume(config)


def test_frontend_health_base_path_image_check_requires_unavailable_then_healthy():
    text = script("frontend-health-check.sh")
    inspect = text.index('docker image inspect "$image" > /dev/null')
    run = re.search(r'container="\$\((docker run .*?)\)"', text, re.S)
    assert run is not None
    assert argv(run[1]) == [
        "docker", "run", "--detach", "--pull=never", "--network", "none",
        "--env", "HOSTNAME=127.0.0.1", "--env", "PORT=3000",
        "--entrypoint", "sleep", "$image", "120",
    ]
    negative = re.search(
        r'if (docker exec .*?); then\n(.*?)\nfi', text, re.S
    )
    assert negative is not None
    assert argv(negative[1]) == [
        "docker", "exec", "$container", "sh", "-c", "$probe"
    ]
    assert 'echo "FAIL frontend probe succeeded without a server" >&2' in negative[2]
    assert "  exit 1" in negative[2]
    start = text.index('docker exec --detach "$container" node server.js')
    healthy = re.search(r"until (docker exec .*?); do\n(.*?)\ndone", text, re.S)
    assert healthy is not None
    assert argv(healthy[1]) == argv(negative[1])
    assert inspect < run.start() < negative.start() < start < healthy.start()
    assert 'deadline=$((SECONDS + 60))' in text[start:healthy.start()]
    assert 'if [ "$SECONDS" -ge "$deadline" ]; then' in healthy[2]
    assert 'docker logs "$container" >&2' in healthy[2]
    assert "    exit 1" in healthy[2]
    assert "  sleep 1" in healthy[2]
    assert 'trap cleanup EXIT' in text[:run.start()]
    assert 'if [ -n "$container" ]; then\n    docker rm --force "$container" > /dev/null' in text
    assert 'echo "frontend image health check: ok"' in text[healthy.end():]
    assert "docker pull" not in text and "docker build" not in text
    assert "|| true" not in text
    readme = (DEPLOY / "README.md").read_text()
    for command in (
        "bash -n services/explorer/deploy/tests/config-check.sh",
        "bash -n services/explorer/deploy/tests/frontend-health-check.sh",
        "bash services/explorer/deploy/tests/frontend-health-check.sh",
    ):
        assert command in readme


def test_repair_record_maps_each_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text())
    task = record["task"]["22"]["33"]["421"]
    assert task["audit"] == "22.30.145"
    assert task["source_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert set(record["finding"]) == {
        "local-database-exposure", "frontend-health-base-path"
    }
    for finding in record["finding"].values():
        assert finding["tests"]
        assert finding["file"] == "services/explorer/deploy/docker-compose.local.yml"
        for name in finding["tests"]:
            assert callable(globals()[name])
    assert "not run" in task["baseline"]
    assert "bash -n" in task["review_only"]
    assert "Docker" in task["review_only"]
