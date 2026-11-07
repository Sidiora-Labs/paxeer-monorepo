"""Regression coverage for public documentation links and package ownership."""

import json
import re
from collections import Counter
from pathlib import Path
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
REPOSITORY = "https://github.com/Sidiora-Labs/paxeer-monorepo"
COMETBFT = "https://github.com/cometbft/cometbft/blob/main/"
VERIFICATION = "consensus/spec/light-client/verification/verification_001_published.md"
PBTS = "consensus/spec/consensus/proposer-based-timestamp/v1/pbts_001_draft.md"
EVIDENCE = "sdk/x/evidence/spec/06_begin_block.md"
COOKIE_POLICY = "apps/perps/src/page-modules/legal/CookiePolicy.tsx"
TOOLKIT = "apps/explorer/toolkit/package/package.json"
REGISTRY = "services/platform/hosted/registry/README.md"
KEYRING = "tools/paxctl/sidecar/docs/keyring.md"
OLD_TARGETS = {
    "docs/site/mkdocs.yml": ("LayerX-Network", "Paxeer-X-Network"),
    "sdk/js/package.json": ("pax-protocol/pax-js",),
    "sdk/js/README.md": (
        "](./dist/",
        "https://github.com/Paxeer-Network/pax-js",
        "checked-in `dist` files",
    ),
    "contracts/sdk-js.md": (
        "](../sdk/js/dist/",
        "implementation is checked in",
    ),
    VERIFICATION: ("github.com/tendermint/spec/", "docs.tendermint.com/"),
    PBTS: ("github.com/tendermint/spec/", "rust-spec/tendermint-accountability"),
    EVIDENCE: ("github.com/tendermint/tendermint/",),
    KEYRING: ("bdchatham-designs", "PLT-497", "in-pod-governance-signing.md"),
    COOKIE_POLICY: ("delete-cookies-in-microsoft-edge-63947406",),
    TOOLKIT: ("github.com/blockscout/frontend",),
    "docs/explorer/ENVS.md": ("https://github.com/varld/gradient-avatar",),
    REGISTRY: ("fly.toml", "Fly.io"),
}


def read(relative_path):
    return (ROOT / relative_path).read_text(encoding="utf-8")


def markdown_targets(text):
    return re.findall(r"\]\(([^\s)]+)\)", text)


def assert_local_target(document, target):
    """Check the actual linked file and GitHub-style Markdown heading, if any."""
    parsed = urlsplit(target)
    assert not parsed.scheme and not parsed.netloc, target
    destination = ((ROOT / document).parent / parsed.path).resolve()
    assert destination.is_relative_to(ROOT), target
    assert destination.exists(), target
    if parsed.fragment:
        headings = re.findall(
            r"^#{1,6}\s+(.+)$", destination.read_text(encoding="utf-8"), re.MULTILINE
        )
        slugs = {
            re.sub(r"\s+", "-", re.sub(r"[^\w\s-]", "", heading.lower()))
            for heading in headings
        }
        assert parsed.fragment in slugs, target


@pytest.mark.parametrize("document,old_targets", OLD_TARGETS.items())
def test_stale_targets_removed_from_every_edited_document(document, old_targets):
    text = read(document)
    for old in old_targets:
        assert old not in text, (document, old)


def test_documentation_site_uses_monorepo_and_its_pages_address():
    settings = dict(
        re.findall(
            r"^(site_url|repo_url|repo_name):\s*(\S+)$",
            read("docs/site/mkdocs.yml"),
            re.MULTILINE,
        )
    )
    assert settings == {
        "site_url": "https://sidiora-labs.github.io/paxeer-monorepo/",
        "repo_url": REPOSITORY,
        "repo_name": "Sidiora-Labs/paxeer-monorepo",
    }


@pytest.mark.parametrize(
    "manifest,directory",
    [
        ("sdk/js/package.json", "sdk/js"),
        (TOOLKIT, "apps/explorer/toolkit/package"),
    ],
)
def test_package_metadata_names_the_owning_repository_and_existing_directory(
    manifest, directory
):
    package = json.loads(read(manifest))
    assert package["repository"] == {
        "type": "git",
        "url": f"{REPOSITORY}.git",
        "directory": directory,
    }
    homepage = urlsplit(package["homepage"])
    assert homepage.scheme == "https" and homepage.netloc == "github.com"
    assert homepage.path == f"/Sidiora-Labs/paxeer-monorepo/tree/main/{directory}"
    assert (ROOT / directory / "package.json").is_file()
    if manifest == TOOLKIT:
        assert package["bugs"]["url"] == f"{REPOSITORY}/issues"


def test_all_seven_sdk_links_resolve_to_existing_package_contracts():
    replacements = {
        "sdk/js/README.md": ["../../contracts/sdk-js.md#encoding-tokenfactory"],
        "contracts/sdk-js.md": [
            "../sdk/js/README.md#pax-jscosmosencoding",
            "../sdk/js/package.json",
            "../sdk/js/README.md#usage-with-cosmjsstargate",
            "../sdk/js/README.md#pax-jscosmostypes",
            "../sdk/js/README.md#pax-jscosmostypes",
            "../sdk/js/README.md#pax-jscosmosrest",
        ],
    }
    assert sum(map(len, replacements.values())) == 7
    for document, targets in replacements.items():
        text = read(document)
        links = Counter(markdown_targets(text))
        for target, count in Counter(targets).items():
            assert links[target] >= count, (document, target)
            assert_local_target(document, target)
        assert not any("/dist/" in target for target in links), document
        assert f"{REPOSITORY}/tree/main/sdk/js" in links, document
        assert "absent from this checkout" in text or "does not contain" in text


def test_eight_archived_consensus_links_point_to_matching_cometbft_pages():
    references = dict(
        re.findall(r"^\[([^\]]+)\]:\s*(\S+)$", read(VERIFICATION), re.MULTILINE)
    )
    remote_functions = (
        "spec/light-client/verification/verification_001_published.md"
        "#used-remote-functions"
    )
    expected = {
        "block": "spec/core/data_structures.md",
        "fullnode": remote_functions,
        "FN-LuckyCase-link": remote_functions,
        "blockchain-validator-set": "spec/core/data_structures.md#validatorset",
        "fullnode-data-structures": "spec/core/data_structures.md#lightblock",
        "FN-ManifestFaulty-link": remote_functions,
    }
    for name, path in expected.items():
        assert references[name] == f"{COMETBFT}{path}"
    assert references["RPC"] == "https://docs.cometbft.com/v0.38/rpc/"
    assert (
        f"[tlatender]: {COMETBFT}spec/light-client/accountability/README.md"
        in read(PBTS)
    )
    assert (
        f"[Evidence]({COMETBFT}spec/core/data_structures.md#evidence)"
        in read(EVIDENCE)
    )
    assert sum(
        read(path).count(COMETBFT) for path in (VERIFICATION, PBTS, EVIDENCE)
    ) == 8


def test_keyring_keeps_operator_instructions_without_private_repository_links():
    text = read(KEYRING)
    assert "## Operator runbook" in text
    assert "Kubernetes Secret" in text
    for variable in (
        "SEI_KEYRING_BACKEND", "SEI_KEYRING_DIR", "SEI_KEYRING_PASSPHRASE"
    ):
        assert variable in text.split("## Operator runbook", 1)[1]
    assert "Mount the keyring read-only" in text
    assert "restarting the sidecar pod" in text
    assert "https://github.com/" not in text


def test_edge_cookie_link_uses_current_support_article():
    text = read(COOKIE_POLICY)
    match = re.search(r"<a\s+([^>]+)>Microsoft Edge</a>", text)
    assert match is not None
    attributes = dict(re.findall(r'([\w-]+)="([^"]*)"', match.group(1)))
    assert attributes["href"] == (
        "https://support.microsoft.com/en-us/edge/"
        "manage-cookies-in-microsoft-edge-view-allow-block-delete-and-use"
    )
    assert attributes["target"] == "_blank"
    assert set(attributes["rel"].split()) == {"noopener", "noreferrer"}


def test_avatar_option_is_preserved_without_the_dead_link():
    row = next(
        line
        for line in read("docs/explorer/ENVS.md").splitlines()
        if line.startswith("| NEXT_PUBLIC_VIEWS_ADDRESS_IDENTICON_TYPE |")
    )
    assert '"gradient_avatar"' in row
    assert "Gradient Avatar" in row
    assert "[Gradient Avatar](" not in row


def test_registry_deployment_documentation_points_to_existing_files():
    text = read(REGISTRY)
    assert "Deploy the service on Kubernetes" in text
    links = markdown_targets(text)
    for target in (
        "deployment.yaml",
        "journal-pvc.yaml",
        "../../../../docker/platform-registry/Dockerfile",
    ):
        assert target in links
        assert_local_target(REGISTRY, target)
    deployment = (ROOT / REGISTRY).parent / "deployment.yaml"
    assert "kind: StatefulSet" in deployment.read_text(encoding="utf-8")
    assert "name: layerx-program-registry" in deployment.read_text(encoding="utf-8")
