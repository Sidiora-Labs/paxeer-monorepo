import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[5]
SERVICE_CONFIG = "services/interop/crates/layerx-interop-service/src/config.rs"
SERVICE_SERVER = "services/interop/crates/layerx-interop-service/src/server.rs"
WEBSEARCH_CONFIG = "services/interop/crates/x-websearch/src/config.rs"
WEBSEARCH_PAYMENT = "services/interop/crates/x-websearch/src/payment.rs"

RETIRED_SURFACES = (
    (
        "interface.11.fmt-deploy-intent-ingester-hosted",
        "services/interop/deploy/intent-ingester/railway.env.example",
        None,
    ),
    (
        "interface.11.http-svc-out-hosted-activities",
        SERVICE_SERVER,
        r'"[^"\n]*/v1/activities(?:[/?"{]|$)',
    ),
    (
        "interface.11.http-svc-out-receipt-authority-authorized-batches",
        SERVICE_SERVER,
        r'"[^"\n]*/v1/authorized-batches(?:[/?"{]|$)',
    ),
    (
        "interface.11.env-svc-layerx-interop-receipt-authority-url",
        SERVICE_CONFIG,
        r'"LAYERX_INTEROP_RECEIPT_AUTHORITY_URL"',
    ),
    (
        "interface.11.env-svc-layerx-interop-receipt-authority-token-file",
        SERVICE_CONFIG,
        r'"LAYERX_INTEROP_RECEIPT_AUTHORITY_TOKEN_FILE"',
    ),
    (
        "interface.11.cfg-xws-payment-payer-did",
        WEBSEARCH_CONFIG,
        r'\bpayer_did\b|"payment\.payer_did"',
    ),
    (
        "interface.11.rs-xws-payer-did",
        WEBSEARCH_PAYMENT,
        r"\bconst\s+PAYER_DID\b",
    ),
    (
        "interface.11.rs-xws-wallet-account",
        WEBSEARCH_PAYMENT,
        r"\bfn\s+wallet_account\s*\(",
    ),
    (
        "interface.11.rs-xws-account-id",
        WEBSEARCH_PAYMENT,
        r"\bfn\s+account_id\s*\(",
    ),
)

PLANNED_PATHS = (
    "services/interop/crates/layerx-committed-proof/src/lib.rs",
    "services/interop/crates/layerx-committed-proof/tests/committed.rs",
    "services/interop/crates/layerx-x402/tests/facilitator_evidence.rs",
    "services/interop/crates/layerx-x402/tests/buyer_sequencer.rs",
    "services/interop/deploy/tests/test_deploy_templates.py",
    "services/interop/deploy/gateway/tests/test_render_receipt_keys.py",
    "services/interop/specs/conformance/committed/vector-v4-committed.json",
    "services/interop/crates/layerx-committed-proof/tests/committed_vectors.rs",
    "services/interop/specs/conformance/x402/sequenced-settlement.json",
    "services/interop/crates/layerx-x402/tests/sequenced_vectors.rs",
    "services/interop/crates/layerx-interop-gateway/tests/workspace_lock.rs",
    "services/interop/crates/layerx-bridge-relayer/tests/sequencer_receipt.rs",
    "services/interop/crates/layerx-gas-station/tests/sequencer_receipt.rs",
    "services/interop/crates/layerx-mirror/tests/committed_freshness.rs",
    "services/interop/crates/x-websearch/tests/sequencer_vectors.rs",
    "services/interop/crates/x-websearch/tests/example_config.rs",
    "services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs",
    "services/interop/crates/layerx-bridge-relayer/src/attestation_api.rs",
    "services/interop/crates/layerx-bridge-relayer/tests/attestation_server.rs",
)


def test_interop_completion_retired_surfaces_absent():
    failures = []
    sources = {}
    for interface, relative_path, pattern in RETIRED_SURFACES:
        path = ROOT / relative_path
        if pattern is None:
            if path.exists() or path.is_symlink():
                failures.append(f"Retired surface {interface} remains at {relative_path}")
            continue
        if not path.exists():
            continue
        if relative_path not in sources:
            sources[relative_path] = re.sub(
                r"^#\[cfg\(test\)\]\s*\nmod \w+ \{\n.*?^\}",
                "",
                path.read_text(encoding="utf-8"),
                flags=re.MULTILINE | re.DOTALL,
            )
        if re.search(pattern, sources[relative_path]):
            failures.append(f"Retired surface {interface} remains at {relative_path}")

    for relative_path in PLANNED_PATHS:
        if not (ROOT / relative_path).is_file():
            failures.append(f"Missing planned path: {relative_path}")

    assert not failures, "Domain 11 completion blocked:\n" + "\n".join(failures)
