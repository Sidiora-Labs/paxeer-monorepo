"""Focused, offline qualification of audit 22.30.169's three test gaps.

The individual legacy assertions are exercised here without their live CLI
tests. HTTP exchanges use the production client, codec and report consumers.
No subprocess, credential, environment variable or external endpoint is needed.
"""

from __future__ import annotations

import json

import pytest

from tools.jev.checks import deps, docs, pr
from tools.jev.report import Report
from tools.jev.tests import test_deps as dependency_tests
from tools.jev.tests import test_docs as documentation_tests
from tools.jev.tests import test_pr as pull_request_tests
from tools.jev.tests.recorded_server import recorded_client


def cases(*classes, covered_by_siblings=()):
    """Select only the named pure/in-process suites, never live or CLI cases."""
    return [
        pytest.param(cls, name, id=f"{cls.__name__}.{name}")
        for cls in classes
        for name in sorted(vars(cls))
        if name.startswith("test_")
        and name not in covered_by_siblings
    ]


@pytest.mark.parametrize("case_class,method", cases(
    dependency_tests.ParseDiagnosticsTests,
    dependency_tests.DependencyScanTests,
    dependency_tests.GroupingTests,
    dependency_tests.QuestionTests,
    dependency_tests.FindingTests,
    covered_by_siblings=("test_default_scanner_selects_all_five_live_graphs",),
))
def test_jev_deps_test_roots(case_class, method):
    case_class(method).debug()


@pytest.mark.parametrize("case_class,method", cases(
    documentation_tests.SectionSplitterTests,
    documentation_tests.PairingTests,
    documentation_tests.QuestionTests,
    documentation_tests.PublishedCatalogTests,
    documentation_tests.PairFindingTests,
    covered_by_siblings=(
        "test_headings_inside_code_fences_are_not_sections",
        "test_check_registers_itself_with_readme_defaults",
    ),
))
def test_jev_doc_test_headings(case_class, method):
    case_class(method).debug()


def test_jev_doc_test_headings_preserves_fenced_heading_boundaries():
    sections = docs.split_sections(documentation_tests.SOURCE, documentation_tests.FENCED)
    assert [section.title for section in sections] == [
        docs.PREAMBLE_TITLE, "Real heading", "Second heading",
    ]
    assert [section.line for section in sections] == [1, 3, 12]
    assert [section.level for section in sections] == [0, 2, 2]
    assert "# not a heading" in sections[1].body
    assert "make build" in sections[1].body
    assert "Trailing prose." in sections[1].body


@pytest.mark.parametrize("method", [
    name for name in sorted(vars(pull_request_tests.BodyFindingTests))
    if name.startswith("test_")
    and name != "test_missing_or_unrelated_answers_fail_at_the_request_boundary"
])
def test_jev_missing_answer_test_preserves_complete_review_and_certification(method):
    pull_request_tests.BodyFindingTests(method).debug()


@pytest.mark.parametrize("advisory_id", [
    dependency_tests.PASTE_ADVISORY, dependency_tests.RUSTLS_ADVISORY,
])
def test_dependency_capture_reaches_client_and_report_with_real_lockfile_anchors(advisory_id):
    advisory = dependency_tests.grouped()[advisory_id]
    state = deps.advisory_state(advisory)
    questions = deps.advisory_questions()
    with recorded_client("deps-triage.json") as exchange:
        evaluation = exchange.client.evaluate(state, questions)
        exchange.assert_evaluation(evaluation, state, questions)
    report = Report(check="deps", revision="recorded-lockfile-evidence",
                    model=evaluation.model, findings=deps.findings_for(advisory, evaluation),
                    calls=1, input_tokens=int(evaluation.usage["input_tokens"]),
                    cost_usd=evaluation.usage["cost"])
    document = json.loads(report.to_json())
    assert document["calls"] == 1
    assert document["input_tokens"] == 512
    assert document["cost_usd"] == evaluation.usage["cost"]
    assert len(document["findings"]) == 2
    assert {finding["question"] for finding in document["findings"]} == set(questions)
    for finding in document["findings"]:
        assert finding["detail"]["workspaces"] == ["services/agent", "services/interop"]
        assert finding["anchor"] == advisory.anchor
        assert finding["detail"]["anchors"] == list(advisory.anchors)
        for anchor in finding["detail"]["anchors"]:
            path, line = anchor.rsplit(":", 1)
            text = (dependency_tests.ROOT / path).read_text(encoding="utf-8").splitlines()
            assert text[int(line) - 1] == f'name = "{advisory.crate}"'
    assert advisory_id in report.to_markdown()


def test_identical_documentation_has_a_complete_clean_response():
    source = docs.split_sections(documentation_tests.SOURCE)
    section = next(section for section in source if section.title == "Build")
    translated = docs.split_sections(documentation_tests.GERMAN,
                                     f"## Build\n\n{section.body}")[0]
    pair = docs.pair_sections([section], [translated])[0][0]
    state = {"section": section.title, "source": docs.sanitize(section.body),
             "translation": docs.sanitize(translated.body)}
    with recorded_client("docs-faithful.json") as exchange:
        evaluation = exchange.client.evaluate(state, docs.QUESTIONS)
        exchange.assert_evaluation(evaluation, state, docs.QUESTIONS)
    assert state["source"] == state["translation"]
    assert docs.pair_findings(pair, evaluation) == []


@pytest.mark.parametrize("fixture,route,claim,honesty", [
    ("commit-valid.json", "auto", 0.04, "consistent"),
    ("commit-overclaims.json", "escalate", 0.87, "overclaims"),
])
def test_complete_commit_review_preserves_honesty_and_recorded_commit_anchors(
    fixture, route, claim, honesty,
):
    commit = pull_request_tests.recorded_commit()
    state = pr.commit_state(commit)
    questions = pr.commit_questions()
    assert state["commit"] == commit.sha
    assert state["message"] == commit.message
    assert state["changed_file_count"] == len(commit.changes)
    assert state["changed_file_count"] > 0
    with recorded_client(fixture) as exchange:
        evaluation = exchange.client.evaluate(state, questions)
        exchange.assert_evaluation(evaluation, state, questions)
    report = Report(check="pr", revision=commit.sha, model=evaluation.model,
                    findings=pr.commit_findings(evaluation, commit))
    pr._account(report, evaluation)
    assert len(report.findings) == 2
    assert [finding.answer for finding in report.findings] == [claim, honesty]
    for finding in report.findings:
        assert finding.anchor == commit.sha
        assert pull_request_tests.SHA.fullmatch(finding.anchor)
        assert finding.subject == commit.sha[:12]
        assert finding.detail["subject"] == commit.subject
        assert finding.route == route
    document = json.loads(report.to_json())
    assert document["revision"] == commit.sha
    assert {finding["anchor"] for finding in document["findings"]} == {commit.sha}
    assert ("_no review or escalate findings_" in pr.render_comment(report)) == (route == "auto")
