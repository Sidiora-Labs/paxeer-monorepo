from pathlib import Path


def test_readmes_name_no_retired_fetcher():
    backend = Path(__file__).resolve().parents[1] / "backend"
    for readme in (backend / "README.md", backend / "apps/indexer/README.md"):
        content = readme.read_text(encoding="utf-8")
        for retired_name in (
            "PaxeerXKernelReceipts",
            "paxeer_x_kernel_receipts",
            "INDEXER_PAXEER_X_KERNEL_RECEIPTS_",
        ):
            assert retired_name not in content, f"{readme} names {retired_name}"
