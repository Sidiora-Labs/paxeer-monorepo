"""Keep hosted documentation free of deployment-specific infrastructure hosts."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
PAGES = (
    "contracts/operations.md",
    "deploy/launchpad/README.md",
    "docs/site/paxeer-docs/src/app/webhooks/page.tsx",
    "docs/wiki/HostedDashboard.md",
    "docs/wiki/HostedWebhooks.md",
    "services/explorer/deploy/railway/README.md",
)
PRIVATE_DNS = re.compile(r"\.(?:internal|svc\.cluster\.local)\b", re.IGNORECASE)
PROVIDER_APPLICATION_HOST = re.compile(
    r"\b(?:[a-z0-9-]+\.)+"
    r"(?:railway\.app|fly\.dev|onrender\.com|herokuapp\.com|"
    r"vercel\.app|netlify\.app)\b",
    re.IGNORECASE,
)
RETAINED_CONTRACTS = {
    "contracts/operations.md": (
        "Public node JSON-RPC uses HTTPS at the configured node origin's root path.",
        '"method":"eth_chainId"',
        "-H 'content-type: application/json'",
    ),
    "deploy/launchpad/README.md": (
        "The Go service deployment definitions use port `8080`.",
        "`/health/ready`",
        "`/api/health`",
    ),
    "docs/site/paxeer-docs/src/app/webhooks/page.tsx": (
        "POST /v1/webhooks/endpoints",
        "GET /v1/webhooks/scheme",
        "Content-Type: application/json",
    ),
    "docs/wiki/HostedDashboard.md": (
        "`LAYERX_DASHBOARD_LISTEN=[::]:9445`",
        "Redis TLS stores",
        "`GET /healthz`",
        "All listed routes are `GET`",
        "`/v1/dashboard/overview`",
    ),
    "docs/wiki/HostedWebhooks.md": (
        "Redis requires `rediss://`",
        "ingress TLS to `[::]:9443`",
        "`POST /internal/v1/events/{kind}/{source_event}`",
        "`GET /internal/v1/events/{id}`",
        "| GET | `/healthz`",
    ),
    "services/explorer/deploy/railway/README.md": (
        "Their HTTP port defaults to 8050.",
        "`GET /api/health/liveness`",
        "`GET /explorer/api/healthz`",
    ),
}


CONFIGURATION_CONTRACTS = {
    "contracts/operations.md": (
        "Hosted-service dependency examples use `OPERATOR_HOST`",
        "Replace `OPERATOR_HOST` with the operator-supplied hostname",
    ),
    "deploy/launchpad/README.md": (
        "Export the four upstream variables before the web build command.",
        "the image registry is supplied\nthrough `REGISTRY`.",
    ),
    "docs/site/paxeer-docs/src/app/webhooks/page.tsx": (
        '"url": "<public-https-receiver-url>"',
        "The URL uses a canonical DNS name and resolves exclusively to public addresses.",
    ),
    "docs/wiki/HostedDashboard.md": (
        "rewrites `/v1/dashboard/*` to `DASHBOARD_API_ORIGIN`",
        "which is required in production.",
    ),
    "docs/wiki/HostedWebhooks.md": (
        "Configure the Redis TLS origin, KMS, identity and all four event-source HTTPS",
        "origins through the keys below.",
    ),
    "services/explorer/deploy/railway/README.md": (
        "Every value is deployment-specific and belongs in Railway's",
        "placeholder form is `<...>`.",
    ),
}


@pytest.mark.parametrize("page", PAGES)
def test_hosted_page_has_no_private_infrastructure_hostname(page):
    text = (ROOT / page).read_text(encoding="utf-8")
    assert not PRIVATE_DNS.search(text), f"Private service DNS in {page}"
    assert not PROVIDER_APPLICATION_HOST.search(text), (
        f"Hosting-provider application hostname in {page}"
    )


@pytest.mark.parametrize("page", PAGES)
def test_hosted_page_explains_operator_configuration(page):
    text = (ROOT / page).read_text(encoding="utf-8")
    for contract in CONFIGURATION_CONTRACTS[page]:
        assert contract in text, f"Missing configuration contract {contract!r} in {page}"
    if page == "contracts/operations.md":
        placeholders = set(re.findall(r"\bOPERATOR_[A-Z0-9_]*HOST\b", text))
        assert placeholders == {"OPERATOR_HOST"}, page
        explanations = re.findall(
            r"^Replace `OPERATOR_HOST` with the operator-supplied hostname [^\n]+\.$",
            text,
            re.MULTILINE,
        )
        assert len(explanations) == 1, page
        uses = text.replace(explanations[0], "")
        assert "OPERATOR_HOST" in uses, page
        assert uses.count("`OPERATOR_HOST`") <= 1, page


@pytest.mark.parametrize("page", PAGES)
def test_operator_examples_preserve_endpoint_contracts(page):
    text = (ROOT / page).read_text(encoding="utf-8")
    for contract in RETAINED_CONTRACTS[page]:
        assert contract in text, f"Missing endpoint contract {contract!r} in {page}"
