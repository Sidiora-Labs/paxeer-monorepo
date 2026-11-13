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
        "https://api1.mainnet-beta.paxeer.network",
        '"method":"eth_chainId"',
        "JSON-RPC 2.0 POST uses the root endpoint",
    ),
    "deploy/launchpad/README.md": (
        "http://OPERATOR_HOST:8080",
        "`/health/ready`",
        "`/api/health`",
    ),
    "docs/site/paxeer-docs/src/app/webhooks/page.tsx": (
        "rediss://OPERATOR_HOST:6379",
        "https://OPERATOR_HOST:9443",
        "`POST /internal/v1/events/{kind}/{source_event}`",
        "`GET /internal/v1/events/{id}`",
        "`GET /healthz`",
    ),
    "docs/wiki/HostedDashboard.md": (
        "http://OPERATOR_HOST:9445",
        "rediss://OPERATOR_HOST:6379",
        "https://OPERATOR_HOST:9443",
        "`GET /healthz`",
        "All listed routes are `GET`",
        "`/v1/dashboard/overview`",
    ),
    "docs/wiki/HostedWebhooks.md": (
        "rediss://OPERATOR_HOST:6379",
        "https://OPERATOR_HOST:9443",
        "`POST /internal/v1/events/{kind}/{source_event}`",
        "`GET /internal/v1/events/{id}`",
        "| GET | `/healthz`",
    ),
    "services/explorer/deploy/railway/README.md": (
        "http://OPERATOR_HOST:8050",
        "`GET /api/health/liveness`",
        "`GET /explorer/api/healthz`",
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
def test_operator_placeholder_is_used_and_explained_once_per_page(page):
    text = (ROOT / page).read_text(encoding="utf-8")
    placeholders = set(re.findall(r"\bOPERATOR_[A-Z0-9_]*HOST\b", text))
    assert placeholders == {"OPERATOR_HOST"}, page
    explanations = re.findall(
        r"^Replace `OPERATOR_HOST` with the operator-supplied hostname [^\n]+\.$",
        text,
        re.MULTILINE,
    )
    assert len(explanations) == 1, f"Expected one substitution note in {page}"
    uses = text.replace(explanations[0], "")
    assert "OPERATOR_HOST" in uses, f"Placeholder must be used in {page}"
    assert uses.count("`OPERATOR_HOST`") <= 1, (
        f"Duplicate standalone placeholder explanation in {page}"
    )


@pytest.mark.parametrize("page", PAGES)
def test_operator_examples_preserve_endpoint_contracts(page):
    text = (ROOT / page).read_text(encoding="utf-8")
    for contract in RETAINED_CONTRACTS[page]:
        assert contract in text, f"Missing endpoint contract {contract!r} in {page}"
