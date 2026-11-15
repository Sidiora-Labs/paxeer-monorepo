import base64
import fnmatch
import json
import os
import re
import shlex
import subprocess
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/search-front.kvx"
IMAGE = ROOT / "docker/search-front"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def copied_files():
    copies = {}
    for line in (IMAGE / "Dockerfile").read_text().splitlines():
        if line.startswith("COPY "):
            words = shlex.split(line)
            sources = [word for word in words[1:-1] if not word.startswith("--")]
            for source in sources:
                copies[ROOT / source] = words[-1]
    return copies


def environment_contract():
    reads, required, defaults = set(), set(), {}
    dockerfile = (IMAGE / "Dockerfile").read_text().replace("\\\n", " ")
    for line in dockerfile.splitlines():
        if line.startswith("ARG "):
            name = line.split()[1].split("=", 1)[0]
            reads.add(name)
            required.add(name)
        if line.startswith("ENV "):
            for assignment in shlex.split(line[4:]):
                name, value = assignment.split("=", 1)
                defaults[name] = value
                reads.add(name)
    for path in copied_files():
        if path.suffix == ".sh":
            text = "\n".join(
                line for line in path.read_text().splitlines()
                if not line.lstrip().startswith("#")
            )
            for match in re.finditer(
                r"\$(?:\{([A-Z][A-Z0-9_]*)([^}]*)\}|([A-Z][A-Z0-9_]*))", text
            ):
                name, modifier, plain = match.groups()
                if modifier and modifier.startswith("$"):
                    continue
                name = name or plain
                reads.add(name)
                if (
                    not modifier
                    or modifier.startswith(("?", ":?"))
                    or (
                        modifier in ("-", ":-")
                        and re.search(re.escape(name) + r" is (?:unset|empty|required)", text)
                    )
                ):
                    required.add(name)
        if path.suffix == ".tsv":
            for line in path.read_text().splitlines():
                if line and not line.startswith("#"):
                    name, guest, mode, roles = line.split("\t")
                    assert guest.startswith("/") and roles
                    reads.add(name)
                    required.add(name)
    return reads, required - defaults.keys(), defaults


def test_service_matches_image_and_rollout():
    config = definition()
    assert set(config) == {"service", "vars", "inputs"}
    assert set(config["service"]) == {"search-front"}
    service = config["service"]["search-front"]
    rollout = tomllib.loads((ROOT / "deploy/rollout.kvx").read_text())["service"][
        "search-front"
    ]
    assert rollout["target"] == "railway"
    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/search-front/Dockerfile"
    assert (ROOT / service["root"]).resolve() == ROOT
    assert service["health"] == rollout["health"]
    assert service["order"] == rollout["order"]
    assert service["restart"] == "ALWAYS"
    example = (
        ROOT / "services/interop/deploy/search-front/railway.env.example"
    ).read_text()
    assert "at least two replicas" in example
    assert service["replicas"] >= 2
    assert service["public"] == 1
    assert service["needs"] == []
    assert "volume" not in service
    assert "target" not in service
    assert (ROOT / service["dockerfile"]).is_file()
    assert service["watch"]
    for pattern in service["watch"]:
        assert list(ROOT.glob(pattern)), pattern
    for path in [*copied_files(), *IMAGE.iterdir()]:
        assert path.is_file(), path
        relative = path.relative_to(ROOT).as_posix()
        assert any(
            fnmatch.fnmatchcase(relative, pattern) for pattern in service["watch"]
        ), relative
    ignore = (IMAGE / "Dockerfile.dockerignore").read_text().splitlines()
    for path in copied_files():
        assert "!" + path.relative_to(ROOT).as_posix() in ignore


def test_variables_follow_source_readers_and_inputs():
    config = definition()
    variables = config["vars"]["search-front"]
    inputs = config["inputs"]["search-front"]
    reads, required, defaults = environment_contract()
    assert required <= variables.keys(), (
        f"Missing required variables: {required - variables.keys()}"
    )
    assert variables.keys() <= reads, f"Unread variables: {variables.keys() - reads}"
    for name in variables.keys() & defaults.keys():
        assert variables[name] != defaults[name], f"Unnecessary default override: {name}"
    expected_inputs = set()
    for name, value in variables.items():
        assert isinstance(value, str) and value
        if value.startswith("env:"):
            expected = (
                "_".join((DEFINITION.parent.name, DEFINITION.stem, name))
                .upper()
                .replace("-", "_")
            )
            assert value == "env:" + expected
            expected_inputs.add(expected)
        else:
            assert name == "PORT" and re.fullmatch(r"[1-9][0-9]{0,4}", value)
            assert int(value) <= 65535
    assert inputs.keys() == expected_inputs
    lines = DEFINITION.read_text().splitlines()
    for name, kind in inputs.items():
        assert kind == "operator"
        index = next(
            i for i, line in enumerate(lines) if line.startswith(name + " =")
        )
        assert index > 0 and lines[index - 1].startswith("# ")
        assert len(lines[index - 1].split()) >= 6


def test_port_hook_is_wired_before_nginx_and_preserves_routes(tmp_path):
    copies = copied_files()
    hook = next(
        path for path, guest in copies.items()
        if guest == "/docker-entrypoint.d/05-listen-port.sh"
    )
    template = next(
        path for path, guest in copies.items()
        if guest == "/etc/nginx/nginx.conf.template"
    )
    dockerfile = (IMAGE / "Dockerfile").read_text()
    assert (
        f"COPY --chmod=0555 {hook.relative_to(ROOT)} "
        "/docker-entrypoint.d/05-listen-port.sh"
    ) in dockerfile
    entrypoint = json.loads(next(
        line[11:] for line in dockerfile.splitlines()
        if line.startswith("ENTRYPOINT ")
    ))
    assert entrypoint[-1] == "/docker-entrypoint.sh"
    assert 'CMD ["nginx", "-g", "daemon off;"]' in dockerfile
    service = definition()["service"]["search-front"]
    for port in (definition()["vars"]["search-front"]["PORT"], "19081", "65535"):
        output = tmp_path / "nginx.conf"
        subprocess.run(
            ["sh", str(hook), str(template), str(output)],
            env={**os.environ, "PORT": port}, check=True,
        )
        rendered = output.read_text()
        assert re.findall(r"listen \[::\]:(\d+) ipv6only=off;", rendered) == [port]
        original = template.read_text()
        assert rendered == re.sub(
            r"(listen \[::\]:)\d+( ipv6only=off;)",
            lambda m: m[1] + port + m[2], original,
        )
        route = re.search(
            r"location = " + re.escape(service["health"]) + r"\s*\{([^}]+)\}",
            rendered,
        )
        assert route and 'return 200 "ok\\n";' in route[1]
        assert "proxy_ssl_verify on;" in rendered


@pytest.mark.parametrize(
    "port",
    [None, "", "0", "08080", "65536", "9999999999999999999999", "-1", "1.5", "8080;return 200"],
)
def test_port_hook_rejects_invalid_port(tmp_path, port):
    output = tmp_path / "nginx.conf"
    environment = dict(os.environ)
    environment.pop("PORT", None)
    if port is not None:
        environment["PORT"] = port
    result = subprocess.run(
        [
            "sh", str(IMAGE / "listen-port.sh"),
            str(ROOT / "services/interop/deploy/search-front/nginx.conf"),
            str(output),
        ],
        env=environment, capture_output=True, text=True,
    )
    assert result.returncode != 0
    assert "PORT" in result.stderr
    assert not output.exists()


def test_upstream_input_is_decoded_by_entrypoint(tmp_path):
    files = copied_files()
    table = next(path for path in files if path.suffix == ".tsv")
    decoder = next(
        path for path, guest in files.items()
        if guest == "/usr/local/bin/layerx-env-files"
    )
    entrypoint = (
        (IMAGE / "Dockerfile").read_text().split("ENTRYPOINT ", 1)[1].splitlines()[0]
    )
    assert json.loads(entrypoint)[0] == "/usr/local/bin/layerx-env-files"
    for line in table.read_text().splitlines():
        name, guest, mode, roles = line.split("\t")
        assert definition()["vars"]["search-front"][name].startswith("env:")
        output = tmp_path / Path(guest).name
        local_table = tmp_path / "files.tsv"
        local_table.write_text("\t".join((name, str(output), mode, roles)) + "\n")
        content = (
            'map $xweb_client $xweb_node { default ""; }\n'
            'map $xweb_client $xweb_eligible { default 0; }\n'
        )
        environment = {
            "PATH": os.defpath, "LAYERX_FILES_TABLE": str(local_table),
            name: base64.b64encode(content.encode()).decode(),
        }
        subprocess.run(["sh", str(decoder), "true"], env=environment, check=True)
        assert output.read_text() == content
        assert output.stat().st_mode & 0o777 == int(mode or "0600", 8)
        environment.pop(name)
        result = subprocess.run(
            ["sh", str(decoder), "true"],
            env=environment, capture_output=True, text=True,
        )
        assert result.returncode != 0
        assert "required variable " + name in result.stderr


def test_trusted_proxy_input_is_required_and_rendered(tmp_path):
    copies = copied_files()
    renderer = next(
        path for path, guest in copies.items() if guest == "/usr/local/bin/real-ip"
    )
    output = tmp_path / "real-ip.conf"
    entrypoint = json.loads(
        (IMAGE / "Dockerfile").read_text().split("ENTRYPOINT ", 1)[1].splitlines()[0]
    )
    assert entrypoint[1:3] == [
        "/usr/local/bin/real-ip", "/etc/nginx/search/conf.d/real-ip.conf"
    ]
    environment = {
        "PATH": os.defpath, "LAYERX_TRUSTED_PROXIES": "127.0.0.1/32, ::1/128"
    }
    subprocess.run(
        ["sh", str(renderer), str(output), "true"], env=environment, check=True,
    )
    assert output.read_text() == (
        "set_real_ip_from 127.0.0.1/32;\nset_real_ip_from ::1/128;\n"
        "real_ip_header X-Forwarded-For;\nreal_ip_recursive on;\n"
    )
    for value in (None, "", " , ", "127.0.0.1;return"):
        output.unlink(missing_ok=True)
        environment.pop("LAYERX_TRUSTED_PROXIES", None)
        if value is not None:
            environment["LAYERX_TRUSTED_PROXIES"] = value
        result = subprocess.run(
            ["sh", str(renderer), str(output), "true"],
            env=environment, capture_output=True, text=True,
        )
        assert result.returncode != 0
        assert "LAYERX_TRUSTED_PROXIES" in result.stderr
        assert not output.exists()
