#!/usr/bin/env python3
"""Create and reconcile Railway projects from a folder of KVX definitions."""

import argparse
import json
import os
import re
import sys
import time
import tomllib
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROLLOUT = ROOT / "deploy/rollout.kvx"
API = "https://backboard.railway.com/graphql/v2"
GHCR = "ghcr.io/sidiora-labs"
NAME = r"[a-z0-9][a-z0-9-]*"
VARIABLE = r"[A-Z_][A-Z0-9_]*"
SECRET = re.compile(r"SECRET|TOKEN|PASSWORD|KEY|DSN|SEED|MNEMONIC|PRIVATE|SIGNING|CREDENTIAL|JWT")
INPUTS = {"generated", "provider", "chain", "operator"}
COMMON = {"kind", "railway_service", "order", "needs", "target", "volume", "public",
          "health", "restart", "replicas"}
KEYS = {
    "database": COMMON | {"image"},
    "image": COMMON | {"image", "credentials"},
    "repo": COMMON | {"repo", "dockerfile", "root", "watch"},
}
REQUIRED = {
    "database": {"kind", "image", "order"},
    "image": {"kind", "image", "order", "health", "restart", "replicas"},
    "repo": {"kind", "repo", "dockerfile", "root", "watch", "order", "health", "restart", "replicas"},
}
IN_PROGRESS = {"BUILDING", "DEPLOYING", "INITIALIZING", "QUEUED", "WAITING"}
REPLACED = {"REPLACED", "REMOVED", "REMOVING", "SKIPPED"}
FAILED = {"FAILED", "CRASHED"}
TERMINAL = {"SUCCESS", "SLEEPING", "NEEDS_APPROVAL"} | REPLACED | FAILED
MASKED = set()


class Malformed(ValueError):
    pass


class Document:
    def __init__(self, path):
        self.path = Path(path)
        self.sections = {}
        self.rows = {}
        section = None
        for row, raw in enumerate(self.path.read_text().splitlines(), 1):
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            head = re.fullmatch(r"\[([A-Za-z0-9_.-]+)\]\s*(?:#.*)?", line)
            if head:
                section = head[1]
                if section in self.sections:
                    raise Malformed(f"{self.path}:{row}: duplicate section [{section}]")
                self.sections[section] = {}
                self.rows[section, None] = row
                continue
            item = re.fullmatch(r"([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)", line)
            if not item or section is None:
                raise Malformed(f"{self.path}:{row}: expected a section or key = value row")
            key, value = item.groups()
            if key in self.sections[section]:
                raise Malformed(f"{self.path}:{row}: duplicate key {key} in [{section}]")
            self.rows[section, key] = row
            try:
                self.sections[section][key] = tomllib.loads("value = " + value)["value"]
            except tomllib.TOMLDecodeError:
                self.fail(section, key, f"invalid value for {key}")

    def fail(self, section, key, message):
        row = self.rows.get((section, key), self.rows.get((section, None), 1))
        raise Malformed(f"{self.path}:{row}: [{section}] {message}")


def input_name(project, service, variable):
    return re.sub(r"[^A-Z0-9_]", "_", f"{project}_{service}_{variable}".upper())


def load(folder, rollout=ROLLOUT):
    folder = Path(folder)
    project = Document(folder / "project.kvx")
    meta = project.sections.get("meta", {})
    for key in set(meta) - {"name", "kind", "project", "environment", "repo"}:
        project.fail("meta", key, f"unknown key {key}")
    for key in ("project", "environment"):
        if not isinstance(meta.get(key), str) or not re.fullmatch(NAME, meta[key]):
            project.fail("meta", key, f"{key} must be a lowercase name")
    if "kind" in meta and meta["kind"] != "railway-project":
        project.fail("meta", "kind", "kind must be railway-project")
    if "repo" in meta and (not isinstance(meta["repo"], str) or
                            not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", meta["repo"])):
        project.fail("meta", "repo", "repo must be owner/repository")
    documents = [project] + [Document(p) for p in sorted(folder.glob("*.kvx")) if p.name != "project.kvx"]
    services, variables, inputs = {}, {}, {}
    for doc in documents:
        for section, values in doc.sections.items():
            if section == "meta" and doc is project:
                continue
            match = re.fullmatch(r"(service|vars|inputs)\.(" + NAME + ")", section)
            if not match:
                doc.fail(section, None, "unknown section")
            group, name = match.groups()
            table = {"service": services, "vars": variables, "inputs": inputs}[group]
            if name in table:
                doc.fail(section, None, f"{group} name {name} stated twice")
            table[name] = (values, doc)
    for group, table in (("vars", variables), ("inputs", inputs)):
        for name, (_, doc) in table.items():
            if name not in services:
                doc.fail(f"{group}.{name}", None, "has no service row")
    external = Document(rollout).sections
    external = {key[8:]: value for key, value in external.items() if key.startswith("service.")}
    out, remote_names = [], set()
    for name, (values, doc) in services.items():
        section = f"service.{name}"
        kind = values.get("kind")
        if not isinstance(kind, str) or kind not in KEYS:
            doc.fail(section, "kind", "unknown kind; expected database, image or repo")
        for key in set(values) - KEYS[kind]:
            doc.fail(section, key, f"unknown key {key}")
        target = values.get("target", "railway")
        required = {"kind", "order"} if target == "other" else set(REQUIRED[kind])
        if kind == "repo":
            required.discard("repo")
        missing = required - set(values)
        if missing:
            doc.fail(section, None, f"missing keys {', '.join(sorted(missing))}")
        for key in (KEYS[kind] - {"order", "replicas", "public", "credentials", "needs", "watch"}) & set(values):
            if not isinstance(values[key], str) or not values[key]:
                doc.fail(section, key, f"{key} must be a non-empty string")
        for key in ("order", "replicas"):
            if key in values and (type(values[key]) is not int or values[key] < 1):
                doc.fail(section, key, f"{key} must be a positive integer")
        for key in ("public", "credentials"):
            if key in values and (type(values[key]) is not int or values[key] not in (0, 1)):
                doc.fail(section, key, f"{key} must be 0 or 1")
        if target not in ("railway", "other"):
            doc.fail(section, "target", "target must be railway or other")
        for key in ("watch", "needs"):
            if key in values and (not isinstance(values[key], list) or
                                  not all(isinstance(v, str) and v for v in values[key])):
                doc.fail(section, key, f"{key} must be a list of names or paths")
        if kind == "repo":
            repo = values.get("repo", meta.get("repo"))
            if repo is None and target != "other":
                doc.fail(section, "repo", "repo must be stated on the service or project")
            if repo is not None and not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
                doc.fail(section, "repo", "repo must be owner/repository")
            if "watch" in values and not values["watch"]:
                doc.fail(section, "watch", "watch must contain at least one pattern")
        if "restart" in values and values["restart"] not in ("ALWAYS", "ON_FAILURE", "NEVER"):
            doc.fail(section, "restart", "unknown restart policy")
        if "health" in values:
            pattern = r"/[A-Za-z0-9._/-]*"
            if values.get("target") == "other":
                pattern += r"|tcp:[0-9]{1,5}"
            if not re.fullmatch(pattern, values["health"]):
                doc.fail(section, "health", "health must be an HTTP path (or tcp:<port> for target other)")
        if "volume" in values and not values["volume"].startswith("/"):
            doc.fail(section, "volume", "volume must be an absolute mount path")
        remote = values.get("railway_service", name)
        if remote in remote_names or (remote != name and remote in services):
            doc.fail(section, "railway_service", f"service name {remote} stated twice")
        remote_names.add(remote)
        vars_, vars_doc = variables.get(name, ({}, doc))
        inputs_, inputs_doc = inputs.get(name, ({}, doc))
        used = set()
        for key, value in vars_.items():
            if not re.fullmatch(VARIABLE, key) or not isinstance(value, str):
                vars_doc.fail(f"vars.{name}", key, f"{key} must be an uppercase variable with a string value")
            if value.startswith("env:"):
                source = value[4:]
                expected = input_name(folder.name, name, key)
                if source != expected:
                    vars_doc.fail(f"vars.{name}", key, f"env name must be {expected}")
                if source not in inputs_:
                    vars_doc.fail(f"vars.{name}", key, f"env name {source} has no inputs row")
                used.add(source)
        for key, value in inputs_.items():
            if key not in used:
                inputs_doc.fail(f"inputs.{name}", key, f"inputs row {key} has no use")
            if not isinstance(value, str) or value not in INPUTS:
                inputs_doc.fail(f"inputs.{name}", key, "input must be generated, provider, chain or operator")
        resolved = dict(values)
        if kind == "repo" and "repo" not in resolved and "repo" in meta:
            resolved["repo"] = meta["repo"]
        out.append(dict(resolved, name=name, railway_service=remote, vars=vars_, inputs=inputs_))
    for svc in out:
        doc = services[svc["name"]][1]
        section = f"service.{svc['name']}"
        for dependency in svc.get("needs", []):
            needed = services[dependency][0] if dependency in services else external.get(dependency)
            if needed is None:
                doc.fail(section, "needs", f"needs names unknown service {dependency}")
            if svc["order"] < needed["order"]:
                doc.fail(section, "order", f"order is below needed service {dependency} ({needed['order']})")
    return dict(meta), sorted(out, key=lambda svc: (svc["order"], svc["name"]))


def mask(*values):
    MASKED.update(value for value in values if isinstance(value, str) and value)


def redact(value):
    text = str(value)
    for secret in sorted(MASKED, key=len, reverse=True):
        forms = {secret, json.dumps(secret)[1:-1], json.dumps(json.dumps(secret)[1:-1])[1:-1]}
        for form in sorted(forms, key=len, reverse=True):
            text = text.replace(form, "[redacted]")
    text = re.sub(r"(?i)\bbearer\s+[^\s\"']+", "Bearer [redacted]", text)
    return re.sub(r"(://[^/@\s:]*:)[^/@\s]*@", r"\1[redacted]@", text)


def say(message, error=False):
    print(redact(message), file=sys.stderr if error else sys.stdout)


def graphql_error(errors):
    if isinstance(errors, list) and errors and all(isinstance(e, dict) and isinstance(e.get("message"), str) for e in errors):
        text = "; ".join(e["message"] for e in errors)
    else:
        text = json.dumps(errors, default=str)
    return "Railway GraphQL error: " + redact(text)[:512]


def http_transport(token):
    mask(token)

    def call(query, variables):
        request = urllib.request.Request(
            API, data=json.dumps({"query": query, "variables": variables}).encode(),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {token}",
                     "User-Agent": "platform-project"})
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                body = json.load(response)
        except urllib.error.HTTPError as error:
            with error:
                try:
                    body = json.load(error)
                except ValueError:
                    raise RuntimeError(f"Railway HTTP {error.code} returned a response that is not JSON") from None
            raise RuntimeError(graphql_error(body)) from None
        except ValueError:
            raise RuntimeError("Railway returned a response that is not JSON") from None
        if not isinstance(body, dict) or body.get("errors") or not isinstance(body.get("data"), dict):
            raise RuntimeError(graphql_error(body.get("errors", body) if isinstance(body, dict) else body))
        return body["data"]

    return call


def replay_transport(path):
    entries = iter(json.loads(Path(path).read_text()))

    def call(query, variables):
        operation = re.match(r"\s*(?:query|mutation)\s+(\w+)", query)[1]
        entry = next(entries, None)
        if entry is None or entry["op"] != operation or entry.get("variables", {}) != variables:
            raise RuntimeError(f"recorded transport request differs for {operation}")
        if "errors" in entry:
            raise RuntimeError(graphql_error(entry["errors"]))
        return entry["data"]

    return call


def read_only(transport):
    def call(query, variables):
        if query.lstrip().startswith("mutation"):
            raise RuntimeError("--plan refuses a write")
        return transport(query, variables)

    return call


def edges(connection):
    return [edge["node"] for edge in (connection or {}).get("edges", [])]


def current_state(gql, meta, services):
    projects, after = [], None
    while True:
        page = gql(
            "query Projects($w: String!, $a: String) { projects(workspaceId: $w, first: 50, after: $a) { "
            "pageInfo { hasNextPage endCursor } edges { node { id name "
            "environments { edges { node { id name } } } services { edges { node { id name } } } "
            "volumes { edges { node { id volumeInstances { edges { node { serviceId environmentId mountPath } } } } } } "
            "} } } }", {"w": meta["workspace"], "a": after})["projects"]
        projects.extend(edges(page))
        if not page["pageInfo"]["hasNextPage"]:
            break
        cursor = page["pageInfo"]["endCursor"]
        if not cursor or cursor == after:
            raise RuntimeError("Railway project pagination did not advance")
        after = cursor
    named = [project for project in projects if project["name"] == meta["project"]]
    if len(named) > 1:
        raise RuntimeError(f"workspace has {len(named)} projects named {meta['project']}")
    project = named[0] if named else None
    state = {"project": project, "env": None, "services": {}}
    if not project:
        return state
    envs = [env for env in edges(project["environments"]) if env["name"] == meta["environment"]]
    if len(envs) > 1:
        raise RuntimeError(f"project has duplicate environments named {meta['environment']}")
    state["env"] = env = envs[0] if envs else None
    mounts = {}
    for volume in edges(project["volumes"]):
        for instance in edges(volume["volumeInstances"]):
            if env and instance["environmentId"] == env["id"]:
                mounts[instance["serviceId"]] = {"id": volume["id"], "path": instance["mountPath"]}
    for svc in services:
        if svc.get("target") == "other":
            continue
        named = [s for s in edges(project["services"]) if s["name"] == svc["railway_service"]]
        if len(named) > 1:
            raise RuntimeError(f"project has duplicate services named {svc['railway_service']}")
        if not named:
            continue
        sid = named[0]["id"]
        if not env:
            state["services"][svc["railway_service"]] = {
                "id": sid, "instance": {}, "variables": {}, "volume": None, "domains": []}
            continue
        instance = gql(
            "query Instance($s: String!, $e: String!) { serviceInstance(serviceId: $s, environmentId: $e) { "
            "source { image repo } dockerfilePath rootDirectory watchPatterns "
            "healthcheckPath healthcheckTimeout restartPolicyType numReplicas latestDeployment { id status meta } } }",
            {"s": sid, "e": env["id"]})["serviceInstance"] or {}
        variables = gql(
            "query Variables($p: String!, $e: String!, $s: String!) { "
            "variables(projectId: $p, environmentId: $e, serviceId: $s, unrendered: true) }",
            {"p": project["id"], "e": env["id"], "s": sid})["variables"] or {}
        mask(*(value for key, value in variables.items()
               if SECRET.search(key) or svc["vars"].get(key, "").startswith("env:")))
        domains = []
        if svc.get("public"):
            domains = (gql(
                "query Domains($p: String!, $e: String!, $s: String!) { "
                "domains(projectId: $p, environmentId: $e, serviceId: $s) { serviceDomains { domain } } }",
                {"p": project["id"], "e": env["id"], "s": sid})["domains"] or {}).get("serviceDomains") or []
        state["services"][svc["railway_service"]] = {
            "id": sid, "instance": instance, "variables": variables, "volume": mounts.get(sid), "domains": domains}
    return state


def load_digests(path):
    digests = json.loads(Path(path).read_text()) if path else {}
    if not isinstance(digests, dict):
        raise Malformed("digest map must be a JSON object")
    for name, digest in digests.items():
        if not isinstance(digest, str) or not re.fullmatch(r"sha256:[a-f0-9]{64}", digest):
            raise Malformed(f"digest for {name} must be sha256:<64hex>")
    return digests


def desired_instance(svc, digests):
    if svc["kind"] == "repo":
        want = {"dockerfilePath": svc["dockerfile"], "rootDirectory": svc["root"], "watchPatterns": svc["watch"]}
    else:
        image = svc["image"]
        if svc["kind"] == "image" and image in digests:
            image = f"{image if '/' in image else GHCR + '/' + image}@{digests[image]}"
        want = {"source": {"image": image}}
    for key, remote in (("health", "healthcheckPath"), ("restart", "restartPolicyType"), ("replicas", "numReplicas")):
        if key in svc:
            want[remote] = svc[key]
    if "health" in svc:
        want["healthcheckTimeout"] = 300
    return want


def plan_service(svc, cur, digests, branch):
    want = desired_instance(svc, digests)
    connection = {"repo": svc["repo"], "branch": branch} if svc["kind"] == "repo" else None
    have = (cur or {}).get("instance") or {}
    diff = {key: value for key, value in want.items() if have.get(key) != value}
    if "source" in diff and {key: (have.get("source") or {}).get(key) for key in want["source"]} == want["source"]:
        diff.pop("source")
    steps = [("create", svc["railway_service"])] if cur is None else []
    steps.extend(("setting", key) for key in sorted(diff))
    if connection:
        repository = (have.get("source") or {}).get("repo")
        metadata = (have.get("latestDeployment") or {}).get("meta") or {}
        if (repository is None or repository.casefold() != connection["repo"].casefold()
                or metadata.get("branch") != connection["branch"]):
            steps.append(("connect", f"{connection['repo']} {connection['branch']}"))
    if svc.get("credentials") and (cur is None or "source" in diff):
        steps.append(("registry", "GHCR_USERNAME GHCR_TOKEN"))
    volume = (cur or {}).get("volume")
    if svc.get("volume") and not volume:
        steps.append(("volume", svc["railway_service"]))
    elif svc.get("volume") and volume["path"] != svc["volume"]:
        steps.append(("mount", svc["railway_service"]))
    if svc.get("public") and not (cur or {}).get("domains"):
        steps.append(("domain", svc["railway_service"]))
    upsert, missing = {}, set()
    for key, declared in sorted(svc["vars"].items()):
        source = declared[4:] if declared.startswith("env:") else None
        value = os.environ.get(source) if source else declared
        if source and not value:
            missing.add(source)
            steps.append(("var", f"{key} env:{source} missing"))
        elif key not in (cur or {}).get("variables", {}) or cur["variables"][key] != value:
            upsert[key] = value
            steps.append(("var", f"{key}" + (f" env:{source}" if source else "")))
    if cur is not None and not steps:
        latest = have.get("latestDeployment") or {}
        if not latest.get("id") or latest.get("status") in FAILED | REPLACED:
            steps.append(("redeploy", svc["railway_service"]))
    return want, connection, diff, steps, upsert, missing


def apply_service(gql, svc, state, want, connection, diff, steps, upsert):
    project, env = state["project"]["id"], state["env"]["id"]
    cur = state["services"].get(svc["railway_service"])

    def write(query, variables):
        data = gql(query, variables)
        operation = re.match(r"mutation (\w+)", query)[1]
        say(f"applied {svc['railway_service']} {operation}")
        return data

    if cur:
        sid = cur["id"]
    else:
        create = {"projectId": project, "name": svc["railway_service"]}
        if connection:
            create.update(source={"repo": connection["repo"]}, branch=connection["branch"])
        else:
            create["source"] = want["source"]
        sid = write("mutation ServiceCreate($i: ServiceCreateInput!) { serviceCreate(input: $i) { id } }",
                    {"i": create})["serviceCreate"]["id"]
    ids = {"s": sid, "e": env}
    update = dict(want) if cur is None else dict(diff)
    kinds = {kind for kind, _ in steps}
    if "registry" in kinds:
        update["registryCredentials"] = {"username": os.environ["GHCR_USERNAME"], "password": os.environ["GHCR_TOKEN"]}
    if update:
        write("mutation InstanceUpdate($s: String!, $e: String!, $i: ServiceInstanceUpdateInput!) { "
              "serviceInstanceUpdate(serviceId: $s, environmentId: $e, input: $i) }", dict(ids, i=update))
    if "connect" in kinds:
        write("mutation ServiceConnect($s: String!, $i: ServiceConnectInput!) { "
              "serviceConnect(id: $s, input: $i) { id } }", {"s": sid, "i": connection})
    if "volume" in kinds:
        write("mutation VolumeCreate($i: VolumeCreateInput!) { volumeCreate(input: $i) { id } }",
              {"i": {"projectId": project, "environmentId": env, "serviceId": sid, "mountPath": svc["volume"]}})
    if "mount" in kinds:
        write("mutation VolumeMount($v: String!, $e: String!, $i: VolumeInstanceUpdateInput!) { "
              "volumeInstanceUpdate(volumeId: $v, environmentId: $e, input: $i) }",
              {"v": cur["volume"]["id"], "e": env, "i": {"mountPath": svc["volume"]}})
    if upsert:
        write("mutation VariablesUpsert($i: VariableCollectionUpsertInput!) { variableCollectionUpsert(input: $i) }",
              {"i": {"projectId": project, "environmentId": env, "serviceId": sid, "variables": upsert, "skipDeploys": True}})
    if "domain" in kinds:
        write("mutation DomainCreate($i: ServiceDomainCreateInput!) { serviceDomainCreate(input: $i) { domain } }",
              {"i": {"serviceId": sid, "environmentId": env}})
    if steps:
        deployment = write("mutation Deploy($s: String!, $e: String!) { serviceInstanceDeployV2(serviceId: $s, environmentId: $e) }",
                           ids)["serviceInstanceDeployV2"]
        return sid, deployment
    return sid, cur["instance"]["latestDeployment"]["id"]


def wait_deployment(gql, service, environment, deployment, timeout, poll):
    deadline = time.monotonic() + timeout
    while True:
        result = gql("query DeploymentStatus($id: String!) { deployment(id: $id) { status } }", {"id": deployment})["deployment"]
        status = result["status"] if result else "REMOVED"
        if status in REPLACED:
            latest = gql(
                "query LatestDeployment($s: String!, $e: String!) { serviceInstance(serviceId: $s, environmentId: $e) { "
                "latestDeployment { id status } } }", {"s": service, "e": environment})["serviceInstance"]
            latest = (latest or {}).get("latestDeployment") or {}
            if latest.get("id") and latest["id"] != deployment:
                deployment = latest["id"]
                if time.monotonic() < deadline:
                    continue
                status = latest["status"]
        if status not in IN_PROGRESS | REPLACED:
            return deployment, status if status in TERMINAL else f"unknown status {status!r}"
        if time.monotonic() >= deadline:
            return deployment, f"TIMEOUT(last {status})"
        time.sleep(poll)


def main(argv, transport=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("folder", type=Path)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--plan", action="store_true")
    modes.add_argument("--apply", action="store_true")
    parser.add_argument("--branch", default="main")
    parser.add_argument("--digests")
    parser.add_argument("--project")
    parser.add_argument("--environment")
    parser.add_argument("--replay")
    args = parser.parse_args(argv)
    MASKED.clear()
    mask(*(os.environ.get(key) for key in ("RAILWAY_API_TOKEN", "RAILWAY_WORKSPACE_ID", "GHCR_USERNAME", "GHCR_TOKEN")))
    try:
        meta, services = load(args.folder)
        for svc in services:
            mask(*(os.environ.get(value[4:]) for value in svc["vars"].values() if value.startswith("env:")))
        for key in ("project", "environment"):
            value = getattr(args, key)
            if value is not None:
                if not re.fullmatch(NAME, value):
                    raise Malformed(f"--{key} must be a lowercase name")
                meta[key] = value
        if not args.branch:
            raise Malformed("--branch must be non-empty")
        digests = load_digests(args.digests)
        missing_images = sorted({svc["image"] for svc in services
                                 if svc["kind"] == "image" and svc.get("target") != "other"
                                 and re.fullmatch(NAME, svc["image"]) and svc["image"] not in digests})
        if missing_images:
            raise Malformed("digest map has no digest for: " + ", ".join(missing_images))
        token = os.environ.get("RAILWAY_API_TOKEN")
        workspace = os.environ.get("RAILWAY_WORKSPACE_ID")
        for key, value in (("RAILWAY_API_TOKEN", token), ("RAILWAY_WORKSPACE_ID", workspace)):
            if not value:
                raise Malformed(f"{key} is not set")
        try:
            timeout = float(os.environ.get("ROLLOUT_TIMEOUT", "1800"))
            poll = float(os.environ.get("ROLLOUT_POLL", "10"))
        except ValueError:
            raise Malformed("ROLLOUT_TIMEOUT and ROLLOUT_POLL must be numbers") from None
        if not 0 <= timeout < float("inf") or not 0 <= poll < float("inf"):
            raise Malformed("ROLLOUT_TIMEOUT and ROLLOUT_POLL must be finite and non-negative")
        meta["workspace"] = workspace
    except (ValueError, OSError) as error:
        say(f"invalid project definition: {error}", error=True)
        return 2
    try:
        transport = transport if transport is not None else replay_transport(args.replay) if args.replay else http_transport(token)
        gql = read_only(transport) if args.plan else transport
        state = current_state(gql, meta, services)
        mode = "plan" if args.plan else "apply"
        say(f"platform-project {mode} project={meta['project']} env={meta['environment']}")
        say(f"project {meta['project']} {'exists' if state['project'] else 'create'}")
        say(f"environment {meta['environment']} {'exists' if state['env'] else 'create'}")
        rows, missing = [], set()
        for svc in services:
            if svc.get("target") == "other":
                say(f"service {svc['order']} {svc['railway_service']} {svc['kind']} target=other")
                continue
            cur = state["services"].get(svc["railway_service"])
            want, connection, diff, steps, upsert, unset = plan_service(svc, cur, digests, args.branch)
            missing.update(unset)
            status = "create" if cur is None else "update" if steps else "unchanged"
            say(f"service {svc['order']} {svc['railway_service']} {svc['kind']} {status}")
            if svc["kind"] == "repo":
                say(f"  repo {svc['repo']}")
            for kind, detail in steps:
                say(f"  {kind} {detail}")
            rows.append((svc, want, connection, diff, steps, upsert))
        say(f"services={len(services)}")
        if args.plan:
            if missing:
                say(f"plan has {len(missing)} unresolved inputs: {', '.join(sorted(missing))}", error=True)
                return 3
            return 0
        if any(svc.get("credentials") for svc, *_ in rows):
            missing.update(key for key in ("GHCR_USERNAME", "GHCR_TOKEN") if not os.environ.get(key))
        if missing:
            say(f"--apply needs these environment variables: {', '.join(sorted(missing))}", error=True)
            return 2
        if not rows:
            return 0
        if not state["project"]:
            created = gql("mutation ProjectCreate($i: ProjectCreateInput!) { projectCreate(input: $i) { "
                          "id environments { edges { node { id name } } } } }",
                          {"i": {"name": meta["project"], "workspaceId": workspace,
                                 "defaultEnvironmentName": meta["environment"]}})["projectCreate"]
            state["project"] = created
            state["env"] = next((env for env in edges(created["environments"]) if env["name"] == meta["environment"]), None)
        if not state["env"]:
            state["env"] = gql("mutation EnvironmentCreate($i: EnvironmentCreateInput!) { environmentCreate(input: $i) { id } }",
                               {"i": {"projectId": state["project"]["id"], "name": meta["environment"]}})["environmentCreate"]
        for order in sorted({svc["order"] for svc, *_ in rows}):
            wave = [(row[0], apply_service(gql, row[0], state, *row[1:])) for row in rows if row[0]["order"] == order]
            for svc, (sid, deployment) in wave:
                deployment, status = wait_deployment(gql, sid, state["env"]["id"], deployment, timeout, poll)
                say(f"wave {order} {svc['railway_service']} {status}")
                if status != "SUCCESS":
                    say(f"wave {order} failed: {svc['railway_service']} deployment {deployment} is {status}; "
                        "later waves were not applied", error=True)
                    return 1
    except (RuntimeError, OSError, ValueError, KeyError, TypeError) as error:
        say(f"railway request failed: {error}", error=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
