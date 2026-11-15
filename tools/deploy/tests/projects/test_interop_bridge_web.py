"""The bridge project definition follows its application and container inputs."""

import fnmatch
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[4]
APP = ROOT / "apps/bridge"
DEFINITION = ROOT / "deploy/projects/interop/bridge-web.kvx"
RECIPE = ROOT / "docker/bridge-web/Dockerfile"


def definition():
    return tomllib.loads(DEFINITION.read_text())


def source_variables():
    """Read environment accesses from production modules, including build config."""
    names = set()
    paths = list((APP / "src").rglob("*")) + list(APP.glob("*.config.*"))
    for path in paths:
        if path.suffix not in {".ts", ".tsx", ".js", ".mjs", ".cjs"}:
            continue
        if {"test", "fixtures"} & set(path.relative_to(APP).parts):
            continue
        if re.search(r"\.(?:spec|test)\.", path.name):
            continue
        source = path.read_text()
        names.update(re.findall(r"\bprocess\s*\.\s*env\s*\.\s*([A-Z][A-Z0-9_]*)", source))
        names.update(re.findall(r'''\bprocess\s*\.\s*env\s*\[\s*['"]([A-Z][A-Z0-9_]*)['"]\s*\]''', source))
    assert names, "The application must expose its deployment settings."
    return names


def build_arguments():
    return set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)", RECIPE.read_text(), re.M))


def test_interop_bridge_web_variables_follow_source():
    manifest = definition()
    rows = manifest["vars"]["bridge-web"]
    public = {name for name in source_variables() if name.startswith("NEXT_PUBLIC_")}
    assert public == build_arguments()
    assert set(rows) == source_variables() | build_arguments() | {"PORT"}
    for name in source_variables() | build_arguments():
        assert rows[name] == f"env:INTEROP_BRIDGE_WEB_{name}"


def test_interop_bridge_web_inputs_are_named_and_described():
    manifest = definition()
    rows = manifest["vars"]["bridge-web"]
    inputs = manifest["inputs"]["bridge-web"]
    expected = {value.removeprefix("env:") for value in rows.values() if value.startswith("env:")}
    assert set(inputs) == expected
    for variable, value in rows.items():
        if not value.startswith("env:"):
            assert variable == "PORT"
            continue
        name = value.removeprefix("env:")
        assert name == f"INTEROP_BRIDGE_WEB_{variable}"
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
        assert inputs[name] in {"generated", "provider", "chain", "operator"}
        assert re.search(rf"^# [^\n]+\n{re.escape(name)} = ", DEFINITION.read_text(), re.M)
        assert inputs[name] == ("chain" if "_VAULT_" in variable or variable.endswith("_PROGRAM_ID") else "operator")


def test_interop_bridge_web_repo_and_watch_inputs():
    manifest = definition()
    assert set(manifest) == {"service", "vars", "inputs"}
    assert set(manifest["service"]) == {"bridge-web"}
    service = manifest["service"]["bridge-web"]
    assert service["kind"] == "repo"
    assert service["railway_service"] == "bridge-web"
    assert service["dockerfile"] == str(RECIPE.relative_to(ROOT))
    assert service["root"] == "/"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 1
    assert service["public"] == 1
    assert service["order"] == 14
    assert service["needs"] == []
    assert "volume" not in service
    watches = service["watch"]
    for watch in watches:
        assert watch.endswith("/**")
        assert (ROOT / watch.removesuffix("/**")).is_dir()
    inputs = {str(RECIPE.relative_to(ROOT))}
    for line in RECIPE.read_text().splitlines():
        if not line.startswith("COPY ") or "--from=" in line:
            continue
        for source in shlex.split(line)[1:-1]:
            path = ROOT / source
            assert path.exists(), source
            inputs.add(source)
            if path.is_dir():
                inputs.update(str(child.relative_to(ROOT)) for child in path.rglob("*") if child.is_file())
    assert all(any(fnmatch.fnmatchcase(path, watch) for watch in watches) for path in inputs)


def test_interop_bridge_web_health_and_runtime():
    manifest = definition()
    service = manifest["service"]["bridge-web"]
    assert service["health"] == "/"
    page = APP / "src/app/page.tsx"
    assert "export default" in page.read_text()
    config = (APP / "next.config.ts").read_text()
    assert re.search(r'''output:\s*['"]standalone['"]''', config)
    assert "outputFileTracingRoot: import.meta.dirname" in config
    assert "cacheComponents: true" in config
    layout = (APP / "src/app/layout.tsx").read_text()
    assert re.search(r"<Suspense\b[\s\S]*?<Providers>[\s\S]*?</Providers>\s*</Suspense>", layout)
    recipe = RECIPE.read_text()
    assert "ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0" in recipe
    assert manifest["vars"]["bridge-web"]["PORT"] == "3000"
    assert 'CMD ["node", "server.js"]' in recipe
    assert "USER 65532:65532" in recipe
    bases = re.findall(r"^FROM\s+(\S+)", recipe, re.M)
    assert len(bases) == 2
    assert all(re.fullmatch(r"node:[^@]+@sha256:[0-9a-f]{64}", base) for base in bases)
    assert "/.next/standalone ./" in recipe
    assert "/.next/static ./.next/static" in recipe
    assert "/public ./public" in recipe
    manifest = json.loads((APP / "package.json").read_text())
    assert manifest["scripts"]["build"] == "next build"
    assert "pnpm install --frozen-lockfile" in recipe
    assert "RUN npm run build" in recipe
    pin = manifest["packageManager"].split("+", 1)[0]
    assert f"--package={pin}" in recipe


def test_interop_bridge_web_image_inventory():
    script = '''
set -euo pipefail
source services/platform/hosted/tests/beta-images.sh
printf '%s\\n' "${IMAGE_NAMES[@]}"
image_source bridge-web
printf '\\n'
image_context bridge-web
printf '\\n'
image_target bridge-web
printf '\\n'
image_build_args bridge-web
'''
    result = subprocess.run(["bash", "-c", script], cwd=ROOT, text=True, capture_output=True, check=True)
    lines = result.stdout.splitlines()
    assert lines.count("bridge-web") == 1
    assert "ghcr.io/sidiora-labs/bridge-web:0.1.0 docker/bridge-web/Dockerfile" in lines
    assert "." in lines
    assert "runtime" in lines
    check = (ROOT / "tools/containers/check.sh").read_text()
    assert "bridge-web|docker/bridge-web/Dockerfile|docker/bridge-web/Dockerfile|." in check
