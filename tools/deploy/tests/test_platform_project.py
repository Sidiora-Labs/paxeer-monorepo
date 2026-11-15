"""Recorded Railway requests for folder-based project reconciliation."""

import copy
import importlib.util
import io
import json
import re
import shutil
import threading
import tomllib
from contextlib import contextmanager, redirect_stderr, redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
FIXTURES = Path(__file__).parent / "fixtures/platform-project"
DIGEST = "sha256:" + "a" * 64
SECRETS = {
    "RAILWAY_API_TOKEN": 'example-api-token-"quoted"',
    "PLATFORM_STORE_POSTGRES_PASSWORD": 'example-db-password-"quoted"\\slash',
    "PLATFORM_WORKER_PROVIDER_KEY": "example-provider-key",
    "PLATFORM_WORKER_SIGNING_KEY": "example-chain-key",
    "PLATFORM_API_AUTH_SECRET": 'example-auth-secret-"quoted"\nsecond-line',
}
NAMES = ("store", "worker", "api")
INSTANCES = json.loads((FIXTURES / "instances.json").read_text())
VARIABLES = {
    "store": {"POSTGRES_DB": "platform", "POSTGRES_PASSWORD": SECRETS["PLATFORM_STORE_POSTGRES_PASSWORD"]},
    "worker": {
        "PORT": "8080", "DATABASE_URL": "${{store.DATABASE_URL}}",
        "PROVIDER_KEY": SECRETS["PLATFORM_WORKER_PROVIDER_KEY"],
        "SIGNING_KEY": SECRETS["PLATFORM_WORKER_SIGNING_KEY"],
    },
    "api": {
        "PORT": "8080", "AUTH_SECRET": SECRETS["PLATFORM_API_AUTH_SECRET"],
        "DATABASE_URL": "${{store.DATABASE_URL}}",
    },
}
VOLUMES = {"store": "/var/lib/postgresql/data", "api": "/cache"}


def connection(nodes):
    return {"edges": [{"node": node} for node in nodes]}


def entry(operation, variables, data):
    return {"op": operation, "variables": variables, "data": data}


class RecordedTransport:
    def __init__(self, entries):
        self.entries = copy.deepcopy(entries)
        self.calls = []

    def __call__(self, query, variables):
        match = re.match(r"\s*(query|mutation)\s+(\w+)\(", query)
        assert match, query
        kind, operation = match.groups()
        if operation == "Instance":
            assert not re.search(r"\b(?:branch|builder)\b", query)
            assert re.search(r"source\s*\{\s*image\s+repo\s*\}", query)
            assert re.search(r"latestDeployment\s*\{\s*id\s+status\s+meta\s*\}", query)
            for field in ("dockerfilePath", "rootDirectory", "watchPatterns", "healthcheckPath",
                          "healthcheckTimeout", "restartPolicyType", "numReplicas"):
                assert re.search(r"\b" + field + r"\b", query)
        if operation == "InstanceUpdate":
            assert not {"branch", "builder"} & variables["i"].keys()
            assert "repo" not in (variables["i"].get("source") or {})
        assert self.entries, f"unexpected request {operation}"
        expected = self.entries.pop(0)
        assert operation == expected["op"]
        assert variables == expected["variables"]
        self.calls.append((kind, operation, copy.deepcopy(variables), query))
        if "errors" in expected:
            raise RuntimeError("Railway GraphQL error: " + json.dumps(expected["errors"]))
        return copy.deepcopy(expected["data"])

    def exhausted(self):
        assert self.entries == []

    def mutations(self):
        return [operation for kind, operation, _, _ in self.calls if kind == "mutation"]


def project_node(names=NAMES, environment=True, project="paxeer-x-platform", env="production", volumes=None):
    volumes = VOLUMES if volumes is None else volumes
    return {
        "id": "p1", "name": project,
        "environments": connection([{"id": "e1", "name": env}] if environment else []),
        "services": connection([{"id": "s-" + name, "name": name} for name in names]),
        "volumes": connection([
            {"id": "v-" + name, "volumeInstances": connection([
                {"serviceId": "s-" + name, "environmentId": "e1", "mountPath": path}])}
            for name, path in volumes.items() if name in names
        ]),
    }


def projects_entry(node=None, after=None, next_cursor=None):
    return entry("Projects", {"w": "workspace-1", "a": after}, {
        "projects": {"pageInfo": {"hasNextPage": next_cursor is not None, "endCursor": next_cursor},
                     **connection([] if node is None else [node])}})


def read_entries(node=None, instances=None, variables=None, statuses=None):
    records = [projects_entry(node)]
    if node is None or not node["environments"]["edges"]:
        return records
    instances = INSTANCES if instances is None else instances
    variables = VARIABLES if variables is None else variables
    for name in NAMES:
        if name not in instances:
            continue
        instance = copy.deepcopy(instances[name])
        if "latestDeployment" not in instance:
            instance["latestDeployment"] = {"id": "dep-" + name, "status": "SUCCESS", "meta": {}}
        if name in (statuses or {}):
            instance["latestDeployment"]["status"] = statuses[name]
        records += [
            entry("Instance", {"s": "s-" + name, "e": "e1"}, {"serviceInstance": instance}),
            entry("Variables", {"p": "p1", "e": "e1", "s": "s-" + name}, {"variables": variables[name]}),
        ]
        if name == "api":
            records.append(entry("Domains", {"p": "p1", "e": "e1", "s": "s-api"},
                                 {"domains": {"serviceDomains": [{"domain": "api.up.railway.app"}]}}))
    return records


def deploy_entry(name, deployment=None):
    return entry("Deploy", {"s": "s-" + name, "e": "e1"}, {"serviceInstanceDeployV2": deployment or "dep-" + name})


def status_entry(name, status="SUCCESS", deployment=None):
    return entry("DeploymentStatus", {"id": deployment or "dep-" + name}, {"deployment": {"status": status}})


def connect_entry(branch="main"):
    return entry("ServiceConnect", {"s": "s-api", "i": {
        "repo": "sidiora-labs/paxeer-monorepo", "branch": branch}}, {"serviceConnect": {"id": "s-api"}})


def service_writes(name, branch="main", credentials=None, create=True):
    records = []
    want = copy.deepcopy(INSTANCES[name])
    want.pop("latestDeployment", None)
    source = want["source"]
    if name == "api":
        want.pop("source")
    if create:
        create_input = {"projectId": "p1", "name": name, "source": source}
        if name == "api":
            create_input["branch"] = branch
        records.append(entry("ServiceCreate", {"i": create_input}, {"serviceCreate": {"id": "s-" + name}}))
    if credentials and name == "worker":
        want["registryCredentials"] = credentials
    records.append(entry("InstanceUpdate", {"s": "s-" + name, "e": "e1", "i": want}, {"serviceInstanceUpdate": True}))
    if name == "api":
        records.append(connect_entry(branch))
    if name in VOLUMES:
        records.append(entry("VolumeCreate", {"i": {
            "projectId": "p1", "environmentId": "e1", "serviceId": "s-" + name, "mountPath": VOLUMES[name]}},
            {"volumeCreate": {"id": "v-" + name}}))
    records.append(entry("VariablesUpsert", {"i": {
        "projectId": "p1", "environmentId": "e1", "serviceId": "s-" + name,
        "variables": VARIABLES[name], "skipDeploys": True}}, {"variableCollectionUpsert": True}))
    if name == "api":
        records.append(entry("DomainCreate", {"i": {"serviceId": "s-api", "environmentId": "e1"}},
                             {"serviceDomainCreate": {"domain": "api.up.railway.app"}}))
    records.append(deploy_entry(name))
    return records


def creation_entries(branch="main", credentials=None, same_wave=False):
    records = [projects_entry(), entry("ProjectCreate", {"i": {
        "name": "paxeer-x-platform", "workspaceId": "workspace-1", "defaultEnvironmentName": "production"}},
        {"projectCreate": {"id": "p1", "environments": connection([{"id": "e1", "name": "production"}])}})]
    records.extend(service_writes("store"))
    records.append(status_entry("store"))
    if same_wave:
        records.extend(service_writes("api", branch=branch))
        records.extend(service_writes("worker", credentials=credentials))
        records.extend([status_entry("api"), status_entry("worker")])
    else:
        records.extend(service_writes("worker", credentials=credentials))
        records.append(status_entry("worker"))
        records.extend(service_writes("api", branch=branch))
        records.append(status_entry("api"))
    return records


@pytest.fixture
def client(tmp_path, monkeypatch):
    spec = importlib.util.spec_from_file_location("platform_project", ROOT / "tools/deploy/platform-project.py")
    tool = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(tool)
    folder = tmp_path / "platform"
    shutil.copytree(FIXTURES, folder)
    digest_file = tmp_path / "digests.json"
    digest_file.write_text(json.dumps({"platform-worker": DIGEST}))
    for name, value in SECRETS.items():
        monkeypatch.setenv(name, value)
    monkeypatch.setenv("RAILWAY_WORKSPACE_ID", "workspace-1")
    monkeypatch.setenv("ROLLOUT_TIMEOUT", "0")
    monkeypatch.setenv("ROLLOUT_POLL", "0")
    monkeypatch.delenv("GHCR_USERNAME", raising=False)
    monkeypatch.delenv("GHCR_TOKEN", raising=False)
    return tool, folder, digest_file


def invoke(client, records, mode="--apply", extra=()):
    tool, folder, digests = client
    transport = RecordedTransport(records)
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = tool.main([str(folder), mode, "--digests", str(digests), *extra], transport=transport)
    result = code, out.getvalue(), err.getvalue()
    assert_private(result[1] + result[2])
    return result, transport


def assert_private(output, secrets=()):
    for secret in (*SECRETS.values(), *secrets):
        for form in (secret, json.dumps(secret)[1:-1], json.dumps(json.dumps(secret)[1:-1])[1:-1]):
            assert form not in output


def replace(folder, filename, old, new):
    path = folder / filename
    text = path.read_text()
    assert old in text
    path.write_text(text.replace(old, new))


def test_plan_three_definition_kinds(client):
    result, transport = invoke(client, [projects_entry()], mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert "service 1 store database create\n" in result[1]
    assert "service 2 worker image create\n" in result[1]
    assert "service 3 api repo create\n" in result[1]
    assert "services=3\n" in result[1]
    for name in SECRETS:
        if name.startswith("PLATFORM_"):
            assert "env:" + name in result[1]
    assert '"8080"' not in result[1]
    assert "setting watchPatterns\n" in result[1]
    assert transport.mutations() == []
    transport.exhausted()


def test_apply_creates_then_second_apply_is_unchanged(client):
    result, first = invoke(client, creation_entries())
    assert result[0] == 0 and result[2] == ""
    assert result[1].index("wave 1 store SUCCESS") < result[1].index("applied worker ServiceCreate")
    assert result[1].index("wave 2 worker SUCCESS") < result[1].index("applied api ServiceCreate")
    first.exhausted()
    assert first.mutations().count("ProjectCreate") == 1
    assert first.mutations().count("ServiceCreate") == 3
    assert first.mutations().count("Deploy") == 3
    for _, operation, variables, query in first.calls:
        if operation == "InstanceUpdate":
            assert "serviceInstanceUpdate(serviceId: $s, environmentId: $e, input: $i)" in query
            assert "registryCredentials" not in variables["i"]
        if operation == "VariablesUpsert":
            assert "variableCollectionUpsert(input: $i)" in query
            assert variables["i"]["skipDeploys"] is True
    records = read_entries(project_node()) + [status_entry(name) for name in NAMES]
    result, second = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert result[1].count(" unchanged\n") == 3
    assert second.mutations() == []
    assert "wave 3 api SUCCESS" in result[1]
    second.exhausted()


def test_repo_branch_override_and_health_wave(client):
    _, folder, _ = client
    replace(folder, "api.kvx", "order = 3", "order = 2")
    result, transport = invoke(client, creation_entries(branch="release", same_wave=True), extra=("--branch", "release"))
    assert result[0] == 0 and result[2] == ""
    assert result[1].index("applied api Deploy") < result[1].index("applied worker Deploy")
    assert result[1].index("applied worker Deploy") < result[1].index("wave 2 api SUCCESS")
    assert result[1].index("wave 2 api SUCCESS") < result[1].index("wave 2 worker SUCCESS")
    transport.exhausted()


def test_repo_creation_sends_connection_and_instance_settings_separately(client):
    result, transport = invoke(client, creation_entries(branch="release"), extra=("--branch", "release"))
    assert result[0] == 0 and result[2] == ""
    creates = [variables["i"] for _, operation, variables, _ in transport.calls
               if operation == "ServiceCreate" and variables["i"]["name"] == "api"]
    assert creates == [{"projectId": "p1", "name": "api", "source": {
        "repo": "sidiora-labs/paxeer-monorepo"}, "branch": "release"}]
    updates = [variables["i"] for _, operation, variables, _ in transport.calls
               if operation == "InstanceUpdate" and variables["s"] == "s-api"]
    assert updates == [{key: value for key, value in INSTANCES["api"].items()
                        if key not in {"source", "latestDeployment"}}]
    assert transport.mutations().count("ServiceConnect") == 1
    for _, _, variables, query in transport.calls:
        assert not re.search(r"\bbuilder\b", query)
        assert '"builder"' not in json.dumps(variables)
    transport.exhausted()


@pytest.mark.parametrize("branch", ["release", "Main"])
def test_existing_repo_branch_change_connects_without_create(client, branch):
    instances = copy.deepcopy(INSTANCES)
    instances["api"]["latestDeployment"]["meta"]["branch"] = branch
    records = read_entries(project_node(), instances=instances)
    result, plan = invoke(client, records, mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("  connect sidiora-labs/paxeer-monorepo main\n") == 1
    assert "setting branch" not in result[1] and "setting source" not in result[1]
    assert plan.mutations() == []
    plan.exhausted()
    records += [status_entry("store"), status_entry("worker"), connect_entry(),
                deploy_entry("api"), status_entry("api")]
    result, applied = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert applied.mutations() == ["ServiceConnect", "Deploy"]
    connect = next(call for call in applied.calls if call[1] == "ServiceConnect")
    assert "serviceConnect(id: $s, input: $i)" in connect[3]
    assert connect[2] == {"s": "s-api", "i": {"repo": "sidiora-labs/paxeer-monorepo", "branch": "main"}}
    applied.exhausted()


def test_repo_name_case_keeps_connection_unchanged(client):
    instances = copy.deepcopy(INSTANCES)
    instances["api"]["source"]["repo"] = "Sidiora-Labs/Paxeer-Monorepo"
    records = read_entries(project_node(), instances=instances)
    result, plan = invoke(client, records, mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert result[1].count(" unchanged\n") == 3
    assert "  connect " not in result[1]
    assert plan.mutations() == []
    plan.exhausted()
    result, applied = invoke(client, records + [status_entry(name) for name in NAMES])
    assert result[0] == 0 and result[2] == ""
    assert applied.mutations() == []
    applied.exhausted()


@pytest.mark.parametrize("deployment", [
    None,
    {"id": "dep-api", "status": "SUCCESS", "meta": None},
    {"id": "dep-api", "status": "SUCCESS", "meta": {}},
])
def test_repo_without_deployment_branch_plans_and_applies_connect(client, deployment):
    instances = copy.deepcopy(INSTANCES)
    instances["api"]["latestDeployment"] = deployment
    records = read_entries(project_node(), instances=instances)
    result, plan = invoke(client, records, mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("  connect sidiora-labs/paxeer-monorepo main\n") == 1
    assert plan.mutations() == []
    plan.exhausted()
    records += [status_entry("store"), status_entry("worker"), connect_entry(),
                deploy_entry("api"), status_entry("api")]
    result, applied = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert applied.mutations() == ["ServiceConnect", "Deploy"]
    applied.exhausted()


@pytest.mark.parametrize("source", [None, {}, {"repo": None}, {"repo": "sidiora-labs/another-repository"}])
def test_repo_connection_is_set_when_repository_differs_or_is_absent(client, source):
    instances = copy.deepcopy(INSTANCES)
    instances["api"]["source"] = source
    records = read_entries(project_node(), instances=instances)
    result, plan = invoke(client, records, mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("  connect sidiora-labs/paxeer-monorepo main\n") == 1
    assert "setting source" not in result[1]
    assert plan.mutations() == []
    plan.exhausted()
    records += [status_entry("store"), status_entry("worker"), connect_entry(),
                deploy_entry("api"), status_entry("api")]
    result, applied = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert applied.mutations() == ["ServiceConnect", "Deploy"]
    applied.exhausted()


def test_apply_updates_only_differences(client, monkeypatch):
    _, folder, _ = client
    replace(folder, "worker.kvx", 'image = "platform-worker"', 'image = "platform-worker"\ncredentials = 1')
    username, password = "example-registry-user", 'example-registry-password-"quoted"'
    monkeypatch.setenv("GHCR_USERNAME", username)
    monkeypatch.setenv("GHCR_TOKEN", password)
    instances = copy.deepcopy(INSTANCES)
    instances["worker"]["source"]["image"] = "ghcr.io/sidiora-labs/platform-worker@sha256:" + "b" * 64
    instances["api"]["latestDeployment"]["meta"]["branch"] = "old"
    instances["api"].update(rootDirectory="/old", watchPatterns=["/old/**"],
                            dockerfilePath="old/Dockerfile", healthcheckPath="/old", numReplicas=2)
    variables = copy.deepcopy(VARIABLES)
    variables["store"]["POSTGRES_PASSWORD"] = "previous-database-password"
    variables["api"]["PORT"] = "9090"
    records = read_entries(project_node(volumes={"store": "/old", "api": "/cache"}), instances, variables)
    records += [
        entry("VolumeMount", {"v": "v-store", "e": "e1", "i": {"mountPath": VOLUMES["store"]}}, {"volumeInstanceUpdate": True}),
        entry("VariablesUpsert", {"i": {"projectId": "p1", "environmentId": "e1", "serviceId": "s-store",
              "variables": {"POSTGRES_PASSWORD": VARIABLES["store"]["POSTGRES_PASSWORD"]}, "skipDeploys": True}},
              {"variableCollectionUpsert": True}),
        deploy_entry("store"), status_entry("store"),
        entry("InstanceUpdate", {"s": "s-worker", "e": "e1", "i": {
            "source": INSTANCES["worker"]["source"], "registryCredentials": {"username": username, "password": password}}},
            {"serviceInstanceUpdate": True}),
        deploy_entry("worker"), status_entry("worker"),
        entry("InstanceUpdate", {"s": "s-api", "e": "e1", "i": {
            key: INSTANCES["api"][key] for key in ("rootDirectory", "watchPatterns", "dockerfilePath", "healthcheckPath", "numReplicas")}},
            {"serviceInstanceUpdate": True}),
        connect_entry(),
        entry("VariablesUpsert", {"i": {"projectId": "p1", "environmentId": "e1", "serviceId": "s-api",
              "variables": {"PORT": "8080"}, "skipDeploys": True}}, {"variableCollectionUpsert": True}),
        deploy_entry("api"), status_entry("api"),
    ]
    result, transport = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert "previous-database-password" not in result[1] + result[2]
    assert_private(result[1] + result[2], (username, password))
    assert "ServiceCreate" not in transport.mutations()
    assert "VolumeCreate" not in transport.mutations()
    assert "DomainCreate" not in transport.mutations()
    transport.exhausted()


def test_existing_service_is_reused_in_new_environment(client):
    node = project_node(names=("store",), environment=False, volumes={})
    records = [projects_entry(node), entry("EnvironmentCreate", {"i": {"projectId": "p1", "name": "production"}},
                                         {"environmentCreate": {"id": "e1"}})]
    for name in NAMES:
        records.extend(service_writes(name, create=name != "store"))
        records.append(status_entry(name))
    result, transport = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert transport.mutations().count("ServiceCreate") == 2
    transport.exhausted()


@pytest.mark.parametrize("filename,old,new,message", [
    ("api.kvx", 'kind = "repo"', 'kind = "repo"\nunknown = "value"', "unknown key unknown"),
    ("project.kvx", 'kind = "railway-project"', 'kind = "railway-project"\nunknown = 1', "unknown key unknown"),
    ("api.kvx", 'kind = "repo"', 'kind = "container"', "unknown kind"),
    ("api.kvx", 'needs = ["worker", "identity"]', 'needs = ["missing"]', "needs names unknown service missing"),
    ("api.kvx", "order = 3", "order = 1", "order is below needed service worker"),
    ("api.kvx", 'needs = ["worker", "identity"]', 'needs = ["archive"]', "order is below needed service archive"),
    ("api.kvx", 'PLATFORM_API_AUTH_SECRET = "operator"', "", "has no inputs row"),
    ("api.kvx", 'AUTH_SECRET = "env:PLATFORM_API_AUTH_SECRET"', 'AUTH_SECRET = "literal"', "has no use"),
    ("api.kvx", 'AUTH_SECRET = "env:PLATFORM_API_AUTH_SECRET"', 'AUTH_SECRET = "env:WRONG_NAME"',
     "env name must be PLATFORM_API_AUTH_SECRET"),
    ("api.kvx", 'PLATFORM_API_AUTH_SECRET = "operator"', 'PLATFORM_API_AUTH_SECRET = "secret"',
     "input must be generated, provider, chain or operator"),
    ("api.kvx", "replicas = 1", "replicas = 0", "replicas must be a positive integer"),
    ("api.kvx", 'watch = ["/docker/platform-gateway/**", "/docker/common/**", "/services/platform/**"]',
     'watch = "/docker/**"', "watch must be a list"),
    ("api.kvx", 'kind = "repo"', 'kind = "repo"\ntarget = "box"', "target must be railway or other"),
    ("api.kvx", 'kind = "repo"', 'kind = "repo"\nrailway_service = "worker"', "service name worker stated twice"),
])
def test_refuses_invalid_definition_with_file_and_row(client, filename, old, new, message):
    _, folder, _ = client
    replace(folder, filename, old, new)
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert re.search(re.escape(str(folder / filename)) + r":\d+:", result[2])
    assert message in result[2]
    assert transport.calls == []


def test_refuses_service_name_declared_in_two_files(client):
    _, folder, _ = client
    (folder / "z-duplicate.kvx").write_text((folder / "api.kvx").read_text())
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert f"{folder / 'z-duplicate.kvx'}:1:" in result[2]
    assert "service name api stated twice" in result[2]
    assert transport.calls == []


def test_definition_can_include_private_database_row(client):
    tool, folder, _ = client
    with (folder / "api.kvx").open("a") as stream:
        stream.write('\n[service.api-store]\nkind = "database"\nimage = "postgres:18"\norder = 1\n')
    _, rows = tool.load(folder)
    assert {row["name"] for row in rows} == {*NAMES, "api-store"}
    assert next(row for row in rows if row["name"] == "api-store")["kind"] == "database"


def test_plan_missing_input_and_apply_refuses_before_writes(client, monkeypatch):
    name = "PLATFORM_API_AUTH_SECRET"
    monkeypatch.delenv(name)
    for mode, code in (("--plan", 3), ("--apply", 2)):
        result, transport = invoke(client, [projects_entry()], mode=mode)
        assert result[0] == code
        assert "env:" + name + " missing" in result[1]
        assert name in result[2]
        assert "services=3" in result[1]
        assert transport.mutations() == []
        transport.exhausted()


def test_private_image_requires_credentials_only_when_applied(client, monkeypatch):
    _, folder, _ = client
    replace(folder, "worker.kvx", 'image = "platform-worker"', 'image = "platform-worker"\ncredentials = 1')
    result, transport = invoke(client, [projects_entry()], mode="--plan")
    assert result[0] == 0
    assert "registry GHCR_USERNAME GHCR_TOKEN" in result[1]
    transport.exhausted()
    result, transport = invoke(client, [projects_entry()])
    assert result[0] == 2
    assert "GHCR_USERNAME" in result[2] and "GHCR_TOKEN" in result[2]
    assert transport.mutations() == []
    transport.exhausted()
    credentials = {"username": "example-registry-user", "password": "example-registry-password"}
    monkeypatch.setenv("GHCR_USERNAME", credentials["username"])
    monkeypatch.setenv("GHCR_TOKEN", credentials["password"])
    result, transport = invoke(client, creation_entries(credentials=credentials))
    assert result[0] == 0 and result[2] == ""
    assert_private(result[1] + result[2], credentials.values())
    transport.exhausted()


@pytest.mark.parametrize("status", ["REPLACED", "REMOVED", "REMOVING", "SKIPPED"])
@pytest.mark.parametrize("latest_status,code", [("SUCCESS", 0), ("FAILED", 1), ("CRASHED", 1)])
def test_follows_replaced_deployment_of_same_service(client, status, latest_status, code):
    records = creation_entries()
    index = next(i for i, record in enumerate(records) if record["op"] == "DeploymentStatus")
    records[index:index + 1] = [status_entry("store", status), entry("LatestDeployment", {"s": "s-store", "e": "e1"}, {
        "serviceInstance": {"latestDeployment": {"id": "dep-store-new", "status": latest_status}}})]
    if code:
        records = records[:index + 2]
    result, transport = invoke(client, records)
    assert result[0] == code
    assert f"wave 1 store {latest_status}" in result[1]
    if code:
        assert f"deployment dep-store-new is {latest_status}" in result[2]
        assert "applied worker" not in result[1]
    else:
        assert result[2] == "" and "wave 3 api SUCCESS" in result[1]
    transport.exhausted()


def test_follows_replacement_while_it_is_building(client, monkeypatch):
    monkeypatch.setenv("ROLLOUT_TIMEOUT", "5")
    records = creation_entries()
    index = next(i for i, record in enumerate(records) if record["op"] == "DeploymentStatus")
    records[index:index + 1] = [
        status_entry("store", "REMOVED"),
        entry("LatestDeployment", {"s": "s-store", "e": "e1"}, {"serviceInstance": {
            "latestDeployment": {"id": "dep-store-new", "status": "BUILDING"}}}),
        status_entry("store", "BUILDING", "dep-store-new"), status_entry("store", "SUCCESS", "dep-store-new"),
    ]
    result, transport = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert "wave 3 api SUCCESS" in result[1]
    transport.exhausted()


@pytest.mark.parametrize("status", ["FAILED", "CRASHED", "SLEEPING", "NEEDS_APPROVAL", "UNKNOWN", "BUILDING"])
def test_failed_or_unready_wave_blocks_later_writes(client, status):
    records = creation_entries()
    index = next(i for i, record in enumerate(records) if record["op"] == "DeploymentStatus")
    records = records[:index] + [status_entry("store", status)]
    result, transport = invoke(client, records)
    assert result[0] == 1
    assert "wave 1 failed: store deployment dep-store" in result[2]
    assert "later waves were not applied" in result[2]
    assert "applied worker" not in result[1] and "applied api" not in result[1]
    if status == "UNKNOWN":
        assert "unknown status 'UNKNOWN'" in result[2]
    if status == "BUILDING":
        assert "TIMEOUT(last BUILDING)" in result[2]
    transport.exhausted()


def test_unchanged_failed_service_is_redeployed(client):
    records = read_entries(project_node(), statuses={"worker": "CRASHED"})
    records += [status_entry("store"), deploy_entry("worker"), status_entry("worker"), status_entry("api")]
    result, transport = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert transport.mutations() == ["Deploy"]
    assert "service 2 worker image update" in result[1]
    transport.exhausted()


def test_target_other_is_planned_and_never_queried_or_applied(client, monkeypatch):
    _, folder, _ = client
    replace(folder, "worker.kvx", 'kind = "image"', 'kind = "image"\ntarget = "other"\ncredentials = 1')
    for name in ("PLATFORM_WORKER_PROVIDER_KEY", "PLATFORM_WORKER_SIGNING_KEY"):
        monkeypatch.delenv(name)
    result, transport = invoke(client, [projects_entry()], mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert "service 2 worker image target=other" in result[1]
    transport.exhausted()
    instances = {name: INSTANCES[name] for name in ("store", "api")}
    records = read_entries(project_node(), instances=instances) + [status_entry("store"), status_entry("api")]
    result, transport = invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert transport.mutations() == []
    assert not [call for call in transport.calls if call[2].get("s") == "s-worker"]
    transport.exhausted()


def test_only_other_target_does_not_create_project(client):
    _, folder, _ = client
    for name in NAMES:
        replace(folder, name + ".kvx", f"[service.{name}]", f'[service.{name}]\ntarget = "other"')
    result, transport = invoke(client, [projects_entry()])
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("target=other") == 3
    assert transport.mutations() == []
    transport.exhausted()


def test_destination_overrides_keep_definition_input_names(client):
    records = creation_entries()
    records[1]["variables"]["i"].update(name="paxeer-x-preview", defaultEnvironmentName="preview")
    records[1]["data"]["projectCreate"]["environments"] = connection([{"id": "e1", "name": "preview"}])
    result, transport = invoke(client, records, extra=("--project", "paxeer-x-preview", "--environment", "preview"))
    assert result[0] == 0 and result[2] == ""
    assert "project=paxeer-x-preview env=preview" in result[1]
    assert "env:PLATFORM_API_AUTH_SECRET" in result[1]
    assert "env:PAXEER_X_PREVIEW_API_AUTH_SECRET" not in result[1]
    transport.exhausted()


def test_input_names_use_folder_name_and_project_name_is_refused(client):
    tool, folder, _ = client
    meta, services = tool.load(folder)
    api = next(service for service in services if service["name"] == "api")
    assert api["vars"]["AUTH_SECRET"] == "env:PLATFORM_API_AUTH_SECRET"
    replace(folder, "api.kvx", "env:PLATFORM_API_AUTH_SECRET", "env:PAXEER_X_PLATFORM_API_AUTH_SECRET")
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert "env name must be PLATFORM_API_AUTH_SECRET" in result[2]
    transport.exhausted()


def test_repo_row_inherits_project_repository_and_plan_shows_it(client):
    _, folder, _ = client
    replace(folder, "api.kvx", 'repo = "sidiora-labs/paxeer-monorepo"\n', "")
    (folder / "project.kvx").write_text(
        (folder / "project.kvx").read_text() + 'repo = "sidiora-labs/paxeer-monorepo"\n'
    )
    result, transport = invoke(client, [projects_entry()], mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert "repo sidiora-labs/paxeer-monorepo" in result[1]
    transport.exhausted()


def test_repo_row_without_project_repository_is_refused(client):
    _, folder, _ = client
    replace(folder, "api.kvx", 'repo = "sidiora-labs/paxeer-monorepo"\n', "")
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert re.search(re.escape(str(folder / "api.kvx")) + r":\d+:", result[2])
    assert "repo must be stated on the service or project" in result[2]
    transport.exhausted()


def test_other_target_requires_only_kind_and_order(client):
    _, folder, _ = client
    (folder / "other.kvx").write_text(
        "[service.other]\nkind = \"image\"\ntarget = \"other\"\norder = 4\n"
    )
    tool, _, _ = client
    _, services = tool.load(folder)
    other = next(service for service in services if service["name"] == "other")
    assert other["target"] == "other"


def test_other_target_rejects_non_path_non_tcp_health(client):
    _, folder, _ = client
    (folder / "other.kvx").write_text(
        "[service.other]\nkind = \"image\"\ntarget = \"other\"\n"
        "order = 4\nhealth = \"http://localhost/health\"\n"
    )
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert re.search(re.escape(str(folder / "other.kvx")) + r":\d+:", result[2])
    assert "health must be an HTTP path (or tcp:<port> for target other)" in result[2]
    transport.exhausted()


def test_optional_digest_map_with_complete_image_reference(client):
    tool, folder, _ = client
    replace(folder, "worker.kvx", 'image = "platform-worker"', 'image = "ghcr.io/sidiora-labs/platform-worker@' + DIGEST + '"')
    transport = RecordedTransport(creation_entries())
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = tool.main([str(folder), "--apply"], transport=transport)
    assert code == 0 and err.getvalue() == ""
    assert_private(out.getvalue())
    transport.exhausted()


def test_missing_digest_for_short_image_is_refused(client):
    _, _, digests = client
    digests.write_text("{}")
    result, transport = invoke(client, [], mode="--plan")
    assert result[0] == 2
    assert "digest map has no digest for: platform-worker" in result[2]
    assert transport.calls == []


def test_paginated_projects_and_duplicate_project_refusal(client):
    records = [projects_entry(next_cursor="page-2"), projects_entry(project_node(), after="page-2")]
    records.extend(read_entries(project_node())[1:])
    result, transport = invoke(client, records, mode="--plan")
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("unchanged") == 3
    transport.exhausted()
    duplicate = projects_entry(project_node())
    duplicate["data"]["projects"]["edges"].append({"node": dict(project_node(), id="p2")})
    result, transport = invoke(client, [duplicate])
    assert result[0] == 1
    assert "2 projects named paxeer-x-platform" in result[2]
    assert transport.mutations() == []
    transport.exhausted()


def test_read_only_wrapper_refuses_mutation(client):
    tool, _, _ = client
    transport = RecordedTransport([])
    with pytest.raises(RuntimeError, match="--plan refuses a write"):
        tool.read_only(transport)("mutation Deploy($s: String!) { deploy }", {"s": "s1"})
    assert transport.calls == []


def test_recorded_transport_file_drives_cli(client, tmp_path):
    tool, folder, digests = client
    recording = tmp_path / "responses.json"
    recording.write_text(json.dumps(creation_entries()))
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = tool.main([str(folder), "--apply", "--digests", str(digests), "--replay", str(recording)])
    assert code == 0 and err.getvalue() == ""
    assert "wave 3 api SUCCESS" in out.getvalue()
    assert_private(out.getvalue())


@contextmanager
def railway_http(records, error_operation=None, error_shape="messages", status=200):
    transport = RecordedTransport(records)
    failures = []
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/graphql/v2"
                assert self.headers["Authorization"] == "Bearer " + SECRETS["RAILWAY_API_TOKEN"]
                assert self.headers["Content-Type"] == "application/json"
                assert self.headers["User-Agent"] == "platform-project"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert set(request) == {"query", "variables"}
                requests.append(request)
                operation = re.match(r"\s*(?:query|mutation)\s+(\w+)", request["query"])[1]
                if operation == error_operation:
                    message = "request refused " + json.dumps(request["variables"]) + json.dumps(list(SECRETS.values()))
                    errors = [{"message": message}] if error_shape == "messages" else {"details": message}
                    body, response_status = {"errors": errors}, status
                else:
                    body, response_status = {"data": transport(request["query"], request["variables"])}, 200
                payload = json.dumps(body).encode()
            except Exception as error:
                failures.append(repr(error))
                payload, response_status = b'{}', 500
            self.send_response(response_status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/graphql/v2", transport, requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert not failures, failures


def test_real_http_transport_sends_and_consumes_recording(client, monkeypatch):
    tool, folder, digests = client
    with railway_http(creation_entries()) as (endpoint, transport, requests):
        monkeypatch.setattr(tool, "API", endpoint)
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = tool.main([str(folder), "--apply", "--digests", str(digests)])
        assert code == 0 and err.getvalue() == ""
        assert "wave 3 api SUCCESS" in out.getvalue()
        assert_private(out.getvalue())
        transport.exhausted()
        assert len(requests) == len(transport.calls)
        assert len(requests) > 10


def test_http_transport_identifies_platform_project_in_user_agent(client, monkeypatch):
    tool, _, _ = client
    with railway_http([projects_entry()]) as (endpoint, transport, requests):
        monkeypatch.setattr(tool, "API", endpoint)
        data = tool.http_transport(SECRETS["RAILWAY_API_TOKEN"])(
            "query Projects($w: String!, $a: String) { projects(workspaceId: $w, after: $a) { edges { node { id } } } }",
            {"w": "workspace-1", "a": None})
        assert data == projects_entry()["data"]
        assert len(requests) == 1
        transport.exhausted()


@pytest.mark.parametrize("operation", ["Projects", "InstanceUpdate", "VariablesUpsert"])
@pytest.mark.parametrize("shape,status", [("messages", 200), ("malformed", 200), ("messages", 400)])
def test_environment_values_are_absent_from_http_errors(client, monkeypatch, operation, shape, status):
    tool, folder, digests = client
    records = creation_entries()
    index = next(i for i, record in enumerate(records) if record["op"] == operation)
    with railway_http(records[:index], operation, shape, status) as (endpoint, transport, requests):
        monkeypatch.setattr(tool, "API", endpoint)
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = tool.main([str(folder), "--apply", "--digests", str(digests)])
        assert code == 1
        assert "Railway GraphQL error" in err.getvalue()
        assert "request refused" in err.getvalue()
        assert_private(out.getvalue() + err.getvalue())
        transport.exhausted()
        assert len(requests) == index + 1


def test_project_metadata_files_name_destinations():
    for name in ("platform", "wallet", "interop", "explorer", "sites"):
        text = (ROOT / "deploy/projects" / name / "project.kvx").read_text()
        meta = tomllib.loads(text)["meta"]
        assert meta["project"] == "paxeer-x-" + name
        assert meta["environment"] == "production"
