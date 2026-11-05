from pathlib import Path

from programme.parity import count_go_tests

ROOT = Path(__file__).resolve().parents[4]


def test_unmatched_pattern_counts_zero() -> None:
    command = "go test -count=1 ./modules/epoch/types/ -run '^(TestDoesNotExist)$'"
    assert count_go_tests(command, ROOT) == 0


def test_matched_pattern_counts_one() -> None:
    command = "go test -count=1 ./precompiles/common/ -run '^(TestRetiredAtMergePlan)$'"
    assert count_go_tests(command, ROOT) == 1
