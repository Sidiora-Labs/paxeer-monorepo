import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs/wallet/service"
TASK_OR_OBSERVATION = re.compile(
    r"\b(?:tasks?|observations?)\s+#?\d+(?:\.\d+)*\b", re.IGNORECASE
)


def wallet_pages():
    pages = sorted(DOCS.rglob("*.md"))
    assert pages, f"No wallet service pages found under {DOCS}"
    return [(path.relative_to(ROOT), path.read_text(encoding="utf-8")) for path in pages]


def test_wallet_docs_name_no_missing_spec_ledger():
    references = [
        str(path)
        for path, text in wallet_pages()
        if "spec/paxeer-x-wallet/qualification.kvx" in text
    ]
    assert not references, "Missing qualification ledger named in: " + ", ".join(references)


def test_wallet_docs_carry_no_task_numbers():
    references = [
        f"{path}: {match.group()}"
        for path, text in wallet_pages()
        for match in TASK_OR_OBSERVATION.finditer(text)
    ]
    assert not references, "Task or observation numbers remain:\n" + "\n".join(references)


def test_wallet_gateway_rollback_names_tracked_manifest():
    pages = dict(wallet_pages())
    text = pages[Path("docs/wallet/service/runbooks/gateway-rollback.md")]
    assert "gateway.toml" not in text, "Rollback names the missing gateway manifest"
    deployment_paths = (
        "docker/wallet-gateway/Dockerfile",
        "services/wallet/deploy/railway.env.example",
        "services/wallet/deploy/redis.railway.env.example",
    )
    for path in deployment_paths:
        assert f"`{path}`" in text, f"Rollback must name {path}"
        assert (ROOT / path).is_file(), f"Deployment reference does not exist: {path}"
    tracked = subprocess.run(
        ["git", "ls-files", "--error-unmatch", "--", *deployment_paths],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    assert set(tracked.stdout.splitlines()) == set(deployment_paths)
    assert "scripts/wallet/check-live.sh gateway" in text
