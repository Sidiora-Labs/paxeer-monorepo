import json
import os
from pathlib import Path
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
CONFIG = ROOT / "services/platform/hosted/dashboard/web/next.config.ts"


def test_dashboard_next_config_names_no_private_host():
    assert ".internal" not in CONFIG.read_text()


@pytest.mark.parametrize("node_env", [None, "development", "test", "production"])
@pytest.mark.parametrize(
    "origin",
    [None, "", "https://dashboard-api.example.com", "http://127.0.0.1:9555/"],
)
def test_dashboard_next_config_reads_dashboard_api_origin(node_env, origin):
    env = os.environ.copy()
    env.pop("NODE_ENV", None)
    env.pop("DASHBOARD_API_ORIGIN", None)
    if node_env is not None:
        env["NODE_ENV"] = node_env
    if origin is not None:
        env["DASHBOARD_API_ORIGIN"] = origin

    result = subprocess.run(
        [
            "node",
            "--input-type=module",
            "--eval",
            "const { default: config } = await import(process.argv[1]);\n"
            'console.log("CONFIG_LOADED");\n'
            "console.log(JSON.stringify(await config.rewrites()));\n",
            CONFIG.as_uri(),
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=15,
    )

    if node_env == "production" and not origin:
        assert result.returncode != 0
        assert "DASHBOARD_API_ORIGIN must be set in production" in result.stderr
        assert result.stdout == "", "configuration must fail during import"
    else:
        assert result.returncode == 0, result.stderr
        loaded, rewrites = result.stdout.splitlines()
        assert loaded == "CONFIG_LOADED"
        expected_origin = origin or "http://127.0.0.1:9445"
        assert json.loads(rewrites) == [
            {
                "source": "/v1/dashboard/:path*",
                "destination": f"{expected_origin.rstrip('/')}/v1/dashboard/:path*",
            }
        ]


def test_dashboard_docker_build_origin(tmp_path):
    origin = "https://dashboard-api.example.com"
    helper = "source services/platform/hosted/tests/beta-images.sh; "
    helper += "image_build_args layerx-dashboard-web"
    env = os.environ.copy()
    env["DASHBOARD_API_ORIGIN"] = origin
    result = subprocess.run(
        ["bash", "-ec", helper],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=15,
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout == f"--build-arg DASHBOARD_API_ORIGIN={origin}"

    workflow = yaml.safe_load((ROOT / ".github/workflows/publish-images.yml").read_text())
    plan = next(
        step for step in workflow["jobs"]["plan"]["steps"] if step.get("id") == "plan"
    )
    assert plan["env"]["DASHBOARD_API_ORIGIN"] == "${{ vars.DASHBOARD_API_ORIGIN }}"
    for job in ("images", "images-dependent"):
        build = next(
            step
            for step in workflow["jobs"][job]["steps"]
            if step.get("uses", "").startswith("docker/build-push-action@")
        )
        assert build["with"]["build-args"] == "${{ matrix.image.build_args }}"

    output = tmp_path / "github-output"
    env["GITHUB_OUTPUT"] = str(output)
    result = subprocess.run(
        ["bash", "-ec", plan["run"]],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, result.stderr
    outputs = dict(line.split("=", 1) for line in output.read_text().splitlines())
    dashboard = next(
        image
        for image in json.loads(outputs["images"])
        if image["name"] == "layerx-dashboard-web"
    )
    assert dashboard["dockerfile"] == "docker/platform-dashboard-web/Dockerfile"
    assert dashboard["build_args"] == f"DASHBOARD_API_ORIGIN={origin}"

    invalid_origins = [None, "", " "]
    for whitespace in " \t\n\r\v\f":
        invalid_origins.extend(
            [
                f"{whitespace}{origin}",
                f"{origin}{whitespace}",
                f"{origin}/{whitespace}path",
            ]
        )
    for invalid in invalid_origins:
        invalid_env = env.copy()
        invalid_env.pop("DASHBOARD_API_ORIGIN", None)
        if invalid is not None:
            invalid_env["DASHBOARD_API_ORIGIN"] = invalid
        for script in (helper, plan["run"]):
            output.unlink(missing_ok=True)
            result = subprocess.run(
                ["bash", "-ec", script],
                cwd=ROOT,
                env=invalid_env,
                capture_output=True,
                text=True,
                timeout=60,
            )
            assert result.returncode != 0, (invalid, result.stdout)
            assert (
                "DASHBOARD_API_ORIGIN must be set and contain no whitespace"
                in result.stderr
            )
            assert result.stdout == ""
            assert not output.exists(), "invalid origins must not publish a matrix"

    inspect_script = """
const fs = require('node:fs');
if (!fs.statSync('server.js').isFile()) throw new Error('standalone server missing');
const manifest = JSON.parse(fs.readFileSync('.next/routes-manifest.json', 'utf8'));
console.log(JSON.stringify({origin: process.env.DASHBOARD_API_ORIGIN,
                           rewrites: manifest.rewrites}));
"""
    for runtime_origin in (None, "https://runtime-api.example.net"):
        command = ["docker", "run", "--rm", "--network", "none", "--entrypoint", "node"]
        if runtime_origin is not None:
            command.extend(["--env", f"DASHBOARD_API_ORIGIN={runtime_origin}"])
        command.extend(["layerx-dashboard-web:issue-1818", "--eval", inspect_script])
        result = subprocess.run(
            command,
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=30,
        )
        assert result.returncode == 0, result.stderr
        inspection = json.loads(result.stdout)
        assert inspection.get("origin") == runtime_origin
        rewrites = inspection["rewrites"]
        routes = rewrites["beforeFiles"] + rewrites["afterFiles"] + rewrites["fallback"]
        dashboard_routes = [
            route for route in routes if route["source"] == "/v1/dashboard/:path*"
        ]
        assert len(dashboard_routes) == 1
        assert dashboard_routes[0]["destination"] == f"{origin}/v1/dashboard/:path*"
