"""Keep visitor assets small and catalogue asset routes backed by tracked sources."""

import json
import os
import struct
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.parse import unquote

import pytest


REPO_ROOT = Path(__file__).resolve().parents[2]
MAX_ASSET_BYTES = 500_000
PUBLIC_ROOTS = {
    "wallet-ui": ("/wallet/", "apps/wallet/public"),
    "explorer": ("/explorer/", "apps/explorer/public"),
}
OPTIMIZED_ASSETS = {
    "apps/launchpad/web/public/shadcn.png": (128, 101),
    "apps/launchpad/web/public/Kindle-Launch-logo-dark.png": (192, 192),
    "apps/launchpad/web/public/icons/logo.svg": (42, 42),
    "apps/launchpad/web/src/app/icon.svg": (64, 64),
    "apps/wallet/public/PAXPORT.png": (1080, 1595),
    "apps/wallet/public/PAXPORT.svg": (256, 192),
    "apps/wallet/public/wallet.svg": (384, 288),
    "apps/wallet/public/paxeerCOEX_light.svg": (256, 128),
}
LARGE_ASSET_ALLOWLIST = {
    "apps/explorer/public/openapi.yaml": (631_987, "Published API schema"),
    "apps/explorer/public/static/fonts/Inter-fallback.sfd": (
        3_912_143,
        "Existing font source",
    ),
    "apps/explorer/public/static/og_placeholder.png": (
        577_355,
        "Social preview image",
    ),
    "apps/launchpad/web/public/Kindle-Launch-logo-light.png": (
        1_531_943,
        "Existing light brand export",
    ),
    "apps/launchpad/web/public/Welcome_banner.png": (
        1_643_539,
        "Existing welcome artwork",
    ),
    "apps/launchpad/web/public/charting_library/bundles/library.f5eaeb6901219f861981.js": (
        2_801_934,
        "Vendored charting runtime",
    ),
    "apps/launchpad/web/public/icons/airdrop_background.png": (
        787_624,
        "Existing airdrop artwork",
    ),
    "apps/perps/public/Dark.png": (859_279, "Existing theme preview"),
    "apps/perps/public/Hotline.png": (840_119, "Existing theme preview"),
    "apps/perps/public/Model_Compressed.glb": (2_074_912, "Existing 3D model"),
    "apps/perps/public/OLED.png": (859_279, "Existing theme preview"),
    "apps/perps/public/assets/airdrop_background.png": (
        787_624,
        "Existing airdrop artwork",
    ),
    "apps/perps/public/assets/logo.svg": (1_626_439, "Existing perps brand export"),
    "apps/perps/public/image.png": (1_842_773, "Existing perps artwork"),
    "apps/perps/public/static/charting_library/charting_library.d.ts": (
        1_023_620,
        "Vendored charting declarations",
    ),
    "apps/wallet/public/6751fc7c-8b71-48f5-b454-c34299955eb3.png": (
        1_680_659,
        "Existing wallet artwork",
    ),
    "apps/wallet/public/a84ac5ac-5ebe-4f96-a9d1-4b3b664db1a5.png": (
        1_627_711,
        "Existing wallet artwork",
    ),
    "apps/wallet/public/icons/ios/1024.png": (502_330, "Store icon export"),
    "apps/wallet/public/icons/windows/LargeTile.scale-400.png": (
        658_980,
        "Windows tile export",
    ),
    "apps/wallet/public/icons/windows/SplashScreen.scale-400.png": (
        654_516,
        "Windows splash export",
    ),
    "apps/wallet/public/paxeerCOEX.svg": (911_172, "Existing dark brand export"),
    "apps/wallet/public/paxport_wallet.png": (1_651_061, "Existing wallet artwork"),
    "apps/wallet/public/points.svg": (803_594, "Existing points artwork"),
}
GENERATED_ASSET_SOURCES = {
    "/explorer/assets/envs.js": (
        "apps/explorer/deploy/scripts/make_envs_script.sh",
    ),
    "/explorer/robots.txt": (
        "apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js",
    ),
    "/explorer/sitemap.xml": (
        "apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js",
    ),
    "/wallet/sw.js": ("apps/wallet/src/pwa/sw.ts", "apps/wallet/src/pwa/build.ts"),
    "/wallet/_next/static/{asset}": ("apps/wallet/next.config.mjs",),
    "/explorer/_next/static/{asset}": ("apps/explorer/next.config.js",),
    "/human-ui/_next/static/{asset}": ("apps/web/next.config.mjs",),
    "/explorer/assets/configs/{asset}": (
        "apps/explorer/deploy/scripts/download_assets.sh",
        "apps/explorer/deploy/scripts/entrypoint.sh",
    ),
    "/explorer/assets/multichain/{asset}": (
        "apps/explorer/deploy/tools/multichain-config-generator/index.ts",
    ),
    "/explorer/assets/essential-dapps/{asset}": (
        "apps/explorer/deploy/tools/essential-dapps-chains-config-generator/index.ts",
    ),
    "/explorer/icons/{asset}": (
        "apps/explorer/deploy/scripts/build_sprite.sh",
    ),
}


@pytest.fixture(scope="module")
def tracked_paths() -> set[str]:
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--full-name", "-z"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
    )
    paths = {os.fsdecode(path) for path in result.stdout.split(b"\0") if path}
    assert paths, "Git reported no tracked files"
    return paths


def is_visitor_asset(name: str) -> bool:
    parts = Path(name).parts
    if len(parts) < 4 or parts[0] != "apps":
        return False
    for offset in (2, 3):
        if parts[offset : offset + 1] == ("public",):
            return True
        if parts[offset : offset + 2] == ("src", "assets"):
            return True
    return False


def test_public_assets_under_weight_cap(tracked_paths: set[str]):
    assets = {name for name in tracked_paths if is_visitor_asset(name)}
    assets.update(OPTIMIZED_ASSETS)
    assert assets, "No visitor assets were found"
    assert not set(OPTIMIZED_ASSETS).intersection(LARGE_ASSET_ALLOWLIST)
    for name, (budget, reason) in LARGE_ASSET_ALLOWLIST.items():
        assert name in assets, f"Stale asset allowance: {name}"
        assert reason.strip() and budget > MAX_ASSET_BYTES
        assert (REPO_ROOT / name).stat().st_size > MAX_ASSET_BYTES, (
            f"Remove the allowance for the optimized asset: {name}"
        )
    offenders = []
    for name in sorted(assets):
        assert name in tracked_paths, f"Asset is not tracked: {name}"
        path = REPO_ROOT / name
        assert path.is_file() and not path.is_symlink(), name
        size = path.stat().st_size
        budget = LARGE_ASSET_ALLOWLIST.get(name, (MAX_ASSET_BYTES, ""))[0]
        if size > budget:
            offenders.append(f"{name}: {size} bytes; maximum {budget}")
    assert not offenders, "Visitor asset weight budget exceeded:\n" + "\n".join(offenders)

    for name, dimensions in OPTIMIZED_ASSETS.items():
        data = (REPO_ROOT / name).read_bytes()
        if name.endswith(".png"):
            assert data[:8] == b"\x89PNG\r\n\x1a\n", name
            assert data[12:16] == b"IHDR", name
            assert struct.unpack(">II", data[16:24]) == dimensions, name
        else:
            root = ET.fromstring(data)
            assert root.tag == "{http://www.w3.org/2000/svg}svg", name
            assert (
                int(root.attrib["width"]), int(root.attrib["height"])
            ) == dimensions, name
            assert root.findall(".//{http://www.w3.org/2000/svg}path"), name
            assert not root.findall(".//{http://www.w3.org/2000/svg}image"), name
            assert b"data:image" not in data, name


def test_route_catalogue_assets_exist(tracked_paths: set[str]):
    catalogue = json.loads((REPO_ROOT / "rpc/px_route_catalogue.json").read_text())
    routes = catalogue["routes"]
    assert routes, "The route catalogue must not be empty"
    asset_routes = [
        route
        for route in routes
        if route.get("ui_route_kind") in ("asset", "generated-asset")
    ]
    assert asset_routes, "The catalogue must declare asset routes"
    seen_sources = set()
    seen_generated = set()
    for route in asset_routes:
        url_path = route["path"]
        assert route["id"] == f"{route['service']}:{route['method']}:{url_path}"
        if route["ui_route_kind"] == "generated-asset":
            assert route["asset_prefix"] and route["asset_suffixes"], route["id"]
            assert route["asset_max_path_bytes"] > 0, route["id"]
            if url_path in GENERATED_ASSET_SOURCES:
                sources = GENERATED_ASSET_SOURCES[url_path]
                seen_generated.add(url_path)
            else:
                assert url_path.endswith("{asset}"), route["id"]
                prefix, public_root = PUBLIC_ROOTS[route["service"]]
                asset_prefix = route["asset_prefix"]
                assert asset_prefix.startswith(prefix), route["id"]
                directory = public_root + "/" + unquote(asset_prefix[len(prefix):])
                assert ".." not in Path(directory).parts, route["id"]
                sources = [
                    name for name in tracked_paths if name.startswith(directory)
                ]
                assert sources, f"No tracked assets for {route['id']}"
        else:
            prefix, public_root = PUBLIC_ROOTS[route["service"]]
            assert url_path.startswith(prefix), route["id"]
            relative = unquote(url_path[len(prefix):])
            assert relative and not Path(relative).is_absolute(), route["id"]
            assert ".." not in Path(relative).parts, route["id"]
            sources = [f"{public_root}/{relative}"]
            seen_sources.update(sources)
        for source in sources:
            assert source in tracked_paths, f"{route['id']}: untracked source {source}"
            assert (REPO_ROOT / source).is_file(), f"{route['id']}: missing {source}"
    assert seen_generated == set(GENERATED_ASSET_SOURCES)
    wallet_assets = {
        name for name in OPTIMIZED_ASSETS if name.startswith("apps/wallet/")
    }
    assert wallet_assets <= seen_sources, "Optimized wallet assets lost their routes"
