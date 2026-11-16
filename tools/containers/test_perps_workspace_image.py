from pathlib import Path
import json
import re
import runpy
import shlex
from fnmatch import fnmatchcase

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKSPACE = ROOT / "apps/perps"
MANIFEST_ACTIVATION = (
    "corepack prepare \"$(node -p 'require(\"./package.json\").packageManager')\" --activate"
)


def read_compile_config(path):
    tokens = re.sub(
        r'"(?:\\.|[^"\\])*"|//[^\n]*|/\*.*?\*/',
        lambda match: match[0] if match[0].startswith('"') else " ",
        path.read_text(),
        flags=re.S,
    )
    return json.loads(re.sub(r",\s*([}\]])", r"\1", tokens))


def workspace_compile_files():
    layout = yaml.safe_load((WORKSPACE / "pnpm-workspace.yaml").read_text())
    members = {}
    for pattern in layout["packages"]:
        for directory in WORKSPACE.glob(pattern):
            manifest = directory / "package.json"
            if manifest.is_file():
                data = json.loads(manifest.read_text())
                members[data["name"]] = (directory, data)
    pending = ["@paxeer-network/web"]
    visited = set()
    configs = set()

    def follow(path, chain=()):
        path = path.resolve()
        assert path.is_relative_to(WORKSPACE), path
        assert path.is_file(), f"Compile configuration is absent: {path}"
        assert path not in chain, f"Compile configuration cycle: {chain + (path,)}"
        if path in configs:
            return
        configs.add(path)
        parents = read_compile_config(path).get("extends", [])
        if isinstance(parents, str):
            parents = [parents]
        for parent in parents:
            if parent.startswith("."):
                target = path.parent / parent
            else:
                parts = parent.split("/")
                count = 2 if parent.startswith("@") else 1
                package = "/".join(parts[:count])
                assert package in members, f"Unknown compile configuration package: {parent}"
                target = members[package][0].joinpath(*parts[count:])
            if not target.suffix:
                target = target.with_suffix(".json")
            follow(target, chain + (path,))

    while pending:
        name = pending.pop()
        if name in visited:
            continue
        visited.add(name)
        directory, manifest = members[name]
        for kind in ("dependencies", "devDependencies", "optionalDependencies", "peerDependencies"):
            pending.extend(
                dependency for dependency, version in manifest.get(kind, {}).items()
                if version.startswith("workspace:")
            )
        for config in directory.glob("tsconfig*.json"):
            follow(config)
    assert WORKSPACE / "tsconfig.json" in configs
    assert WORKSPACE / "packages/tsconfig/base.json" in configs
    return configs


def context_excludes(path):
    rules = (WORKSPACE / ".dockerignore").read_text().splitlines()

    def matches(parts, pattern):
        if not pattern:
            return not parts
        if pattern[0] == "**":
            return matches(parts, pattern[1:]) or bool(parts) and matches(parts[1:], pattern)
        return bool(parts) and fnmatchcase(parts[0], pattern[0]) and matches(parts[1:], pattern[1:])

    relative = path.relative_to(WORKSPACE)
    for candidate in (*reversed(relative.parents), relative):
        excluded = False
        for rule in rules:
            rule = rule.strip()
            if not rule or rule.startswith("#"):
                continue
            negate = rule.startswith("!")
            pattern = rule.removeprefix("!").strip("/")
            if matches(candidate.parts, pattern.split("/")):
                excluded = not negate
        if excluded:
            return True
    return False


def test_perps_compile_config_chains_reach_builder():
    recipe = (WORKSPACE / "Dockerfile").read_text()
    deps = recipe.split(" AS deps\n", 1)[1].split("\nFROM ", 1)[0]
    before_install = deps.split("RUN pnpm install --frozen-lockfile", 1)[0]
    assert "WORKDIR /app\n" in before_install
    copies = []
    for line in before_install.splitlines():
        if line.startswith("COPY "):
            arguments = shlex.split(line)[1:]
            assert not any(argument.startswith("--") for argument in arguments), line
            copies.extend((source, arguments[-1]) for source in arguments[:-1])
    inputs = workspace_compile_files() | {
        WORKSPACE / name for name in (
            "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", ".npmrc",
        )
    }
    for path in sorted(inputs):
        relative = path.relative_to(WORKSPACE)
        assert not context_excludes(path), f"Build context excludes {relative}"
        copied = False
        for source, destination in copies:
            origin = WORKSPACE / source
            if origin.is_dir() and path.is_relative_to(origin):
                target = Path("/app") / destination / path.relative_to(origin)
            elif origin == path:
                target = Path("/app") / destination
                if destination.endswith("/"):
                    target /= path.name
            else:
                continue
            copied |= target == Path("/app") / relative
        assert copied, f"Install stage does not copy {relative} to /app/{relative}"
    builder = recipe.split(" AS builder\n", 1)[1].split("\nFROM ", 1)[0]
    assert builder.index("WORKDIR /app") < builder.index("COPY --from=deps /app ./")
    assert builder.index("COPY --from=deps /app ./") < builder.index("pnpm build")


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
