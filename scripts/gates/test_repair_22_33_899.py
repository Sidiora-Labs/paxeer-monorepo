"""Run all gate regressions through the real CLI and isolated committed sources."""

from scripts.gates import test_run_gate


class TestRepair2233899(test_run_gate.RunGateTest):
    """The repair gate includes the existing criteria and every retention regression."""
