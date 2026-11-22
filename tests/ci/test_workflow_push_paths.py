from pathlib import Path

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
TARGET_WORKFLOWS = ("node-build.yml", "codeql.yml", "wallet-test.yml")


def workflow_triggers(name):
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows" / name).read_text(encoding="utf-8")
    )
    return workflow[True]


@pytest.mark.parametrize("name", TARGET_WORKFLOWS)
def test_push_workflows_ignore_only_specification_records(name):
    push = workflow_triggers(name)["push"]
    assert push["branches"] == ["main"]
    assert push["paths-ignore"] == ["spec/**", "GOTCHA.kvx"]
    assert "paths" not in push


@pytest.mark.parametrize("name", TARGET_WORKFLOWS)
def test_pull_requests_have_no_path_filters(name):
    pull_request = workflow_triggers(name)["pull_request"]
    assert pull_request is None or isinstance(pull_request, dict)
    if pull_request is not None:
        assert "paths" not in pull_request
        assert "paths-ignore" not in pull_request


@pytest.mark.parametrize("name", TARGET_WORKFLOWS)
def test_ignored_push_paths_exist(name):
    for pattern in workflow_triggers(name)["push"]["paths-ignore"]:
        assert any(ROOT.glob(pattern)), (name, pattern)
