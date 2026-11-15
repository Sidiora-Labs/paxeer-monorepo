import ast
import configparser
import csv
import fnmatch
import re
import shlex
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
MANIFEST = ROOT / "deploy/projects/wallet/wallet-gateway.kvx"
CONTAINER = ROOT / "docker/wallet-gateway"
SOURCE = ROOT / "services/wallet/gateway/src"


def read_rows(path):
    parser = configparser.ConfigParser(interpolation=None)
    parser.optionxform = str
    parser.read_string(path.read_text())
    return {
        section: {key: ast.literal_eval(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def definition():
    rows = read_rows(MANIFEST)
    assert set(rows) == {
        "service.wallet-gateway",
        "vars.wallet-gateway",
        "inputs.wallet-gateway",
    }
    return tuple(
        rows[f"{section}.wallet-gateway"]
        for section in ("service", "vars", "inputs")
    )


def file_inputs():
    with (CONTAINER / "files.tsv").open() as stream:
        rows = list(csv.reader(stream, delimiter="\t"))
    assert rows
    assert all(len(row) == 4 for row in rows)
    assert len({row[0] for row in rows}) == len(rows)
    return rows


def schema_fields():
    text = (SOURCE / "env.ts").read_text()
    body = text.split("export const EnvSchema = z", 1)[1].split(".object({", 1)[1]
    body = body.split("\n  })", 1)[0]
    fields = list(re.finditer(r"(?m)^    ([A-Z][A-Z0-9_]*)(:|,)", body))
    assert fields
    expressions = {}
    for index, field in enumerate(fields):
        if field[2] == ":":
            end = (
                fields[index + 1].start()
                if index + 1 < len(fields)
                else len(body)
            )
            expression = body[field.end():end]
            expression = re.sub(r"//[^\n]*", "", expression)
        else:
            declaration = re.search(
                rf"export const {field[1]}\s*=\s*(.*?);", text, re.S
            )
            assert declaration, f"schema declaration missing: {field[1]}"
            expression = declaration[1]
        expressions[field[1]] = expression
    return expressions


def environment_reads():
    expressions = schema_fields()
    required = {
        name
        for name, expression in expressions.items()
        if not re.search(r"\.(?:optional|default|catch)\s*\(", expression)
    }
    direct = set()
    for path in SOURCE.rglob("*.ts"):
        text = path.read_text()
        direct.update(re.findall(r"process\.env\.([A-Z][A-Z0-9_]*)", text))
        direct.update(
            re.findall(r"process\.env\[['\"]([A-Z][A-Z0-9_]*)['\"]\]", text)
        )
    shell = (CONTAINER / "entrypoint.sh").read_text()
    shell_reads = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)(?=[:}])", shell))
    shell_required = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*):?\?", shell))
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    arguments = set(re.findall(r"(?m)^ARG ([A-Z][A-Z0-9_]*)", dockerfile))
    materialized = {row[0] for row in file_inputs()}
    return (
        set(expressions) | direct | shell_reads | arguments | materialized,
        required | direct | shell_required | arguments | materialized,
    )


def test_wallet_gateway_variables_follow_source_readers():
    _, variables, _ = definition()
    reads, required = environment_reads()
    assert required <= variables.keys(), (
        f"missing source variables: {sorted(required - variables.keys())}"
    )
    assert variables.keys() <= reads, (
        f"unread variables: {sorted(variables.keys() - reads)}"
    )
    assert all(isinstance(value, str) for value in variables.values())
    env_source = (SOURCE / "env.ts").read_text()
    for match in re.finditer(
        r"for \(const key of \[(.*?)\] as const\)", env_source, re.S
    ):
        keys = set(re.findall(r"['\"]([A-Z][A-Z0-9_]*)['\"]", match[1]))
        assert keys
        assert keys <= variables.keys(), (
            f"missing custody variables: {sorted(keys - variables.keys())}"
        )
        assert all(variables[key] for key in keys)
    assert variables["ATTESTOR_ENDPOINTS"].startswith("env:")
    assert variables["WALLET_GAS_STATION_URL"].startswith("env:")


def test_wallet_gateway_inputs_are_named_typed_and_documented():
    _, variables, inputs = definition()
    prefix = "_".join((MANIFEST.parent.name, MANIFEST.stem))
    prefix = prefix.replace("-", "_").upper()
    used = set()
    for name, value in variables.items():
        if value.startswith("env:"):
            external = value.removeprefix("env:")
            assert external == f"{prefix}_{name}"
            used.add(external)
        else:
            assert value.startswith(("${{", "/")) or name in {"NODE_ENV", "PORT"}, (
                f"deployment input must use env: or a service reference: {name}"
            )
    assert used == inputs.keys()
    assert set(inputs.values()) <= {"generated", "provider", "chain", "operator"}
    lines = MANIFEST.read_text().splitlines()
    for name in inputs:
        matches = [
            index for index, line in enumerate(lines)
            if line.startswith(f"{name} =")
        ]
        assert len(matches) == 1
        comment = lines[matches[0] - 1]
        assert comment.startswith("# ") and " as " in comment


def test_wallet_gateway_build_context_and_watch_patterns_cover_copies():
    service, _, _ = definition()
    context = (ROOT / service["root"]).resolve()
    assert context == ROOT
    dockerfile = context / service["dockerfile"]
    assert dockerfile == CONTAINER / "Dockerfile"
    assert dockerfile.is_file()
    patterns = service["watch"].split(",")
    assert len(patterns) == len(set(patterns))
    for pattern in patterns:
        assert not pattern.startswith("/") and ".." not in Path(pattern).parts
        assert list(context.glob(pattern)), f"watch pattern has no paths: {pattern}"

    def watched(path):
        relative = path.relative_to(context).as_posix()
        return any(fnmatch.fnmatchcase(relative, pattern) for pattern in patterns)

    for path in CONTAINER.iterdir():
        assert watched(path), f"container input is not watched: {path}"
    text = dockerfile.read_text().replace("\\\n", " ")
    for line in text.splitlines():
        if not line.startswith("COPY "):
            continue
        parts = shlex.split(line)[1:]
        if any(part.startswith("--from=") for part in parts):
            continue
        paths = [part for part in parts if not part.startswith("--")][:-1]
        assert paths
        for name in paths:
            path = context / name
            assert path.exists(), f"container copy input missing: {name}"
            copied = (
                [path] if path.is_file()
                else [p for p in path.rglob("*") if p.is_file()]
            )
            assert copied
            for item in copied:
                assert watched(item), f"container copy input is not watched: {item}"


def test_wallet_gateway_matches_rollout_and_database_dependency():
    service, variables, _ = definition()
    rollout = read_rows(ROOT / "deploy/rollout.kvx")
    row = rollout["service.wallet-gateway"]
    assert service["kind"] == "repo"
    assert row["target"] == "railway"
    assert service["health"] == row["health"]
    assert service["order"] == row["order"]
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert "volume" not in service
    references = {
        name
        for value in variables.values()
        for name in re.findall(r"\$\{\{([a-z0-9-]+)\.[A-Z0-9_]+\}\}", value)
    }
    assert references == set(service["needs"].split(",")) == {"wallet-postgres"}
    for name in references:
        dependency = rollout[f"service.{name}"]
        assert dependency["railway_service"] == name
        assert dependency["order"] < service["order"]
    assert variables["DATABASE_URL"] == "${{wallet-postgres.DATABASE_URL}}"
    assert "connectionString: env.DATABASE_URL" in (SOURCE / "db/pool.ts").read_text()


def test_wallet_gateway_port_health_and_entrypoint_use_declared_values():
    service, variables, _ = definition()
    index = (SOURCE / "index.ts").read_text()
    assert "app.listen({ port: env.PORT, host: '::' })" in index
    assert f"app.get('{service['health']}'," in index
    assert index.index("await runMigrations()") < index.index(
        f"app.get('{service['health']}',"
    )
    assert "PORT" in schema_fields()
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    exposed = re.findall(r"(?m)^EXPOSE (\d+)$", dockerfile)
    assert exposed == [variables["PORT"]]
    image_env = dict(
        re.findall(r"(?m)^ENV ([A-Z][A-Z0-9_]*)=(\S+)$", dockerfile)
    )
    assert image_env["PORT"] == variables["PORT"]
    assert variables["NODE_ENV"] == image_env["NODE_ENV"] == "production"
    assert (
        'ENTRYPOINT ["/usr/local/bin/layerx-env-files", '
        '"/usr/local/bin/wallet-entrypoint"]'
    ) in dockerfile
    assert 'CMD ["node", "/app/gateway/dist/index.js"]' in dockerfile
    entrypoint = (CONTAINER / "entrypoint.sh").read_text()
    assert 'exec gosu node:node "$@"' in entrypoint
    assert "chown node:node" in entrypoint and 'chmod 0600 "$key"' in entrypoint
    key_path = re.search(r"(?m)^key=(/\S+)$", entrypoint)
    assert key_path
    assert variables["WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE"] == key_path[1]


def test_wallet_gateway_materialized_files_match_runtime_paths():
    _, variables, _ = definition()
    expressions = schema_fields()
    rows = file_inputs()
    for name, path, mode, roles in rows:
        assert variables[name].startswith("env:")
        assert path.startswith("/run/secrets/")
        assert mode in {"", "0600"} and roles == "*"
    paths = {row[0]: row[1] for row in rows}
    configured_paths = {
        key: value for key, value in variables.items() if key.endswith("_FILE")
    }
    assert configured_paths
    for key, path in configured_paths.items():
        assert key in expressions
        name = key.removesuffix("_FILE")
        candidates = {name, name.replace("_PRIVATE_KEY", "_KEY")} & paths.keys()
        assert len(candidates) == 1, f"file input missing: {key}"
        assert paths[candidates.pop()] == path, f"file destination differs: {key}"
    common = (ROOT / "docker/common/env-files.sh").read_text()
    assert 'base64 -d >"$tmp"' in common
    assert 'unset "$name"' in common
    assert 'ENV LAYERX_FILES_OWNER=node:node' in (
        CONTAINER / "Dockerfile"
    ).read_text()


def test_wallet_gateway_build_arguments_preserve_source_identity():
    _, variables, inputs = definition()
    dockerfile = (CONTAINER / "Dockerfile").read_text()
    arguments = set(re.findall(r"(?m)^ARG ([A-Z][A-Z0-9_]*)", dockerfile))
    assert arguments <= variables.keys()
    assert variables["SOURCE_REVISION"] == "${{RAILWAY_GIT_COMMIT_SHA}}"
    for name in ("RAILWAY_GIT_COMMIT_SHA", "SOURCE_TREE"):
        assert variables[name].startswith("env:")
        assert inputs[variables[name].removeprefix("env:")] == "operator"
    assert 'test "${#SOURCE_REVISION}" -eq 40' in dockerfile
    assert 'test "${#SOURCE_TREE}" -eq 40' in dockerfile
    assert '*[!0-9a-f]*) exit 1' in dockerfile
