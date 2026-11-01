import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]


def recipe(makefile, target):
    match = re.search(
        rf"^{re.escape(target)}:[^\n]*\n((?:\t[^\n]*\n)+)", makefile, re.MULTILINE
    )
    assert match is not None, f"Missing recipe for {target}"
    return [line.strip() for line in match.group(1).splitlines()]


def test_human_build_builds_sdk_first():
    makefile = (ROOT / "Makefile").read_text()
    assert recipe(makefile, "human-build")[0] == "$(MAKE) agent-sdk-ts-build"
    sdk_recipe = recipe(makefile, "agent-sdk-ts-build")
    assert sdk_recipe[0] == (
        "@if [ ! -d services/agent/sdk/typescript/node_modules ]; then \\"
    )
    assert (
        "npm --prefix services/agent/sdk/typescript ci "
        "--ignore-scripts --no-audit --no-fund; \\"
    ) in sdk_recipe
    assert sdk_recipe[-1] == "npm --prefix services/agent/sdk/typescript run build"


def test_web_image_builds_agent_sdk_before_web_build():
    lines = (ROOT / "docker/web/Dockerfile").read_text().splitlines()
    sdk_install = lines.index(
        "RUN npm --prefix /src/services/agent/sdk/typescript ci "
        "--ignore-scripts --no-audit --no-fund"
    )
    sdk_build = lines.index(
        "RUN npm --prefix /src/services/agent/sdk/typescript run build"
    )
    web_copy = lines.index("COPY apps/web /src/apps/web")
    web_build = lines.index(
        'RUN LAYERX_HUMAN_SERVICE_URL="$LAYERX_HUMAN_SERVICE_URL" npm run build'
    )
    assert sdk_build == sdk_install + 1
    assert sdk_install < sdk_build < web_copy < web_build


def test_sdk_exports_have_default_condition():
    package = json.loads(
        (ROOT / "services/agent/sdk/typescript/package.json").read_text()
    )
    assert package["main"] == "./dist/src/index.js"
    assert package["types"] == "./dist/src/index.d.ts"
    for subpath, name in ((".", "index"), ("./browser", "browser")):
        conditions = package["exports"][subpath]
        assert conditions["types"] == f"./dist/src/{name}.d.ts"
        assert conditions["import"] == f"./dist/src/{name}.js"
        assert conditions["default"] == conditions["import"]
