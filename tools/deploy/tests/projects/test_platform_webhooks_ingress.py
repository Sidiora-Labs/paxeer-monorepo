import configparser
import fnmatch
import json
import re
import shlex
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/webhooks-ingress.kvx"
CONTAINER = ROOT / "docker/platform-webhooks"
SOURCE = ROOT / "services/platform/hosted/webhooks"
SERVICE = DEFINITION.stem


def load_rows(path):
    parser = configparser.ConfigParser(interpolation=None, strict=True)
    parser.optionxform = str
    parser.read_string(path.read_text())
    return {
        section: {key: json.loads(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def rust_mask(source):
    tokens = r'//[^\n]*|/\*[\s\S]*?\*/|r(#+)?"[\s\S]*?"\1|"(?:\\.|[^"\\])*"'
    return re.sub(tokens, lambda match: " " * len(match[0]), source)


def block_end(mask, start):
    assert mask[start] == "{"
    depth = 1
    for position in range(start + 1, len(mask)):
        depth += (mask[position] == "{") - (mask[position] == "}")
        if depth == 0:
            return position + 1
    raise AssertionError("Rust configuration block must be balanced")


def rust_functions():
    functions = {}
    for path in (SOURCE / "src").glob("*.rs"):
        if path.stem.endswith("_support"):
            continue
        source = path.read_text()
        mask = rust_mask(source)
        owners = []
        for match in re.finditer(r"\bimpl\s+(\w+)\s*\{", mask):
            start = mask.index("{", match.start())
            owners.append((start, block_end(mask, start), match[1]))
        for match in re.finditer(r"\bfn\s+(\w+)\s*\(", mask):
            start = mask.index("{", match.end())
            end = block_end(mask, start)
            owner = next(
                (name for left, right, name in owners if left < start < right),
                path.stem,
            )
            functions[(owner, match[1])] = source[match.start():end]
    return functions


def ingress_body(body):
    mask = rust_mask(body)
    match = re.search(r"\bif ingress\s*\{", mask)
    if not match:
        return body
    start = mask.index("{", match.start())
    end = block_end(mask, start)
    following = re.match(r"\s*else\s*\{", mask[end:])
    if following:
        other_start = mask.index("{", end)
        other_end = block_end(mask, other_start)
        return body[:match.start()] + body[start + 1:end - 1] + body[other_end:]
    if re.search(r"\breturn\b", mask[start:end]):
        return body[:match.start()] + body[start + 1:end - 1]
    return body


def ingress_configuration(variables):
    functions = rust_functions()
    pending = [("main", "config")]
    visited = set()
    bodies = []
    while pending:
        key = pending.pop()
        if key in visited:
            continue
        visited.add(key)
        assert key in functions, f"Configuration function {key} must exist"
        body = ingress_body(functions[key])
        guard = re.search(
            r'let Some\(\w+\) = (?:std::)?env::var_os\("([A-Z_]+)"\) else\s*\{',
            body,
        )
        if guard and guard[1] not in variables:
            body = body[:guard.end()]
        bodies.append(body)
        mask = rust_mask(body)
        for owner, name in re.findall(r"\b(\w+)::(\w+)\s*\(", mask):
            if (owner, name) in functions:
                pending.append((owner, name))
        for name in re.findall(r"(?<![:\w.])([a-z_]\w*)\s*\(", mask):
            if (key[0], name) in functions:
                pending.append((key[0], name))
            else:
                local = [
                    (owner, name)
                    for owner in ("main", "hosted", "trusted")
                    if (owner, name) in functions
                ]
                if len(local) == 1:
                    pending.extend(local)
    return bodies, functions


def environment_reads(variables):
    bodies, functions = ingress_configuration(variables)
    readers = set()
    for (_, name), body in functions.items():
        if re.search(r"env::var\(name\)\s*\.map_err", body):
            readers.add(name)
    while True:
        derived = {
            name for (_, name), body in functions.items()
            if any(re.search(rf"\b{reader}\(name\)", body) for reader in readers)
        }
        if derived <= readers:
            break
        readers |= derived
    required, available = set(), set()
    for body in bodies:
        stems = re.findall(r'\(EventKind::\w+, "([A-Z_]+)"\)', body)
        for template in re.findall(r'format!\("([A-Z_{}a-z]+)"\)', body):
            if "{stem}" in template:
                assert stems, "Dynamic source settings must name their event stems"
                body = body.replace(
                    f'format!("{template}")',
                    '"' + template.replace("{stem}", stems[0]) + '"',
                )
                expanded = {template.replace("{stem}", stem) for stem in stems}
                available |= expanded
                required |= expanded
        for match in re.finditer(
            r'(?:std::)?env::var(?:_os)?\("([A-Z][A-Z0-9_]*)"\)', body
        ):
            available.add(match[1])
            if re.match(r"\s*\.map_err\s*\(", body[match.end():]):
                required.add(match[1])
        for match in re.finditer(r'\b(\w+)\(\s*&?"([A-Z][A-Z0-9_]*)"', body):
            if match[1] in readers:
                required.add(match[2])
                available.add(match[2])
            elif match[1] in {"number", "read"}:
                available.add(match[2])
    return required, available


def entry_settings():
    source = (SOURCE / "env-init.sh").read_text()
    match = re.search(r"\ningress\)\n([\s\S]*?)\n\s*;;", source)
    assert match, "Ingress startup must select its own role"
    prefix = source[:source.index('case "${LAYERX_ROLE:-}"')]
    body = prefix + match[1]
    exports = dict(re.findall(r'\b([A-Z][A-Z0-9_]*)=([^\s\\]+)', body))
    exported = set(re.findall(r"\bexport ([A-Z][A-Z0-9_]*)", body))
    exported |= {
        name for name in exports
        if name.startswith("LAYERX_WEBHOOKS_") and name != "LAYERX_WEBHOOKS_INSTANCE_ID"
    }
    exported |= set(re.findall(r"^export ([A-Z][A-Z0-9_]*)$", body, re.M))
    available = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)", body))
    files = {
        name: value for name, value in exports.items()
        if value.startswith("/run/layerx/")
    }
    return source, exported, available, files


def file_rows():
    rows = []
    for line in (CONTAINER / "files.tsv").read_text().splitlines():
        if line and not line.startswith("#"):
            name, guest, mode, roles = line.split("\t")
            if "*" in roles.split(",") or "ingress" in roles.split(","):
                rows.append((name, guest, mode))
    assert len({name for name, _, _ in rows}) == len(rows)
    assert len({guest for _, guest, _ in rows}) == len(rows)
    return rows


def test_ingress_variables_follow_configuration_reads():
    rows = load_rows(DEFINITION)
    variables = rows[f"vars.{SERVICE}"]
    required, available = environment_reads(variables)
    _, exported, shell_reads, _ = entry_settings()
    material = {name for name, _, _ in file_rows()}
    common = (ROOT / "docker/common/env-files.sh").read_text()
    common_reads = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)(?=[:}])", common))
    build_arguments = set(
        re.findall(r"^ARG (\w+)", (CONTAINER / "Dockerfile").read_text(), re.M)
    )
    assert required, "Ingress configuration must read required settings"
    assert required - exported <= variables.keys(), sorted(
        required - exported - variables.keys()
    )
    assert material | build_arguments <= variables.keys()
    assert variables.keys() <= (
        available | shell_reads | common_reads | material | build_arguments
    )
    assert not variables.keys() & exported - shell_reads
    assert variables["LAYERX_ROLE"] == "ingress"
    assert variables["LAYERX_FILES_OWNER"] == "65532:65532"


def test_ingress_material_matches_files_read_by_the_binary():
    rows = load_rows(DEFINITION)
    variables = rows[f"vars.{SERVICE}"]
    required, _ = environment_reads(variables)
    _, _, _, exported_files = entry_settings()
    destinations = {guest for _, guest, _ in file_rows()}
    consumed = {
        value for name, value in {**variables, **exported_files}.items()
        if name in required and value.startswith("/run/layerx/")
    }
    assert destinations == consumed
    for name, _, mode in file_rows():
        assert variables[name].startswith("env:")
        assert not mode or int(mode, 8) == 0o600


def test_ingress_inputs_have_project_service_names_and_formats():
    rows = load_rows(DEFINITION)
    variables = rows[f"vars.{SERVICE}"]
    inputs = rows[f"inputs.{SERVICE}"]
    prefix = re.sub(r"[^A-Z0-9]", "_", f"{DEFINITION.parent.name}_{SERVICE}".upper())
    expected = set()
    for name, value in variables.items():
        if value.startswith("env:"):
            env_name = value.removeprefix("env:")
            assert env_name == f"{prefix}_{name}"
            expected.add(env_name)
    assert inputs.keys() == expected
    lines = DEFINITION.read_text().splitlines()
    for name, classification in inputs.items():
        assert classification in {"generated", "provider", "chain", "operator"}
        index = next(i for i, line in enumerate(lines) if line.startswith(name + " ="))
        assert lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6
    for value in variables.values():
        assert not re.search(r"(?:https?|rediss?)://(?!\$\{\{)", value)
        assert not re.search(r"-----BEGIN|\b0x[0-9a-fA-F]{40,}\b", value)


def test_ingress_service_matches_rollout_and_private_topology():
    rows = load_rows(DEFINITION)
    assert set(rows) == {f"{kind}.{SERVICE}" for kind in ("service", "vars", "inputs")}
    service = rows[f"service.{SERVICE}"]
    rollout = load_rows(ROOT / "deploy/rollout.kvx")
    original = rollout[f"service.{SERVICE}"]
    assert service["kind"] == "repo"
    assert service["health"] == "/livez"
    assert original["health"] == "/healthz"
    assert service["order"] == original["order"]
    assert original["railway_service"] == SERVICE
    assert service["restart"] == "ALWAYS"
    assert service["public"] == 0
    manifest = (SOURCE / "deployment.yaml").read_text()
    ingress = manifest.split("  name: layerx-webhooks\nspec:\n", 1)[1].split("---", 1)[0]
    assert service["replicas"] == int(re.search(r"replicas: (\d+)", ingress)[1])
    assert "volume" not in service
    references = {
        name for value in rows[f"vars.{SERVICE}"].values()
        for name in re.findall(r"\$\{\{([\w-]+)\.", value)
    }
    assert set(service["needs"]) == references
    for name in references:
        dependency = rollout[f"service.{name}"]
        assert dependency["railway_service"] == name
        assert dependency["order"] < service["order"]


def test_ingress_build_context_and_watches_cover_local_dependencies():
    service = load_rows(DEFINITION)[f"service.{SERVICE}"]
    dockerfile = ROOT / service["dockerfile"]
    assert dockerfile == CONTAINER / "Dockerfile"
    assert dockerfile.is_file()
    assert service["root"] == "."
    watch = service["watch"]
    for pattern in watch:
        assert list(ROOT.glob(pattern)), pattern
    source_copies = []
    for line in dockerfile.read_text().splitlines():
        if line.startswith("COPY ") and "--from=" not in line:
            sources = [
                part for part in shlex.split(line)[1:-1]
                if not part.startswith("--")
            ]
            source_copies.extend(sources)
    assert source_copies
    for source in source_copies:
        if source.startswith("artifacts/"):
            continue
        path = ROOT / source
        assert path.exists(), source
        representative = str(path.relative_to(ROOT)).rstrip("/")
        if path.is_dir():
            representative += "/Cargo.toml"
        assert any(fnmatch.fnmatchcase(representative, pattern) for pattern in watch), source
    for folder in (CONTAINER, ROOT / "docker/common", SOURCE):
        for path in folder.iterdir():
            if path.is_file():
                assert any(
                    fnmatch.fnmatchcase(str(path.relative_to(ROOT)), pattern)
                    for pattern in watch
                )
    assert "COPY services/ services/" in dockerfile.read_text()
    assert "COPY programs/ programs/" in dockerfile.read_text()
    for stage in dockerfile.read_text().split("FROM ")[1:]:
        if "apt-get install" in stage:
            assert "util-linux" in stage
    ignore = (CONTAINER / "Dockerfile.dockerignore").read_text()
    for source in source_copies:
        if source.startswith("docker/"):
            assert f"!{source}" in ignore.splitlines()


def test_ingress_health_uses_port_and_preserves_mutual_tls():
    service = load_rows(DEFINITION)[f"service.{SERVICE}"]
    variables = load_rows(DEFINITION)[f"vars.{SERVICE}"]
    source, _, _, _ = entry_settings()
    health = source.split("health() {", 1)[1].split("\n}", 1)[0]
    assert 'elif [ -n "${PORT:-}" ]' in health
    assert 'echo "[::]:$PORT"' in health
    assert "LAYERX_HEALTH_ADDR" not in variables
    assert 1024 <= int(variables["PORT"]) <= 65535
    assert 'LAYERX_WEBHOOKS_HEALTH_LISTEN="$(health "[::]:8080")"' in source
    assert 'LAYERX_WEBHOOKS_LISTEN="${LAYERX_WEBHOOKS_LISTEN:-[::]:9443}"' in source
    assert 'LAYERX_WEBHOOKS_ROLE=ingress LAYERX_WEBHOOKS_LISTENER=tls' in source
    assert "setpriv --reuid=65532 --regid=65532 --clear-groups --no-new-privs" in source
    main = (SOURCE / "src/main.rs").read_text()
    assert f'request.path == "{service["health"]}"' in main
    assert 'request.path == "/livez"' in main
    assert '"live": true' in main
    assert '"role": role' in main
    assert ".map(TcpListener::bind)" in main
    assert "serve_health(&listener" in main
    assert "WebPkiClientVerifier::builder" in main
    assert "tls_config(Some(&client_ca))" in main


def test_webhooks_ingress_network_label_input() -> None:
    sections = load_rows(DEFINITION)
    input_name = "PLATFORM_WEBHOOKS_INGRESS_LAYERX_WEBHOOKS_NETWORK_ID"
    assert sections["vars.webhooks-ingress"]["LAYERX_WEBHOOKS_NETWORK_ID"] == f"env:{input_name}"
    assert sections["inputs.webhooks-ingress"][input_name] == "operator"
    lines = DEFINITION.read_text().splitlines()
    index = next(i for i, line in enumerate(lines) if line.startswith(input_name + " ="))
    assert lines[index - 1] == (
        "# Network label used by the project gateway: "
        "1-64 printable ASCII bytes without spaces."
    )
