"""Keep endpoint examples independent of private service DNS."""

import importlib.util
import json
from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[2]
BETA_AND_FAUCET_ENDPOINTS = {
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
INTEROP_GATEWAY_AUTHORITY = "interop-gateway.example.internal:9443"
INTEROP_GATEWAY_ORIGIN = f"https://{INTEROP_GATEWAY_AUTHORITY}"
INTEROP_GATEWAY_ENDPOINTS = {
    "services/interop/deploy/gateway/config.example.json": (
        INTEROP_GATEWAY_ORIGIN,
        INTEROP_GATEWAY_AUTHORITY,
    ),
    "services/interop/deploy/gateway/trust-inputs.example.json": (
        INTEROP_GATEWAY_ORIGIN,
        INTEROP_GATEWAY_AUTHORITY,
    ),
}
DOC_ENDPOINTS = {**BETA_AND_FAUCET_ENDPOINTS, **INTEROP_GATEWAY_ENDPOINTS}
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


@pytest.mark.parametrize("page", BETA_AND_FAUCET_ENDPOINTS)
def test_beta_and_faucet_docs_keep_private_network_only_wording(page):
    content = (ROOT / page).read_text(encoding="utf-8")
    assert re.search(r"private-network only|private network only", content, re.I), (
        f"{page} must retain the private-network only wording checked by gate 108.0.7"
    )


@pytest.mark.parametrize("page", INTEROP_GATEWAY_ENDPOINTS)
def test_interop_gateway_examples_name_no_in_network_host(page):
    content = (ROOT / page).read_text(encoding="utf-8")
    assert PRIVATE_SERVICE_HOST.search(content) is None, page
    example = json.loads(content)
    for section, field, expected in (
        ("ap2_assets", "audience", INTEROP_GATEWAY_ORIGIN),
        ("visa_agents", "agent_domain", INTEROP_GATEWAY_ORIGIN),
        ("visa_targets", "authority", INTEROP_GATEWAY_AUTHORITY),
    ):
        assert example[section], (page, section)
        for entry in example[section]:
            assert entry[field] == expected, (page, section, field)


def test_render_self_check_fixture_matches_examples():
    spec = importlib.util.spec_from_file_location(
        "interop_gateway_render",
        ROOT / "services/interop/deploy/gateway/render.py",
    )
    renderer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(renderer)
    material, roots = renderer.self_test_root_fixture()
    assert material["audience"] == INTEROP_GATEWAY_ORIGIN
    for page in INTEROP_GATEWAY_ENDPOINTS:
        example = json.loads((ROOT / page).read_text(encoding="utf-8"))
        for section, field in (
            ("ap2_assets", "audience"),
            ("visa_agents", "agent_domain"),
            ("visa_targets", "authority"),
        ):
            assert [entry[field] for entry in roots[section]] == [
                entry[field] for entry in example[section]
            ], (page, section, field)
