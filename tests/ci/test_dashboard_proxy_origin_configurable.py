import json
import os
from pathlib import Path
import subprocess

import pytest


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
