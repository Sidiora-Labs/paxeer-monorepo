from __future__ import annotations

import hashlib
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

from tools.jev import cli
from tools.jev.checks import deps, docs
from tools.jev.client import JevClient, JevConfigError
from tools.jev.report import Report


FIXTURES = Path(__file__).parent / "fixtures" / "repair_22_33_704"
ROOT = Path(__file__).resolve().parents[3]
RECORDED_REVISION = "264615cba89456c37eeb19535cb16b81846f6a68"
WORKSPACES = (
    "services/agent", "services/human", "services/platform", "programs", "services/interop"
)
DEPS_RESPONSE = {
    "model": "typesafe/jev-1.13-20260917",
    "id": "gen-dec-1789715517-deps",
    "answers": {
        "priority": {
            "type": "choice", "choice": "production_reachable", "confidence": 0.91,
            "probabilities": {
                "production_reachable": 0.91, "dev_only": 0.05,
                "already_mitigated": 0.02, "needs_human": 0.02,
            },
        },
        "urgency": {
            "type": "score", "score": 1.62, "confidence": 0.58,
            "legend": {"0": "low", "1": "medium", "2": "high"},
            "probabilities": {"0": 0.02, "1": 0.34, "2": 0.64},
        },
    },
    "usage": {"input_tokens": 512, "output_tokens": 64, "cost": 0.0000215},
}
DRIFT_RESPONSE = {
    "model": "typesafe/jev-1.13-20260917", "id": "gen-dec-docs-stale-reference",
    "answers": {
        "same_claims": {"type": "noul", "noul": 0.04},
        "drift_kind": {
            "type": "choice", "choice": "stale_reference", "confidence": 0.96,
            "probabilities": {
                "faithful": 0.01, "missing_content": 0.01, "extra_content": 0.01,
                "stale_reference": 0.96, "mistranslated": 0.01,
            },
        },
    },
    "usage": {"input_tokens": 128, "output_tokens": 24, "cost": 0.00001},
}
FAITHFUL_RESPONSE = {
    "model": "typesafe/jev-1.13-20260917", "id": "gen-dec-docs-faithful",
    "answers": {
        "same_claims": {"type": "noul", "noul": 0.99},
        "drift_kind": {
            "type": "choice", "choice": "faithful", "confidence": 0.96,
            "probabilities": {
                "faithful": 0.96, "missing_content": 0.01, "extra_content": 0.01,
                "stale_reference": 0.01, "mistranslated": 0.01,
            },
        },
    },
    "usage": {"input_tokens": 128, "output_tokens": 24, "cost": 0.00001},
}
EMPTY_SCAN = json.dumps({
    "type": "summary",
    "fields": {
        check: {"errors": 0, "warnings": 0, "notes": 0, "helps": 0}
        for check in ("advisories", "bans", "sources")
    },
}) + "\n"


@pytest.fixture
def jev_server(tmp_path):
    exchanges = []
    received = []
    failures = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            raw = self.rfile.read(int(self.headers["Content-Length"]))
            try:
                assert self.path == "/api/alpha/decisions"
                assert self.headers["Authorization"] == "Bearer fixture-key"
                assert self.headers["Content-Type"] == "application/json"
                document = json.loads(raw)
                assert raw == json.dumps(
                    document, sort_keys=True, separators=(",", ":")
                ).encode("utf-8")
                state, questions, response = exchanges.pop(0)
                assert document == {"model": "fixture/jev", "state": state, "questions": questions}
                response_raw = json.dumps(response).encode("utf-8")
                received.append((raw, response_raw))
                status = 200
            except (AssertionError, IndexError, ValueError) as error:
                failures.append(str(error))
                response_raw = b'{"error":"recorded exchange did not match"}'
                status = 400
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response_raw)))
            self.end_headers()
            self.wfile.write(response_raw)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    log_path = tmp_path / "requests.jsonl"
    client = JevClient(
        api_key="fixture-key", model="fixture/jev", max_attempts=1,
        endpoint=f"http://127.0.0.1:{server.server_port}/api/alpha/decisions",
        log_path=log_path,
    )
    try:
        yield client, exchanges, received
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert failures == []
        assert exchanges == []
        logs = [json.loads(line) for line in log_path.read_text().splitlines()] if received else []
        assert len(logs) == len(received)
        for log, (request, response) in zip(logs, received):
            assert log["request_sha256"] == hashlib.sha256(request).hexdigest()
            assert log["response_sha256"] == hashlib.sha256(response).hexdigest()
            assert log["requested_model"] == "fixture/jev"
            assert log["model"] == "typesafe/jev-1.13-20260917"


def arguments(check, argv=()):
    parser = cli.build_parser(check.CHECK_NAME)
    check.add_arguments(parser)
    return parser.parse_args(["--out", "unused-report-directory", *argv])


def write_capture(tmp_path, returncode, text):
    path = tmp_path / "scan-result.json"
    path.write_text(json.dumps({"returncode": returncode, "stderr": text}), encoding="utf-8")
    return path


def test_jev_moved_workspaces(jev_server, tmp_path):
    client, exchanges, _ = jev_server
    assert deps.WORKSPACES == WORKSPACES
    assert deps.selected_workspaces(None) == WORKSPACES
    selected = deps.selected_workspace_directories(ROOT, None)
    assert selected == [(name, ROOT / name) for name in WORKSPACES]
    for name in WORKSPACES:
        args = arguments(deps, ["--workspace", name])
        assert deps.selected_workspace_directories(ROOT, args.workspace) == [(name, ROOT / name)]
        scanned = deps.scan_workspace(ROOT / name)
        assert scanned.normal
        assert scanned.normal.isdisjoint(scanned.dev)

    index = deps.DependencyIndex(ROOT)
    assert index.classify("services/agent", "rustls") == deps.PRODUCTION
    assert index.classify("agent", "rustls") == deps.PRODUCTION
    assert index.classify("services/interop", "rustls") == deps.PRODUCTION
    assert index.classify("services/agent", "native-tls") == deps.DEV_ONLY

    # The committed cargo-deny transcripts are fed through the production input path.
    args = arguments(deps, [
        "--input", f"{FIXTURES / 'agent-advisories.jsonl'}=agent",
        "--input", f"{FIXTURES / 'interop-advisories.jsonl'}=services/interop",
    ])
    sources = deps.collect_sources(args, ROOT)
    assert [workspace for workspace, _ in sources] == ["services/agent", "services/interop"]
    diagnostics = [item for workspace, text in sources for item in deps.parse_diagnostics(text, workspace)]
    groups = deps.group_advisories(diagnostics, index)
    assert len(groups) == 2
    for advisory in groups:
        assert advisory.workspaces == ("services/agent", "services/interop")
        for anchor in advisory.anchors:
            path, line = anchor.rsplit(":", 1)
            assert (ROOT / path).is_file()
            assert 0 < int(line) <= len((ROOT / path).read_text().splitlines())
        exchanges.append((deps.advisory_state(advisory), deps.advisory_questions(), DEPS_RESPONSE))
    report = deps.CHECK.evaluate_sources(
        sources, ROOT, client, Report("deps", RECORDED_REVISION, client.model)
    )
    assert report.calls == 2
    assert len(report.findings) == 4
    assert {item.anchor for item in report.findings} == {
        "services/agent/Cargo.lock:101", "services/agent/Cargo.lock:119"
    }
    assert all(item.detail["workspaces"] == ["services/agent", "services/interop"] for item in report.findings)
    rustls = [item for item in report.findings if item.detail["crate"] == "rustls"]
    assert all(item.detail["dependency_kind"] == "production" for item in rustls)
    json_path, md_path = report.write(tmp_path / "report")
    assert len(json.loads(json_path.read_text())["findings"]) == 4
    assert "services/agent/Cargo.lock:119" in md_path.read_text()


def test_jev_selected_missing_manifest_is_nonzero(tmp_path, capsys):
    for name in WORKSPACES:
        directory = tmp_path / name
        directory.mkdir(parents=True)
        (directory / "Cargo.toml").write_text('[workspace]\nmembers = []\n')
    (tmp_path / "services/human/Cargo.toml").unlink()
    with pytest.raises(JevConfigError, match="services/human/Cargo.toml.*missing"):
        deps.selected_workspace_directories(tmp_path, None)
    out_dir = tmp_path / "report"
    assert cli.main([
        "deps", "--run", "--workspace", "services/human",
        "--repo-root", str(tmp_path), "--out", str(out_dir),
    ]) == 2
    assert "selected workspace manifest" in capsys.readouterr().err
    assert not out_dir.exists()


def test_jev_scanner_successful_empty_scan(jev_server, tmp_path):
    client, _, received = jev_server
    capture = write_capture(tmp_path, 0, EMPTY_SCAN)
    args = arguments(deps, ["--input", f"{capture}=services/agent"])
    sources = deps.collect_sources(args, ROOT)
    report = deps.CHECK.evaluate_sources(
        sources, ROOT, client, Report("deps", RECORDED_REVISION, client.model)
    )
    assert report.calls == 0
    assert report.findings == []
    assert received == []


def test_jev_scanner_advisory_exit_keeps_findings(jev_server, tmp_path):
    client, exchanges, received = jev_server
    text = (FIXTURES / "agent-advisories.jsonl").read_text()
    assert deps.validate_cargo_deny("services/agent", 1, text) == text
    capture = write_capture(tmp_path, 1, text)
    sources = deps.collect_sources(arguments(deps, ["--input", f"{capture}=services/agent"]), ROOT)
    advisories = deps.group_advisories(
        deps.parse_diagnostics(text, "services/agent"), deps.DependencyIndex(ROOT)
    )
    assert {item.advisory_id for item in advisories} == {"RUSTSEC-2024-0436", "RUSTSEC-2026-0285"}
    for advisory in advisories:
        exchanges.append((deps.advisory_state(advisory), deps.advisory_questions(), DEPS_RESPONSE))
    report = deps.CHECK.evaluate_sources(
        sources, ROOT, client, Report("deps", RECORDED_REVISION, client.model)
    )
    assert report.calls == len(received) == 2
    assert len(report.findings) == 4
    assert [item.answer for item in report.findings] == [
        "production_reachable", "high", "production_reachable", "high"
    ]
    assert report.input_tokens == 1024
    assert report.cost_usd == pytest.approx(0.000043)
    assert report.model == DEPS_RESPONSE["model"]


@pytest.mark.parametrize("returncode,text", [
    (1, ""),
    (0, ""),
    (1, "configuration failed: private-credential-value"),
    (0, "not json\n" + EMPTY_SCAN),
    (101, EMPTY_SCAN),
    (-9, EMPTY_SCAN),
    (True, EMPTY_SCAN),
    (1.0, EMPTY_SCAN),
    (1, EMPTY_SCAN),
    (0, EMPTY_SCAN + EMPTY_SCAN),
    (0, json.dumps({"type": "summary", "fields": {}})),
    (0, '[]\n' + EMPTY_SCAN),
    (0, '{"type":"log","fields":{"message":"private-credential-value"}}\n' + EMPTY_SCAN),
])
def test_jev_scanner_failure_is_nonzero_with_nonsecret_evidence(tmp_path, capsys, returncode, text):
    capture = write_capture(tmp_path, returncode, text)
    out_dir = tmp_path / "report"
    assert cli.main([
        "deps", "--input", f"{capture}=services/agent", "--out", str(out_dir)
    ]) == 2
    error = capsys.readouterr().err
    assert "cargo-deny scan services/agent" in error
    assert f"stderr_sha256={hashlib.sha256(text.encode()).hexdigest()}" in error
    assert "exit=" in error
    assert "private-credential-value" not in error
    assert not out_dir.exists()


def test_jev_scanner_rejects_incomplete_or_contradictory_advisories():
    text = (FIXTURES / "agent-advisories.jsonl").read_text()
    records = [json.loads(line) for line in text.splitlines()]
    with pytest.raises(JevConfigError, match="contradicts completion counts"):
        deps.validate_cargo_deny("services/agent", 0, text)
    with pytest.raises(JevConfigError, match="completion summary"):
        deps.validate_cargo_deny("services/agent", 1, "\n".join(text.splitlines()[:-1]))
    records[1]["fields"]["advisory"].pop("id")
    with pytest.raises(JevConfigError, match="malformed advisory diagnostic"):
        deps.validate_cargo_deny("services/agent", 1, "\n".join(map(json.dumps, records)))
    for bad_count in (-1, True, "zero"):
        summary = json.loads(EMPTY_SCAN)
        summary["fields"]["advisories"]["errors"] = bad_count
        with pytest.raises(JevConfigError, match="invalid advisories completion counts"):
            deps.validate_cargo_deny("services/agent", 0, json.dumps(summary))
    with pytest.raises(JevConfigError, match="no advisory diagnostics"):
        deps.validate_cargo_deny("services/agent", 1, json.dumps(records[0]) + "\n" + json.dumps(records[-1]))
    summary = json.loads(EMPTY_SCAN)
    summary["fields"]["advisories"]["warnings"] = 1
    with pytest.raises(JevConfigError, match="no advisory diagnostics"):
        deps.validate_cargo_deny("services/agent", 0, json.dumps(summary))


@pytest.mark.parametrize("text", ["", "not json", '{"type":"diagnostic","fields":{}}'])
def test_jev_scanner_raw_capture_requires_completion(tmp_path, capsys, text):
    path = tmp_path / "incomplete.jsonl"
    path.write_text(text)
    out_dir = tmp_path / "report"
    assert cli.main([
        "deps", "--input", f"{path}=services/agent", "--out", str(out_dir)
    ]) == 2
    assert "cargo-deny scan services/agent" in capsys.readouterr().err
    assert not out_dir.exists()


@pytest.mark.parametrize("kind,source,translation,source_text,translation_text,source_evidence,translation_evidence", [
    (
        "fenced-command", "```sh\nmake paxd-build\n```", "```sh\nmake layerxd\n```",
        "make paxd-build", "make layerxd",
        {"commands": ["make paxd-build"], "destinations": []},
        {"commands": ["make layerxd"], "destinations": []},
    ),
    (
        "inline-command", "Run `make paxd-build`.", "Run `make layerxd`.",
        "Run make paxd-build.", "Run make layerxd.",
        {"commands": ["make paxd-build"], "destinations": []},
        {"commands": ["make layerxd"], "destinations": []},
    ),
    (
        "endpoint", "Endpoint https://rpc.example.test/", "Endpoint https://rpc.example.test/rpc",
        "Endpoint https://rpc.example.test/", "Endpoint https://rpc.example.test/rpc",
        {"commands": [], "destinations": ["https://rpc.example.test/"]},
        {"commands": [], "destinations": ["https://rpc.example.test/rpc"]},
    ),
    (
        "target-path", "See [guide](services/agent/README.md).", "See [guide](agent/README.md).",
        "See guide (services/agent/README.md).", "See guide (agent/README.md).",
        {"commands": [], "destinations": ["services/agent/README.md"]},
        {"commands": [], "destinations": ["agent/README.md"]},
    ),
    (
        "linked-endpoint", "[RPC](https://rpc.example.test/)", "[RPC](https://rpc.example.test/rpc)",
        "RPC (https://rpc.example.test/)", "RPC (https://rpc.example.test/rpc)",
        {"commands": [], "destinations": ["https://rpc.example.test/"]},
        {"commands": [], "destinations": ["https://rpc.example.test/rpc"]},
    ),
    (
        "reference-link", "[guide]: services/agent/README.md", "[guide]: agent/README.md",
        "[guide]: services/agent/README.md", "[guide]: agent/README.md",
        {"commands": [], "destinations": ["services/agent/README.md"]},
        {"commands": [], "destinations": ["agent/README.md"]},
    ),
    (
        "html-link", '<a href="services/agent/README.md">guide</a>', '<a href="agent/README.md">guide</a>',
        "services/agent/README.md guide", "agent/README.md guide",
        {"commands": [], "destinations": ["services/agent/README.md"]},
        {"commands": [], "destinations": ["agent/README.md"]},
    ),
])
def test_jev_docs_operational_drift_reaches_real_client(
    jev_server, tmp_path, kind, source, translation, source_text, translation_text,
    source_evidence, translation_evidence,
):
    client, exchanges, received = jev_server
    source_path = tmp_path / "README.md"
    translation_path = tmp_path / f"{kind}.de.md"
    source_path.write_text(f"## Guide\n{source}\n")
    translation_path.write_text(f"## Anleitung\n{translation}\n")
    expected = {
        "section": "Guide", "source": source_text, "translation": translation_text,
        "source_evidence": source_evidence, "translation_evidence": translation_evidence,
    }
    assert source_evidence != translation_evidence
    exchanges.append((expected, docs.QUESTIONS, DRIFT_RESPONSE))
    args = arguments(docs, ["--source", str(source_path), "--translations", str(translation_path)])
    report = docs.CHECK.evaluate_catalog(
        docs.configured_catalog(args), client, Report("docs", RECORDED_REVISION, client.model)
    )
    assert report.calls == len(received) == 1
    assert len(report.findings) == 2
    drift = next(item for item in report.findings if item.question == "drift_kind")
    assert drift.answer == "stale_reference"
    assert drift.route == "auto"
    assert drift.anchor == f"{translation_path}:1"
    assert drift.detail["source_anchor"] == f"{source_path}:1"
    assert drift.detail["translation_anchor"] == drift.anchor
    assert report.input_tokens == 128
    assert report.cost_usd == 0.00001
    output, _ = report.write(tmp_path / "report")
    assert any(item["answer"] == "stale_reference" for item in json.loads(output.read_text())["findings"])


def test_jev_docs_site_catalog_and_independent_page_anchors(jev_server, tmp_path):
    client, exchanges, received = jev_server
    real = docs.configured_catalog(arguments(docs))
    assert docs.TRANSLATION_CATALOG == (
        ("README.md", "docs/readme/README.*.md"),
        ("README.md", "docs/site/docs/overview/i18n/*.md"),
    )
    assert [source for source, _ in real] == [ROOT / "README.md"] * 2
    assert {page.name for page in real[1][1]} == {
        "de.md", "es.md", "fr.md", "ja.md", "pt-br.md", "ru.md", "zh-cn.md"
    }
    (tmp_path / "README.md").write_text("## Guide\n```sh\nmake paxd-build\n```\n")
    paths = [
        "docs/readme/README.de.md", "docs/readme/README.fr.md",
        "docs/site/docs/overview/i18n/de.md", "docs/site/docs/overview/i18n/fr.md",
    ]
    expected = {
        "section": "Guide", "source": "make paxd-build", "translation": "make layerxd",
        "source_evidence": {"commands": ["make paxd-build"], "destinations": []},
        "translation_evidence": {"commands": ["make layerxd"], "destinations": []},
    }
    for path in paths:
        page = tmp_path / path
        page.parent.mkdir(parents=True, exist_ok=True)
        page.write_text("## Anleitung\n```sh\nmake layerxd\n```\n")
        exchanges.append((expected, docs.QUESTIONS, DRIFT_RESPONSE))
    catalog = docs.configured_catalog(arguments(docs), tmp_path)
    assert len(catalog) == 2
    assert [len(pages) for _, pages in catalog] == [2, 2]
    report = docs.CHECK.evaluate_catalog(
        catalog, client, Report("docs", RECORDED_REVISION, client.model)
    )
    assert report.calls == len(received) == 4
    drift = [item for item in report.findings if item.question == "drift_kind"]
    assert len(drift) == 4
    assert {item.anchor for item in drift} == {f"{tmp_path / path}:1" for path in paths}
    assert all(item.answer == "stale_reference" for item in drift)


def test_jev_docs_empty_catalog_is_explicit(tmp_path, capsys):
    source = tmp_path / "README.md"
    source.write_text("## Guide\nCurrent instructions.\n")
    args = arguments(docs)
    with pytest.raises(JevConfigError, match="docs/readme/README.*empty"):
        docs.configured_catalog(args, tmp_path)
    readme = tmp_path / "docs/readme/README.de.md"
    readme.parent.mkdir(parents=True)
    readme.write_text("## Anleitung\nCurrent instructions.\n")
    with pytest.raises(JevConfigError, match="docs/site/docs/overview/i18n/.*empty"):
        docs.configured_catalog(args, tmp_path)
    out_dir = tmp_path / "report"
    assert cli.main([
        "docs", "--source", str(source), "--translations", str(tmp_path / "absent/*.md"),
        "--out", str(out_dir),
    ]) == 2
    assert "translation catalog" in capsys.readouterr().err
    assert not out_dir.exists()
    # A glob selecting just the source is also empty after self-comparisons are removed.
    args = arguments(docs, ["--source", str(source), "--translations", str(source)])
    with pytest.raises(JevConfigError, match="catalog.*empty"):
        docs.configured_catalog(args)


def test_jev_docs_presentation_noise_and_missing_sections(jev_server, tmp_path):
    client, exchanges, received = jev_server
    source = tmp_path / "README.md"
    translation = tmp_path / "README.de.md"
    source.write_text("## Guide\nRun `make paxd-build`.\n## Required\nDo not omit.\n")
    translation.write_text(
        '## Anleitung\nRun `make paxd-build`.\n'
        '![badge](https://images.example.test/badge.svg)\n<img src="noise.png">\n'
    )
    evidence = {"commands": ["make paxd-build"], "destinations": []}
    exchanges.append(({
        "section": "Guide", "source": "Run make paxd-build.", "translation": "Run make paxd-build.",
        "source_evidence": evidence, "translation_evidence": evidence,
    }, docs.QUESTIONS, FAITHFUL_RESPONSE))
    args = arguments(docs, ["--source", str(source), "--translations", str(translation)])
    report = docs.CHECK.evaluate_catalog(
        docs.configured_catalog(args), client, Report("docs", RECORDED_REVISION, client.model)
    )
    assert report.calls == len(received) == 1
    assert len(report.findings) == 1
    assert report.findings[0].answer == "missing_section"
    assert report.findings[0].anchor == f"{source}:3"
    assert report.findings[0].detail["translation"] == str(translation)


def test_jev_docs_fenced_heading_and_command_normalization():
    content = "## Guide\n```sh\n# shell comment\ncurl \\\n  --fail https://rpc.example.test/\n```\n## Next\nDone.\n"
    sections = docs.split_sections(Path("README.md"), content)
    assert [section.title for section in sections] == ["Guide", "Next"]
    assert docs.comparison_evidence(sections[0].body) == {
        "commands": ["# shell comment\ncurl --fail https://rpc.example.test/"],
        "destinations": [],
    }
    assert "https://rpc.example.test/" in docs.sanitize(sections[0].body)
