"""Keep first-party repository links and citation metadata canonical."""

from datetime import date
import json
from pathlib import Path
import re
import subprocess
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[2]
CANONICAL_REPOSITORY = "https://github.com/Sidiora-Labs/paxeer-monorepo"
PACKAGE_MANIFESTS = (
    "sdk/integrations/agents/package.json",
    "sdk/integrations/express/package.json",
    "sdk/integrations/next/package.json",
    "services/agent/sdk/typescript/package.json",
    "services/platform/middleware/agent/package.json",
    "services/platform/middleware/buyer/package.json",
    "services/platform/middleware/merchant/package.json",
    "services/platform/middleware/seller/package.json",
)
MAVEN_MANIFESTS = (
    "sdk/integrations/android/pom.xml",
    "sdk/integrations/spring/pom.xml",
    "sdk/platform/jvm/pom.xml",
)
VENDORED_TREES = (
    "core/deps/",
    "programs/vendor/",
    "sdk/js/",
    "tools/loadtest/",
    "tools/paxctl/",
    "tools/uci/",
    "tools/wasm-tools/",
    "tools/hyperfide/",
    "docs/hyperfide/",
    "apps/explorer/",
    "services/explorer/",
    "tools/benchmark/loadtest/contracts/evm/lib/",
    "apps/inference-peer/engine/",
)
# This names the dependency's upstream source, not the first-party repository.
PROVENANCE_ALLOWLIST = {
    (
        "wasm-runtime/README.md",
        "https://github.com/Paxeer-Network/pax-cosmwasm",
    ): "[Paxeer-Network/pax-cosmwasm](https://github.com/Paxeer-Network/pax-cosmwasm)",
}
GITHUB_URL = re.compile(
    r"(?:(?:https?|git|ssh)://(?:git@)?|git@)github\.com[/:]"
    r"(?P<owner>[A-Za-z0-9_.-]+)/(?P<repository>[A-Za-z0-9_.-]+)"
    r"[^\s<>\]\)\"'`]*",
    re.IGNORECASE,
)
LEGACY_REPOSITORIES = {
    "paxeer-x-network",
    "layerx-network",
    "layerx-protocol",
    "kindlelaunch",
    "paxeer-network",
}
LEGACY_OWNERS = {"layerx-protocol", "kindlelaunch", "paxeer-network", "sei-protocol"}


def test_manifest_repository_urls_are_canonical():
    for name in PACKAGE_MANIFESTS:
        manifest = json.loads((ROOT / name).read_text(encoding="utf-8"))
        directory = Path(name).parent.as_posix()
        assert manifest["repository"] == {
            "type": "git",
            "url": f"git+{CANONICAL_REPOSITORY}.git",
            "directory": directory,
        }, name
        assert manifest["homepage"] == (
            f"{CANONICAL_REPOSITORY}/tree/main/{directory}"
        ), name

    namespace = {"m": "http://maven.apache.org/POM/4.0.0"}
    for name in MAVEN_MANIFESTS:
        manifest = ET.parse(ROOT / name).getroot()
        directory = Path(name).parent.as_posix()
        expected = {
            "m:url": f"{CANONICAL_REPOSITORY}/tree/main/{directory}",
            "m:scm/m:url": CANONICAL_REPOSITORY,
            "m:scm/m:connection": f"scm:git:{CANONICAL_REPOSITORY}.git",
            "m:scm/m:developerConnection": (
                "scm:git:ssh://git@github.com/Sidiora-Labs/paxeer-monorepo.git"
            ),
            "m:licenses/m:license/m:url": f"{CANONICAL_REPOSITORY}/blob/main/LICENSE",
        }
        for field, url in expected.items():
            assert manifest.findtext(field, namespaces=namespace) == url, (
                f"{name}: {field}"
            )


def test_docs_link_canonical_repository():
    tracked = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    pages = [
        name
        for name in tracked
        if Path(name).suffix.lower() in {".md", ".mdx", ".rst"}
        and not name.startswith((*VENDORED_TREES, "spec/"))
        and not {"vendor", "node_modules"}.intersection(Path(name).parts)
    ]
    assert "docs/site/docs/index.md" in pages
    assert "docs/site/docs/overview/i18n/ja.md" in pages
    failures = []
    references = 0
    for name in pages:
        for number, line in enumerate(
            (ROOT / name).read_text(encoding="utf-8").splitlines(), 1
        ):
            for match in GITHUB_URL.finditer(line):
                url = match.group()
                origin = PROVENANCE_ALLOWLIST.get((name, url))
                if origin is not None and line.strip() == origin:
                    continue
                owner = match["owner"].lower()
                repository = match["repository"].lower().removesuffix(".git")
                if repository in LEGACY_REPOSITORIES or owner in LEGACY_OWNERS:
                    failures.append(f"{name}:{number}: {url}")
                if url.startswith(CANONICAL_REPOSITORY):
                    references += 1

    assert references, "No canonical repository URLs found in first-party docs"
    assert not failures, "Noncanonical repository URLs:\n" + "\n".join(failures)


def test_citation_version_matches_release():
    release = json.loads(
        (ROOT / "tools/release/version.json").read_text(encoding="utf-8")
    )
    citation = (ROOT / "CITATION.cff").read_text(encoding="utf-8")
    fields = dict(re.findall(r"^([a-z-]+): (.+)$", citation, flags=re.M))
    assert fields["version"] == release["version"]
    assert fields["repository-code"] == CANONICAL_REPOSITORY
    released = date.fromisoformat(fields["date-released"])
    assert released.isoformat() == fields["date-released"]
    assert released <= date.today(), "Citation release date is in the future"
