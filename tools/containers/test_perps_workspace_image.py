from pathlib import Path
import json
import re
import runpy

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKSPACE = ROOT / "apps/perps"
MANIFEST_ACTIVATION = (
    "corepack prepare \"$(node -p 'require(\"./package.json\").packageManager')\" --activate"
)


def test_perps_workspace_install_contains_every_member():
    manifest = json.loads((WORKSPACE / "package.json").read_text())
    assert re.fullmatch(r"pnpm@\d+\.\d+\.\d+\+sha512\.[0-9a-f]{128}", manifest["packageManager"])
    layout = yaml.safe_load((WORKSPACE / "pnpm-workspace.yaml").read_text())
    assert layout["packages"] == ["apps/*", "packages/*"]
    recipe = (WORKSPACE / "Dockerfile").read_text()
    deps = recipe.split(" AS deps\n", 1)[1].split("\nFROM ", 1)[0]
    install = deps.index("RUN pnpm install --frozen-lockfile")
    for instruction in (
        "COPY package.json pnpm-lock.yaml ./",
        "COPY pnpm-workspace.yaml .npmrc ./",
        "COPY apps ./apps",
        "COPY packages ./packages",
        MANIFEST_ACTIVATION,
    ):
        assert deps.index(instruction) < install
    builder = recipe.split(" AS builder\n", 1)[1].split("\nFROM ", 1)[0]
    assert "COPY --from=deps /app ./" in builder
    assert MANIFEST_ACTIVATION in builder
    members = [
        path for pattern in layout["packages"]
        for path in WORKSPACE.glob(pattern) if (path / "package.json").is_file()
    ]
    assert WORKSPACE / "apps/web" in members
    assert WORKSPACE / "packages/paxeer-backend" in members
    lock = yaml.safe_load((WORKSPACE / "pnpm-lock.yaml").read_text())
    for member in members:
        assert member.relative_to(WORKSPACE).as_posix() in lock["importers"]
    ignore = (WORKSPACE / ".dockerignore").read_text().splitlines()
    for excluded in ("**/node_modules", "**/.next*", "**/dist", ".env", ".env.*", "**/.env", "**/.env.*"):
        assert excluded in ignore
    assert "apps" not in ignore and "packages" not in ignore


def test_perps_railway_image_and_runtime_settings_match():
    project = runpy.run_path(str(ROOT / "tools/deploy/railway-perps-project.py"))
    _, services = project["load"](ROOT / "deploy/railway-perps-project.kvx")
    web = next(service for service in services if service["name"] == "web")
    railway = json.loads((WORKSPACE / "railway.json").read_text())
    compose = yaml.safe_load((WORKSPACE / "deploy/docker-compose.yml").read_text())
    app = compose["services"]["app"]
    assert web["image"] == "perps-web"
    assert web["vars"]["PORT"] == app["environment"]["PORT"] == "3000"
    assert web["health"] == railway["deploy"]["healthcheckPath"] == "/api/build-id"
    assert web["timeout"] == railway["deploy"]["healthcheckTimeout"] == 300
    assert web["restart"] == railway["deploy"]["restartPolicyType"] == "ON_FAILURE"
    assert web["replicas"] == web["public"] == 1
    assert set(web["vars"]) | {"VERCEL_GIT_COMMIT_SHA"} == set(app["environment"])
    for name in ("NODE_ENV", "HOSTNAME"):
        assert web["vars"][name] == app["environment"][name]
    for name in ("PRICE_FEED_ORIGIN", "PRICE_FEED_REST_API_KEY", "PRICE_FEED_WS_API_KEY",
                 "CANDLE_TRUSTED_PROXY_IPS", "PRESENCE_TRUSTED_PROXY_IPS"):
        assert web["vars"][name] == "env:" + name
        assert name not in app["build"]["args"]
    assert not any(name.startswith("NEXT_PUBLIC_") for name in web["vars"])
    assert app["build"]["context"] == ".."
    assert app["build"]["dockerfile"] == railway["build"]["dockerfilePath"] == "Dockerfile"
