from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / (
    "services/explorer/services/smart-contract-verifier/"
    "smart-contract-verifier/src/zksync/zksolc_standard_json/output/mod.rs"
)


def test_zksolc_output_has_no_long_commented_block():
    consecutive_comments = 0
    for number, line in enumerate(OUTPUT.read_text(encoding="utf-8").splitlines(), 1):
        if line.lstrip().startswith("//"):
            consecutive_comments += 1
        else:
            consecutive_comments = 0
        assert consecutive_comments <= 20, (
            f"{OUTPUT.relative_to(ROOT)}:{number}: "
            f"{consecutive_comments} consecutive comment-only lines (maximum 20)"
        )
