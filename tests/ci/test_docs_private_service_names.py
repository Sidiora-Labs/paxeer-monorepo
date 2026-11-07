"""Keep beta and faucet endpoint examples independent of private service DNS."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[2]
DOC_ENDPOINTS = {
    "docs/platform/beta-environment.md": (
        "https://OPERATOR-PARAMETERS-ORIGIN/v1/parameters",
        "`/v1/status`",
        "https://OPERATOR-GATEWAY-ORIGIN",
        "https://OPERATOR-FAUCET-ORIGIN",
    ),
    "docs/site/docs/platform/faucet.md": (
        "`LAYERX_IDENTITY_INTROSPECTION_URL` | "
        "`https://OPERATOR-IDENTITY-ORIGIN:9443/v1/introspect`",
        "`LAYERX_TESTNET_FUNDING_URL` | "
        "`https://OPERATOR-CONTROL-ADMIN-ORIGIN/admin/v1/testnet/fund`",
        "`LAYERX_FAUCET_REDIS_URL` | `rediss://OPERATOR-FAUCET-REDIS:6379`",
    ),
    "docs/wiki/HostedBetaControl.md": (
        "https://OPERATOR-CORE-ADMIN-ORIGIN:9444",
        "`LAYERX_TESTNET_CORE_ADMIN_URL`",
    ),
    "docs/wiki/HostedFaucet.md": (
        "https://OPERATOR-CONTROL-ADMIN-ORIGIN/admin/v1/testnet/fund",
        "`LAYERX_TESTNET_FUNDING_URL`",
        "`LAYERX_FAUCET_REDIS_URL` | Operator-supplied, for example "
        "`rediss://OPERATOR-FAUCET-REDIS:6379`",
    ),
}
PRIVATE_SERVICE_HOST = re.compile(
    r"\.svc\b|\.cluster\.local\b|\.layerx-testnet\b|\blayerx-testnet\.",
    re.IGNORECASE,
)


@pytest.mark.parametrize("page", DOC_ENDPOINTS)
def test_beta_and_faucet_docs_name_no_private_service_hosts(page):
    content = (ROOT / page).read_text(encoding="utf-8")
    assert PRIVATE_SERVICE_HOST.search(content) is None, (
        f"{page} must use operator placeholders instead of private service DNS"
    )
    for endpoint in DOC_ENDPOINTS[page]:
        assert endpoint in content, f"{page} must retain endpoint example: {endpoint}"


@pytest.mark.parametrize("page", DOC_ENDPOINTS)
def test_beta_and_faucet_docs_keep_private_network_only_wording(page):
    content = (ROOT / page).read_text(encoding="utf-8")
    assert re.search(r"private-network only|private network only", content, re.I), (
        f"{page} must retain the private-network only wording checked by gate 108.0.7"
    )
