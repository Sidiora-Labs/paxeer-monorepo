"""Keep both hosted gateway pages aligned with the readiness document scopes."""

from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DOCUMENTS = (
    "docs/site/docs/platform/hosted-gateway.md",
    "docs/wiki/HostedGateway.md",
)


def test_readiness_doc_describes_chain_only_document() -> None:
    for path in DOCUMENTS:
        document = (ROOT / path).read_text(encoding="utf-8")
        readiness = document.split("## Readiness\n", 1)[1].split("\n## ", 1)[0]
        readiness = " ".join(readiness.split())
        chain_only, kernel = readiness.split("When any `KernelBackend`", 1)

        for claim in (
            "no kernel backend configured",
            "chain-only readiness document",
            "`backends` lists only configured backends plus `paxeer_chain`",
            "`status` is `ready` when every listed backend and `chain_serving` are ready",
            "`components` contains only `durable_store`",
            "omits the five kernel components and `principal_state_boundary`",
        ):
            assert claim in chain_only, f"{path}: missing chain-only claim: {claim}"

        for claim in (
            "is configured, `backends` gains all five kernel entries",
            "`core_agent_boundary`",
            "`public_core`",
            "`independent_receipt_authority`",
            "`identity`",
            "`program_registry`",
            "`components` gains those same five entries and `principal_state_boundary`",
            "with the string `unavailable`",
            "that field is present only when a kernel backend is configured",
            "agent-boundary `/readyz` must be 200 JSON with `ready`",
            "matching `network_id` and `wire_version`",
            "`synchronous_receipts` plus `state_snapshot`",
            "authority `/readyz` must be 200 JSON with `ready`",
            "matching network/wire (route flags not required)",
            "registry `GET /healthz` must be 200 JSON `status=ready`",
            "`service=program-registry`",
            "`GET /readyz`",
            "answers HTTP 200 when every configured backend is ready",
            "`GET /readyz/core`",
            "answers HTTP 200 only when its readiness document is complete",
        ):
            assert claim in kernel, f"{path}: missing kernel or route claim: {claim}"

        for lines in (
            "3575-3592",
            "3718-3719",
            "3732-3740",
            "138-144",
            "3593-3609",
            "3735-3740",
            "3292-3293",
            "3295-3296",
            "3742-3747",
        ):
            citation = f"services/platform/hosted/gateway/src/main.rs:{lines}"
            assert citation in readiness, f"{path}: missing current citation: {citation}"

        normalized = " ".join(document.split())
        assert "principal_state_boundary` is always the string" not in normalized, path
        assert "Identity is not a `/readyz` component" not in normalized, path
        for lines in ("1660-1711", "2799-2831", "2828"):
            assert f"main.rs:{lines}`" not in readiness, path
