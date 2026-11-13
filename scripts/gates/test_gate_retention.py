"""Run all gate regressions through the real CLI and isolated committed sources."""

from scripts.gates import test_run_gate


class TestGateRetention(test_run_gate.RunGateTest):
    """Gate retention includes the existing criteria and every retention regression."""
