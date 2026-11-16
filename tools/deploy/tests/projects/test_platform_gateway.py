"""The platform gateway definition follows the native gateway's startup inputs."""

import ast
import fnmatch
import re
import tomllib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/gateway.kvx"
CONTAINER = ROOT / "docker/platform-gateway"
GATEWAY = ROOT / "services/platform/hosted/gateway"
ENV_NAME = re.compile(r"[A-Z][A-Z0-9_]*\Z")
RUST_STRING = re.compile(r'"(?:[^"\\]|\\.)*"')
RUST_TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|r(#+)?".*?"\1|"(?:[^"\\]|\\.)*"'
    r"|'(?:\\.|[^'\\\n])'",
    re.S,
)


def mask_rust(source):
    return RUST_TOKEN.sub(
        lambda match: "".join("\n" if c == "\n" else " " for c in match[0]),
        source,
    )


def block_end(masked, opening):
    depth = 0
    for position in range(opening, len(masked)):
        if masked[position] == "{":
            depth += 1
        elif masked[position] == "}":
            depth -= 1
            if depth == 0:
                return position + 1
    raise AssertionError("Rust block must be closed")


def production_source(path):
    source = path.read_text()
    masked = mask_rust(source)
    spans = []
    for match in re.finditer(
        r"#\[cfg\(test\)\]\s*(?:pub(?:\([^)]*\))?\s+)?"
        r"(?:mod|fn|struct|enum|use|type|const|static|impl)\b",
        masked,
    ):
        start = match.end()
        opening = masked.find("{", start)
        semicolon = masked.find(";", start)
        if semicolon >= 0 and (opening < 0 or semicolon < opening):
            end = semicolon + 1
        elif opening >= 0:
            end = block_end(masked, opening)
        else:
            raise AssertionError(f"unsupported test item in {path}")
        spans.append((match.start(), end))
    for start, end in reversed(spans):
        source = source[:start] + " " * (end - start) + source[end:]
    return source


def runtime_sources():
    return {path.stem: production_source(path) for path in (GATEWAY / "src").glob("*.rs")}


def environment_literals(source):
    return {
        match[0][1:-1]
        for match in RUST_STRING.finditer(source)
        if ENV_NAME.fullmatch(match[0][1:-1])
    }


def rust_functions(source):
    masked = mask_rust(source)
    functions = {}
    for match in re.finditer(r"\bfn\s+(\w+)\b[^{}]*\{", masked):
        end = block_end(masked, match.end() - 1)
        functions[match[1]] = source[match.start():end]
    return functions


def constant_environments(source):
    constants = {}
    for match in re.finditer(r"\bconst\s+(\w+)\s*:[^=]+=(.*?);", source, re.S):
        constants[match[1]] = environment_literals(match[2])
    return constants


def environment_reads(body, constants, aliases=None):
    reads = set()
    required = set()
    masked = mask_rust(body)
    aliases = {} if aliases is None else aliases
    calls = re.finditer(
        r'\b(?:env::var(?:_os)?|(?:super::)?read_secret)\s*\(\s*'
        r'("(?:[^"\\]|\\.)*"|\w+)\s*\)',
        body,
    )
    for call in calls:
        argument = call[1]
        if argument.startswith('"'):
            names = environment_literals(argument)
        elif argument in aliases:
            names = aliases[argument]
        elif argument in constants:
            names = constants[argument]
        else:
            names = environment_literals(body)
            for name, values in constants.items():
                if re.search(rf"\b{re.escape(name)}\b", masked):
                    names |= values
        reads |= names
        tail = body[call.end():]
        guarded = False
        for guard in re.finditer(
            rf"\bif\s+(?:std::)?env::var_os\(\s*{re.escape(argument)}\s*\)"
            r"\.is_some\(\)\s*\{",
            body,
        ):
            end = block_end(mask_rust(body), guard.end() - 1)
            if guard.end() <= call.start() < end:
                guarded = True
        if not guarded and (
            "read_secret" in call[0] or re.match(r"\s*\.map_err\b", tail)
        ):
            required |= names
    if re.search(r"\.map\((?:std::)?env::var(?:_os)?\)", body):
        for name, values in constants.items():
            if re.search(rf"\b{re.escape(name)}\b", masked):
                reads |= values
    return reads, required


def startup_requirements(sources, variables):
    functions = {
        (module, name): body
        for module, source in sources.items()
        for name, body in rust_functions(source).items()
    }
    pending = [("main", "run")]
    visited = set()
    required = set()
    while pending:
        key = pending.pop()
        if key in visited:
            continue
        visited.add(key)
        module, name = key
        body = functions[key]
        if key == ("main", "run"):
            body = re.split(
                r"\bfor\s+\w+\s+in\s+\w+\.incoming\(\)", body, maxsplit=1
            )[0]
        aliases = {}
        if key == ("main", "configured_service_identity"):
            pairs = {
                match[1]: (match[2], match[3])
                for match in re.finditer(
                    r'let (\w+) = \(\s*"([A-Z][A-Z0-9_]*)",\s*'
                    r'"([A-Z][A-Z0-9_]*)",?\s*\);',
                    body,
                )
            }
            selection = re.search(
                r"let \((\w+), (\w+)\) = if (\w+) \{ (\w+) \} else \{ (\w+) \};",
                body,
            )
            assert selection
            identity, password, flag, first, second = selection.groups()
            assert first in pairs and second in pairs
            assert re.search(
                rf"let {flag} = env::var_os\({first}\.0\)\.is_some\(\)"
                rf" \|\| env::var_os\({first}\.1\)\.is_some\(\);",
                body,
            )
            selected = (
                first if set(pairs[first]).intersection(variables) else second
            )
            aliases = dict(zip(
                (identity, password), ({name} for name in pairs[selected]),
            ))
        reads, needed = environment_reads(
            body, constant_environments(sources[module]), aliases,
        )
        # Optional groups return before loading their inputs when none are configured.
        if reads and not reads.intersection(variables) and (
            "Ok(None)" in body or (".all(" in body and "return Ok(())" in body)
        ):
            continue
        required |= needed
        for call in re.finditer(r"(?<![\w.])((?:\w+::)*)(\w+)\s*\(", mask_rust(body)):
            qualifiers = call[1].split("::")[:-1]
            target_module = module
            if qualifiers:
                known = [part for part in qualifiers if part in sources]
                if known:
                    target_module = known[-1]
                elif qualifiers[0] not in ("super", "Self", "self"):
                    continue
                elif qualifiers[0] == "super":
                    target_module = "main"
            target = (target_module, call[2])
            if target == ("main", "tls_config"):
                listener = next(iter(environment_reads(body, {})[0]), None)
                if name == "listener_config" and variables.get(listener) == "plain":
                    assert re.search(r'Ok\(mode\) if mode == "plain" =>', body)
                    assert "Ok(Listener::Plain)" in body
                    continue
            if target in functions:
                pending.append(target)
    return required


def definition():
    return tomllib.loads(DEFINITION.read_text())


def retired_roster_names():
    source = (ROOT / "tools/bringup/roster.go").read_text()
    listed = re.search(
        r"for _, name := range \[\]string\{([^}]*)\}", source, re.S
    )
    assert listed, "roster retired-name list is declared"
    names = set(re.findall(r'"([^"]+)"', listed[1]))
    names.update(re.findall(r'target == "([^"]+)"', source))
    return names


def string_values(value):
    if isinstance(value, dict):
        for nested in value.values():
            yield from string_values(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from string_values(nested)
    elif isinstance(value, str):
        yield value


def image_copies():
    copies = {}
    for line in (CONTAINER / "Dockerfile").read_text().splitlines():
        if line.startswith("COPY ") and "--from=" not in line:
            fields = [field for field in line.split()[1:] if not field.startswith("--")]
            for source in fields[:-1]:
                copies[fields[-1]] = source
    return copies


def selected_file_rows(variables):
    shim = (ROOT / "docker/common/env-files.sh").read_text()
    selector, default = re.search(r"table=\$\{(\w+):-([^}]+)\}", shim).groups()
    destination = variables.get(selector, default)
    table = ROOT / image_copies()[destination]
    rows = [
        line.split("\t")
        for line in table.read_text().splitlines()
        if line and not line.startswith("#")
    ]
    assert all(len(row) == 4 for row in rows)
    role_variable = re.search(r"role=\$\{(\w+):-\}", shim)[1]
    role = variables.get(role_variable, "")
    return [
        row for row in rows
        if "*" in row[3].split(",") or role in row[3].split(",")
    ]


def test_gateway_service_matches_container_and_rollout():
    data = definition()
    assert set(data) == {"service", "vars", "inputs"}
    assert set(data["service"]) == set(data["vars"]) == set(data["inputs"]) == {"gateway"}
    service = data["service"]["gateway"]
    assert set(service) == {
        "kind", "dockerfile", "root", "watch", "health", "restart", "replicas",
        "public", "order", "needs",
    }
    assert service["kind"] == "repo"
    assert ROOT / service["dockerfile"] == CONTAINER / "Dockerfile"
    assert (ROOT / service["dockerfile"]).is_file()
    assert service["root"] == "/"
    assert service["restart"] == "ALWAYS"
    deployment = (GATEWAY / "deployment.yaml").read_text()
    gateway_deployment = deployment.split("kind: Deployment", 1)[1]
    assert service["replicas"] == int(re.search(r"replicas: (\d+)", gateway_deployment)[1])
    assert service["public"] == 1
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"]
    assert rollout["gateway"]["railway_service"] == "gateway"
    assert service["order"] == rollout["gateway"]["order"]
    assert len(service["needs"]) == len(set(service["needs"]))
    for name in service["needs"]:
        assert name in rollout
        assert rollout[name]["order"] < service["order"]
    redis = data["vars"]["gateway"]["LAYERX_GATEWAY_REDIS_URL"]
    referenced = re.findall(r"\$\{\{([\w-]+)\.", redis)
    assert referenced and set(referenced) <= set(service["needs"])
    assert "volume" not in service


def test_gateway_watch_patterns_cover_build_inputs():
    service = definition()["service"]["gateway"]
    patterns = [pattern.lstrip("/") for pattern in service["watch"]]
    for pattern in patterns:
        assert list(ROOT.glob(pattern)), pattern
    def watched(path):
        return any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns)
    for folder in (CONTAINER, ROOT / "docker/common"):
        for path in folder.rglob("*"):
            if path.is_file():
                assert watched(path.relative_to(ROOT).as_posix()), path
    for source in image_copies().values():
        assert (ROOT / source).exists(), source
        if source != ".":
            assert watched(source), source
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    manifest = re.search(r"--manifest-path (\S+)", dockerfile)[1]
    assert watched(manifest)
    workspace = tomllib.loads((ROOT / manifest).read_text())["workspace"]
    pending = [ROOT / manifest]
    for member in workspace["members"]:
        pending.append((ROOT / manifest).parent / member / "Cargo.toml")
    visited = set()
    while pending:
        path = pending.pop().resolve()
        if path in visited:
            continue
        visited.add(path)
        assert path.is_file(), path
        assert watched(path.relative_to(ROOT).as_posix()), path
        cargo = tomllib.loads(path.read_text())
        groups = [cargo.get("dependencies", {}), cargo.get("build-dependencies", {})]
        groups.append(cargo.get("workspace", {}).get("dependencies", {}))
        for group in groups:
            for dependency in group.values():
                if isinstance(dependency, dict) and "path" in dependency:
                    pending.append(path.parent / dependency["path"] / "Cargo.toml")


def test_gateway_variables_follow_source_readers():
    variables = definition()["vars"]["gateway"]
    sources = runtime_sources()
    allowed = set()
    for source in sources.values():
        constants = constant_environments(source)
        for body in rust_functions(source).values():
            allowed |= environment_reads(body, constants)[0]
    shim = (ROOT / "docker/common/env-files.sh").read_text()
    allowed |= set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)[:-]", shim))
    rows = selected_file_rows(variables)
    allowed |= {row[0] for row in rows}
    arguments = set(re.findall(r"(?m)^ARG\s+(\w+)", (CONTAINER / "Dockerfile").read_text()))
    allowed |= arguments
    assert set(variables) <= allowed, sorted(set(variables) - allowed)
    required = startup_requirements(sources, variables) | arguments | {row[0] for row in rows}
    assert required <= set(variables), sorted(required - set(variables))
    assert all(ENV_NAME.fullmatch(name) for name in variables)
    assert all(isinstance(value, str) and value for value in variables.values())


def test_gateway_inputs_have_names_categories_and_formats():
    data = definition()
    used = set()
    for variable, value in data["vars"]["gateway"].items():
        if value.startswith("env:"):
            name = value.removeprefix("env:")
            assert name == f"PLATFORM_GATEWAY_{variable}"
            used.add(name)
        else:
            # Coordinates use service references; operator values use inputs.
            assert not re.search(r"(?:https?|rediss?)://(?!\$\{\{)", value)
    inputs = data["inputs"]["gateway"]
    assert set(inputs) == used
    assert set(inputs.values()) <= {"generated", "provider", "chain", "operator"}
    lines = DEFINITION.read_text().splitlines()
    for name in inputs:
        assert ENV_NAME.fullmatch(name)
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6


def test_gateway_identity_variables_follow_source_readers():
    variables = definition()["vars"]["gateway"]
    sources = runtime_sources()
    groups = {
        ("main", "configured_identity"): {
            "LAYERX_GATEWAY_IDENTITY_URL",
            "LAYERX_GATEWAY_IDENTITY_TOKEN_FILE",
            "LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE",
        },
        ("main", "configured_service_identity"): {
            "LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12",
            "LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE",
        },
        ("rpc_register", "configured_token"): {
            "LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE",
        },
        ("rpc_register", "configured_tenant"): {"LAYERX_GATEWAY_TENANT"},
    }
    for (module, function), expected in groups.items():
        body = rust_functions(sources[module])[function]
        reads, _ = environment_reads(body, constant_environments(sources[module]))
        assert expected <= reads
        assert expected <= variables.keys()
    identity = rust_functions(sources["main"])["configured_identity"]
    _, required = environment_reads(identity, constant_environments(sources["main"]))
    assert required == {"LAYERX_GATEWAY_IDENTITY_TOKEN_FILE"}
    assert variables["LAYERX_ROLE"] == "identity"
    for name in ("LAYERX_GATEWAY_IDENTITY_URL", "LAYERX_GATEWAY_TENANT"):
        assert variables[name] == f"env:PLATFORM_GATEWAY_{name}"
    paths = {
        "LAYERX_GATEWAY_IDENTITY_TOKEN_FILE": "/run/secrets/identity-token",
        "LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE": "/run/secrets/identity-provisioning-token",
        "LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE": "/run/secrets/key-provisioning-key",
        "LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12": "/run/secrets/client.p12",
        "LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE": "/run/secrets/client-password",
    }
    for name, path in paths.items():
        assert variables[name] == path


def test_gateway_identity_inputs_are_categorized_and_documented():
    inputs = definition()["inputs"]["gateway"]
    expected = {
        "LAYERX_GATEWAY_IDENTITY_URL": (
            "operator", ("absolute HTTPS", "identity service"),
        ),
        "LAYERX_GATEWAY_TENANT": (
            "operator",
            ("1 to 95", "lowercase", "digits", "dots", "underscores", "hyphens"),
        ),
        "ENDPOINT_IDENTITY_TOKEN": (
            "operator", ("base64-encoded token text", "issued by the identity service"),
        ),
        "ENDPOINT_IDENTITY_PROVISIONING_TOKEN": (
            "operator", ("base64-encoded token text", "issued by the identity service"),
        ),
        "ENDPOINT_KEY_PROVISIONING_KEY": (
            "generated",
            (
                "base64-encoded", "64 hexadecimal characters",
                "generated once", "equal on every replica",
            ),
        ),
        "ENDPOINT_CLIENT_P12": (
            "generated", ("Base64-encoded", "PKCS#12", "written by the launcher"),
        ),
        "ENDPOINT_CLIENT_PASSWORD": (
            "generated", ("Base64-encoded", "password", "written by the launcher"),
        ),
    }
    lines = DEFINITION.read_text().splitlines()
    for variable, (category, descriptions) in expected.items():
        name = f"PLATFORM_GATEWAY_{variable}"
        assert inputs[name] == category
        index = next(
            i for i, line in enumerate(lines) if line.startswith(name + " =")
        )
        comment = lines[index - 1]
        assert comment.startswith("# ")
        assert all(description in comment for description in descriptions)


def test_gateway_port_listener_and_health_are_served():
    data = definition()
    variables = data["vars"]["gateway"]
    main = production_source(GATEWAY / "src/main.rs")
    port = re.search(r'\.or_else\(\|_\| env::var\("(\w+)"\)\.map\(\|port\| format!\("\[::\]:\{port\}"\)\)\)', main)
    assert port, "gateway must bind the hosting port on all interfaces"
    assert 0 < int(variables[port[1]]) <= 65535
    listen = re.search(r'listen: env::var\("(\w+)"\)', main)[1]
    assert listen not in variables
    listener = rust_functions(main)["listener_config"]
    mode = re.search(r'env::var\("(\w+)"\)', listener)[1]
    assert variables[mode] == "plain"
    assert 'Ok(mode) if mode == "plain" =>' in listener
    certificates = re.search(r"const LISTENER_CERTIFICATE_VARIABLES[^=]*=(.*?);", main, re.S)[1]
    assert not environment_literals(certificates).intersection(variables)
    health = data["service"]["gateway"]["health"]
    route = rust_functions(main)["route_with_peer"]
    assert re.search(rf'if request.method == "GET" && request.path == "{re.escape(health)}"', route)
    health_body = route.split(f'request.path == "{health}"', 1)[1].split(";", 1)[0]
    assert re.search(r"return json_response\(\s*200,", health_body)


def test_gateway_file_inputs_are_materialized_and_staged():
    variables = definition()["vars"]["gateway"]
    rows = selected_file_rows(variables)
    assert len(rows) == 9
    assert {row[0]: row[1] for row in rows if row[3] == "identity"} == {
        "ENDPOINT_IDENTITY_TOKEN": "/run/secrets/identity-token",
        "ENDPOINT_IDENTITY_PROVISIONING_TOKEN": "/run/secrets/identity-provisioning-token",
        "ENDPOINT_KEY_PROVISIONING_KEY": "/run/secrets/key-provisioning-key",
        "ENDPOINT_CLIENT_P12": "/run/secrets/client.p12",
        "ENDPOINT_CLIENT_PASSWORD": "/run/secrets/client-password",
    }
    entry = (CONTAINER / "entrypoint.sh").read_text().split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
    tree = ast.parse(entry)
    staged = set()
    for node in tree.body:
        if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == "FILE_VARIABLES" for target in node.targets):
            staged.update(ast.literal_eval(node.value))
    assert staged
    destinations = set()
    for row in rows:
        assert len(row) == 4
        name, destination, mode, roles = row
        assert name in variables
        assert destination.startswith("/run/secrets/")
        assert mode == "0600"
        assert roles == (
            "identity" if name in {
                "ENDPOINT_IDENTITY_TOKEN", "ENDPOINT_IDENTITY_PROVISIONING_TOKEN",
                "ENDPOINT_KEY_PROVISIONING_KEY",
                "ENDPOINT_CLIENT_P12", "ENDPOINT_CLIENT_PASSWORD",
            } else "*"
        )
        assert destination not in destinations
        destinations.add(destination)
        consumers = {name for name, value in variables.items() if value == destination}
        assert consumers and consumers <= staged, consumers - staged
    for name, value in variables.items():
        if value.startswith("/run/secrets/"):
            assert value in destinations, name
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    ignore = (CONTAINER / "Dockerfile.dockerignore").read_text().splitlines()
    for source in image_copies().values():
        if source.startswith("docker/"):
            assert "!" + source in ignore
    assert 'ENTRYPOINT ["/usr/local/bin/layerx-env-files", "/usr/local/bin/layerx-gateway-entrypoint"]' in dockerfile
    assert 'environment[variable] = copy_file(environment[variable])' in entry
    assert '"4020:4020"' in entry
    assert 'stat.S_IMODE(before.st_mode) == 0o600' in entry
    assert 'os.fchown(output.fileno(), UID, UID)' in entry


def test_gateway_without_role_selects_only_chain_files():
    variables = dict(definition()["vars"]["gateway"])
    del variables["LAYERX_ROLE"]
    assert selected_file_rows(variables) == [
        ["ENDPOINT_OUTBOUND_CA", "/run/secrets/ca.crt.der", "0600", "*"],
        ["ENDPOINT_REDIS_USERNAME", "/run/secrets/redis-username", "0600", "*"],
        ["ENDPOINT_REDIS_PASSWORD", "/run/secrets/redis-password", "0600", "*"],
        ["ENDPOINT_RECEIPT_KEYS", "/run/secrets/receipt-keys.json", "0600", "*"],
    ]


def test_gateway_wiring_avoids_retired_roster_names():
    retired = retired_roster_names()
    for path in sorted((ROOT / "deploy/projects/platform").glob("*.kvx")):
        data = tomllib.loads(path.read_text())
        for value in string_values(data):
            for name in retired:
                assert not re.search(
                    rf"(?<![A-Za-z0-9_-]){re.escape(name)}(?![A-Za-z0-9_-])",
                    value,
                ), f"{path}: {name}"
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())
    for value in string_values(rollout["service"]):
        for name in retired:
            assert not re.search(
                rf"(?<![A-Za-z0-9_-]){re.escape(name)}(?![A-Za-z0-9_-])",
                value,
            ), f"deploy/rollout.kvx: {name}"
