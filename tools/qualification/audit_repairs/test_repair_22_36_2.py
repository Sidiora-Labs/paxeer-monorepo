"""Offline checks for the platform documentation and executable sample relocation."""

import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
DOCS = ROOT / "docs/platform"
SAMPLES = ROOT / "services/platform/examples/samples"
OLD_DOCS = "services/platform/docs"
NODE_SAMPLES = ("first-payment-typescript", "paid-endpoint-express", "paid-route-next")
SAMPLE_ENTRIES = {
    "first-payment-csharp": "Program.cs",
    "first-payment-go": "main.go",
    "first-payment-jvm": "src/main/java/com/sidiora/layerx/docs/FirstPayment.java",
    "first-payment-python": "main.py",
    "first-payment-swift": "Sources/FirstPayment/FirstPayment.swift",
    "first-payment-typescript": "index.mjs",
    "mobile-payment-android": "src/main/java/com/sidiora/layerx/docs/MobilePayment.java",
    "mobile-payment-ios": "Sources/MobilePayment/MobilePayment.swift",
    "paid-endpoint-express": "index.mjs",
    "paid-endpoint-fastapi": "app.py",
    "paid-endpoint-spring": "src/main/java/com/sidiora/layerx/docs/PaidApiApplication.java",
    "paid-route-next": "lib/layerx.js",
    "verify-receipt-rust": "src/main.rs",
}
MOVED_DOCUMENTS = (
    ".gitignore",
    "README.md",
    "beta-environment.md",
    "beta-resets.ics",
    "build/build_site.py",
    "build/test_beta_surfaces.py",
    "capabilities.kvx",
    "content/beta.md",
    "content/concepts/agents.md",
    "content/concepts/enforcement.md",
    "content/concepts/idempotency.md",
    "content/concepts/money.md",
    "content/concepts/paying.md",
    "content/concepts/receipts.md",
    "content/environments/emulator.md",
    "content/framework/android.md",
    "content/framework/express.md",
    "content/framework/fastapi.md",
    "content/framework/ios.md",
    "content/framework/next.md",
    "content/framework/spring.md",
    "content/guide/agent-middleware.md",
    "content/guide/buyer-middleware.md",
    "content/guide/interop.md",
    "content/guide/merchant-middleware.md",
    "content/guide/programs.md",
    "content/guide/receipts.md",
    "content/guide/reference-applications.md",
    "content/guide/seller-middleware.md",
    "content/guide/webhooks.md",
    "content/index.md",
    "content/install.md",
    "content/quickstart/csharp.md",
    "content/quickstart/go.md",
    "content/quickstart/jvm.md",
    "content/quickstart/python.md",
    "content/quickstart/rust.md",
    "content/quickstart/swift.md",
    "content/quickstart/typescript.md",
    "content/reference/agent-api.md",
    "content/reference/enforcement.md",
    "content/reference/errors.md",
    "content/reference/human-api.md",
    "content/reference/samples.md",
    "samples.kvx",
    "site.kvx",
)
MOVED_SAMPLE_FILES = (
    "first-payment-csharp/FirstPayment.csproj",
    "first-payment-csharp/Program.cs",
    "first-payment-go/go.mod",
    "first-payment-go/main.go",
    "first-payment-jvm/pom.xml",
    "first-payment-jvm/src/main/java/com/sidiora/layerx/docs/FirstPayment.java",
    "first-payment-python/layerx_transport.py",
    "first-payment-python/main.py",
    "first-payment-python/requirements.txt",
    "first-payment-swift/Package.swift",
    "first-payment-swift/Sources/FirstPayment/FirstPayment.swift",
    "first-payment-typescript/index.mjs",
    "first-payment-typescript/package.json",
    "mobile-payment-android/pom.xml",
    "mobile-payment-android/src/main/java/com/sidiora/layerx/docs/MobilePayment.java",
    "mobile-payment-ios/Package.swift",
    "mobile-payment-ios/Sources/MobilePayment/MobilePayment.swift",
    "paid-endpoint-express/fulfillments.mjs",
    "paid-endpoint-express/index.mjs",
    "paid-endpoint-express/package.json",
    "paid-endpoint-express/resource.json",
    "paid-endpoint-fastapi/app.py",
    "paid-endpoint-fastapi/requirements.txt",
    "paid-endpoint-fastapi/resource.json",
    "paid-endpoint-spring/pom.xml",
    "paid-endpoint-spring/resource.json",
    "paid-endpoint-spring/src/main/java/com/sidiora/layerx/docs/PaidApiApplication.java",
    "paid-endpoint-spring/src/main/resources/application.yaml",
    "paid-route-next/.env.example",
    "paid-route-next/.gitignore",
    "paid-route-next/app/layerx/webhooks/route.js",
    "paid-route-next/app/paid/route.js",
    "paid-route-next/lib/fulfillments.js",
    "paid-route-next/lib/layerx.js",
    "paid-route-next/next.config.mjs",
    "paid-route-next/package.json",
    "paid-route-next/resource.json",
    "verify-receipt-rust/Cargo.lock",
    "verify-receipt-rust/Cargo.toml",
    "verify-receipt-rust/src/main.rs",
)
NPM_BASELINE_SHA256 = "a733ff574a1ef96698f6cd3a73cc3df76567d8aab8586ac615afb258c154c661"
FENCED_CODE = re.compile(r"^(`{3,}|~{3,})[^\n]*\n.*?^\1[ \t]*$", re.M | re.S)
MARKDOWN_LINK = re.compile(r"!?\[[^\]]*\]\(\s*(<[^>]+>|[^\s)]+)(?:\s+[^)]*)?\)")
REFERENCE_LINK = re.compile(r"^\s{0,3}\[[^\]]+\]:\s*(<[^>]+>|\S+)", re.M)


@pytest.fixture(scope="module")
def builder():
    path = DOCS / "build/build_site.py"
    spec = importlib.util.spec_from_file_location("platform_documentation_builder", path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def test_documentation_tree_and_all_thirteen_samples_moved():
    assert not (ROOT / OLD_DOCS).exists()
    for name in MOVED_DOCUMENTS:
        assert (DOCS / name).is_file(), name
    for name in MOVED_SAMPLE_FILES:
        assert (SAMPLES / name).is_file(), name
    assert not (DOCS / "samples").exists()
    assert not (DOCS / ".gitkeep").exists()
    assert {path.name for path in SAMPLES.iterdir() if path.is_dir()} == set(SAMPLE_ENTRIES)
    for name, entry in SAMPLE_ENTRIES.items():
        assert (SAMPLES / name / entry).is_file(), (name, entry)
    assert (SAMPLES / "paid-route-next/.env.example").is_file()
    assert (SAMPLES / "verify-receipt-rust/Cargo.lock").is_file()
    assert (SAMPLES / ".gitignore").is_file()
    published = set(subprocess.run(
        ["git", "ls-files", "--", "docs/platform", "services/platform/examples/samples"],
        cwd=ROOT, check=True, capture_output=True, text=True,
    ).stdout.splitlines())
    expected = {f"docs/platform/{name}" for name in MOVED_DOCUMENTS}
    expected |= {f"services/platform/examples/samples/{name}" for name in MOVED_SAMPLE_FILES}
    assert expected <= published, "Moved files missing from the candidate: " + ", ".join(sorted(expected - published))


def test_node_workspace_entries_and_npm_lock_preserve_resolutions():
    expected = {f"services/platform/examples/samples/{name}" for name in NODE_SAMPLES}
    manifest = json.loads((ROOT / "package.json").read_text())
    assert manifest["packageManager"] == "npm@11.19.1"
    assert expected <= set(manifest["workspaces"])
    assert not (ROOT / "pnpm-workspace.yaml").exists()
    assert not (ROOT / "pnpm-lock.yaml").exists()
    lock_bytes = (ROOT / "package-lock.json").read_bytes()
    assert hashlib.sha256(lock_bytes).hexdigest() == NPM_BASELINE_SHA256
    lock = json.loads(lock_bytes)
    assert lock["packages"][""]["workspaces"] == manifest["workspaces"]
    for path in expected:
        package = json.loads((ROOT / path / "package.json").read_text())
        assert lock["packages"][path]["name"] == package["name"]
        assert lock["packages"][f"node_modules/{package['name']}"] == {"resolved": path, "link": True}


def test_no_tracked_file_retains_the_old_documentation_path():
    tracked = subprocess.run(
        ["git", "ls-files", "-z"], cwd=ROOT, check=True, capture_output=True,
    ).stdout.decode().split("\0")
    forbidden = OLD_DOCS.encode()
    own_path = Path(__file__).resolve()
    offenders = []
    for name in tracked:
        if not name or name.startswith("spec/") or name == "GOTCHA.kvx":
            continue
        path = ROOT / name
        # Decision 2665: repair records and changelogs preserve historical paths.
        if (
            path.suffix == ".kvx" and path.stem.startswith(("repair_", "repair-"))
        ) or path.name.lower().startswith("changelog"):
            continue
        if path.resolve() == own_path or not path.is_file():
            continue
        if path.is_symlink():
            if OLD_DOCS in str(path.readlink()):
                offenders.append(name)
            continue
        with path.open("rb") as stream:
            tail = b""
            while chunk := stream.read(1024 * 1024):
                if forbidden in tail + chunk:
                    offenders.append(name)
                    break
                tail = chunk[-len(forbidden):]
    assert not offenders, "Old documentation paths remain in tracked files: " + ", ".join(offenders)


def test_every_relative_markdown_link_resolves():
    checked = 0
    problems = []
    for path in DOCS.rglob("*.md"):
        prose = FENCED_CODE.sub("", path.read_text())
        targets = MARKDOWN_LINK.findall(prose) + REFERENCE_LINK.findall(prose)
        for raw in targets:
            target = urlsplit(raw.strip("<>"))
            if target.scheme or target.netloc:
                continue
            checked += 1
            relative = unquote(target.path)
            destination = ROOT / relative.lstrip("/") if relative.startswith("/") else path.parent / relative
            if not destination.exists():
                problems.append(f"{path.relative_to(ROOT)} -> {raw}")
    assert checked > 30, "The documentation link inventory must be exercised"
    assert not problems, "Unresolved Markdown links: " + "; ".join(problems)


def test_site_generator_reads_moved_samples_and_renders_source_links(builder):
    samples = builder.load_samples(DOCS)
    assert set(samples) == set(SAMPLE_ENTRIES)
    measurements = builder.measure_samples(samples)
    assert len(measurements) == 13
    for identifier, sample in samples.items():
        assert sample.path.resolve() == SAMPLES / identifier
        assert sample.location == f"services/platform/examples/samples/{identifier}"
        assert sample.entry == SAMPLE_ENTRIES[identifier]
    capabilities = builder.load_capabilities(DOCS)
    for path, generated in builder.generated_documents(DOCS, ROOT, capabilities, samples, measurements).items():
        assert path.read_text() == generated, str(path.relative_to(ROOT))
    sections, pages, settings = builder.load_sections(DOCS)
    rendered = {
        identifier: builder.render_page(
            page, builder.render_markdown(builder.site_links(page.source.read_text(), page, pages)),
            sections, pages, settings,
        )
        for identifier, page in pages.items()
    }
    known = {identifier.replace("/", "-") + ".html" for identifier in pages} | {"search.js"}
    checked, problems = builder.check_links(pages, rendered, known)
    assert checked > len(pages)
    assert problems == []
    page = pages["quickstart/swift"]
    linked = builder.site_links("[iOS](../framework/ios.md?q=1#payment)", page, pages)
    assert linked == "[iOS](framework-ios.html?q=1#payment)"
    code = "```js\nconst example = '[iOS](../framework/ios.md)';\n```"
    assert builder.site_links(code, page, pages) == code


def test_workspace_and_gate_consumers_resolve_the_relocated_paths():
    for name in ("services/platform/workspace.kvx", "services/platform/tools/workspace-gate/src/workspace.kvx"):
        manifest = (ROOT / name).read_text()
        roots = json.loads(re.search(r"^roots = (.+)$", manifest, re.M).group(1))
        assert "docs/platform" in roots
        assert '[dir.docs]\npath = "docs/platform"' in manifest
        for value in re.findall(r'^path = "([^"]+)"$', manifest, re.M):
            assert (ROOT / value).is_dir(), value
    gate = (ROOT / "services/platform/tools/docs-gate").read_text()
    assert '"$repo_root/docs/platform/build/build_site.py" --check "$repo_root"' in gate
    for name in (
        "services/platform/Makefile.inc", "services/platform/tools/benchmark/src/main.rs",
        "sdk/integrations/next/example/test/fulfillment.test.mjs",
    ):
        assert "services/platform/examples/samples/" in (ROOT / name).read_text()


def test_pnpm_importers_and_registry_resolutions_match_the_npm_lock():
    # 22.39.104 removed the unused root pnpm workspace and lock. Validate
    # the complete npm importer graph and every registry resolution instead.
    assert not (ROOT / "pnpm-lock.yaml").exists()
    assert not (ROOT / "pnpm-workspace.yaml").exists()
    root_manifest = json.loads((ROOT / "package.json").read_text())
    lock_bytes = (ROOT / "package-lock.json").read_bytes()
    assert hashlib.sha256(lock_bytes).hexdigest() == NPM_BASELINE_SHA256
    lock = json.loads(lock_bytes)
    assert lock["lockfileVersion"] == 3
    packages = lock["packages"]
    workspaces = root_manifest["workspaces"]
    assert packages[""]["workspaces"] == workspaces
    assert set(NODE_SAMPLES) <= {Path(path).name for path in workspaces}
    for relative in ["", *workspaces]:
        manifest = json.loads((ROOT / relative / "package.json").read_text())
        importer = packages[relative]
        for group in ("dependencies", "devDependencies", "optionalDependencies",
                      "peerDependencies", "peerDependenciesMeta"):
            assert importer.get(group, {}) == manifest.get(group, {}), (relative, group)
        if relative:
            assert packages[f"node_modules/{manifest['name']}"] == {
                "resolved": relative, "link": True,
            }
    registry = {path: entry for path, entry in packages.items()
                if "node_modules/" in path and not entry.get("link")}
    assert registry
    for path, entry in registry.items():
        assert entry["version"], path
        assert entry["resolved"].startswith("https://registry.npmjs.org/"), path
        assert entry["integrity"].startswith("sha512-"), path
