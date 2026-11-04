from __future__ import annotations

import hashlib
import json
import threading
from functools import partial
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

from tools.jev import cli
from tools.jev.checks import docs
from tools.jev.client import (
    DEFAULT_MODEL,
    JevClient,
    JevRequestError,
    canonical_request,
    choice,
    noul,
    score,
)
from tools.jev.report import Report


FIXTURE = Path(__file__).parent / "fixtures" / "decision_response.json"
STATE = {"message": "My card was declined three times."}
QUESTIONS = {
    "is_urgent": noul("The customer needs help immediately."),
    "department": choice(
        "Which team should handle this message?",
        {
            "sales": "Wants to buy something",
            "billing": "Payment, invoice or card problem",
            "technical": "Software or device problem",
        },
    ),
    "frustration": score("Rate the frustration.", ["Calm", "Frustrated", "Very angry"]),
}


@pytest.fixture
def response_document():
    return json.loads(FIXTURE.read_bytes())


@pytest.fixture
def peer(tmp_path):
    responses = []
    received = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            raw = self.rfile.read(int(self.headers["Content-Length"]))
            received.append((self.path, dict(self.headers), raw))
            response = responses.pop(0)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(
        target=partial(server.serve_forever, poll_interval=0.01), daemon=True
    )
    thread.start()
    client = JevClient(
        api_key="recorded-key",
        endpoint=f"http://127.0.0.1:{server.server_port}/api/alpha/decisions",
        model=DEFAULT_MODEL,
        max_attempts=1,
        timeout_seconds=5,
        log_path=tmp_path / "calls.jsonl",
    )
    try:
        yield client, responses, received
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert responses == []
        for path, headers, raw in received:
            assert path == "/api/alpha/decisions"
            assert headers["Authorization"] == "Bearer recorded-key"
            assert headers["Content-Type"] == "application/json"
            document = json.loads(raw)
            assert set(document) == {"model", "state", "questions"}
            assert document["model"] == DEFAULT_MODEL
            assert raw == canonical_request(
                document["model"], document["state"], document["questions"]
            )[0]


def assert_rejected(peer, document, questions=QUESTIONS, state=STATE):
    client, responses, received = peer
    raw = json.dumps(document, separators=(",", ":")).encode("utf-8")
    responses.append(raw)
    before = len(received)
    with pytest.raises(JevRequestError) as raised:
        client.evaluate(state, questions)
    assert len(received) == before + 1
    assert json.loads(received[-1][2])["questions"] == questions
    error = raised.value
    assert error.status == 0
    assert error.body_excerpt
    assert error.request_sha256 == hashlib.sha256(received[-1][2]).hexdigest()
    assert error.response_sha256 == hashlib.sha256(raw).hexdigest()
    assert not client.log_path.exists()
    return error


@pytest.mark.parametrize(
    "value",
    [float("nan"), float("inf"), -float("inf"), -0.01, 1.01, True, "0.5", None, 10**400],
    ids=[
        "nan", "infinity", "negative-infinity", "negative", "over-one",
        "bool", "string", "null", "overflow",
    ],
)
@pytest.mark.parametrize(
    "answer_name,field,probability_key",
    [
        ("is_urgent", "noul", None),
        ("department", "confidence", None),
        ("frustration", "confidence", None),
        ("department", "probabilities", "billing"),
        ("frustration", "probabilities", "1"),
    ],
)
def test_jev_answer_numeric_bounds(
    peer, response_document, value, answer_name, field, probability_key
):
    answer = response_document["answers"][answer_name]
    if probability_key is None:
        answer[field] = value
    else:
        answer[field][probability_key] = value
    error = assert_rejected(peer, response_document)
    assert answer_name in error.body_excerpt


@pytest.mark.parametrize(
    "value",
    [float("nan"), float("inf"), -float("inf"), -0.01, 2.01, True, "1", None, 10**400],
    ids=[
        "nan", "infinity", "negative-infinity", "negative", "over-scale",
        "bool", "string", "null", "overflow",
    ],
)
def test_jev_requested_score_domain(peer, response_document, value):
    response_document["answers"]["frustration"]["score"] = value
    assert "frustration" in assert_rejected(peer, response_document).body_excerpt


@pytest.mark.parametrize("answer_name", ["department", "frustration"])
@pytest.mark.parametrize(
    "malformation",
    [
        "missing", "null", "list", "empty", "zero-mass", "under-mass",
        "over-mass", "missing-key", "unknown-key",
    ],
)
def test_jev_coherent_declared_probability_distribution(
    peer, response_document, answer_name, malformation
):
    answer = response_document["answers"][answer_name]
    probabilities = answer["probabilities"]
    if malformation == "missing":
        del answer["probabilities"]
    elif malformation == "null":
        answer["probabilities"] = None
    elif malformation == "list":
        answer["probabilities"] = list(probabilities.values())
    elif malformation == "empty":
        answer["probabilities"] = {}
    elif malformation == "zero-mass":
        answer["probabilities"] = dict.fromkeys(probabilities, 0.0)
    elif malformation == "under-mass":
        answer["probabilities"] = {key: value / 2 for key, value in probabilities.items()}
    elif malformation == "over-mass":
        probabilities[next(iter(probabilities))] = 0.1
    elif malformation == "missing-key":
        # The omitted option has zero mass: normalization alone cannot detect this.
        del probabilities[next(iter(probabilities))]
    elif malformation == "unknown-key":
        probabilities["undeclared"] = probabilities.pop(next(iter(probabilities)))
    error = assert_rejected(peer, response_document)
    assert answer_name in error.body_excerpt


@pytest.mark.parametrize(
    "malformation",
    [
        "missing-answers", "null-answers", "list-answers", "empty", "partial",
        "unknown", "replacement", "nonobject", "wrong-noul", "wrong-choice",
        "wrong-score", "unknown-type", "undeclared-choice",
    ],
)
def test_jev_incomplete_response(peer, response_document, malformation):
    answers = response_document["answers"]
    if malformation == "missing-answers":
        del response_document["answers"]
    elif malformation == "null-answers":
        response_document["answers"] = None
    elif malformation == "list-answers":
        response_document["answers"] = []
    elif malformation == "empty":
        answers.clear()
    elif malformation == "partial":
        del answers["is_urgent"]
    elif malformation == "unknown":
        answers["extra"] = {"type": "noul", "noul": 0.9}
    elif malformation == "replacement":
        answers["extra"] = answers.pop("is_urgent")
    elif malformation == "nonobject":
        answers["is_urgent"] = 0.95
    elif malformation == "wrong-noul":
        answers["is_urgent"] = dict(answers["department"])
    elif malformation == "wrong-choice":
        answers["department"] = dict(answers["is_urgent"])
    elif malformation == "wrong-score":
        answers["frustration"] = dict(answers["is_urgent"])
    elif malformation == "unknown-type":
        answers["is_urgent"]["type"] = "unsupported"
    elif malformation == "undeclared-choice":
        answers["department"]["choice"] = "undeclared"
    assert_rejected(peer, response_document)


def test_jev_score_uses_requested_scale_not_response_legend(peer, response_document):
    response_document["answers"]["frustration"]["legend"]["3"] = "invented level"
    response_document["answers"]["frustration"]["score"] = 3.0
    assert_rejected(peer, response_document)


def test_jev_score_rejects_value_above_shorter_requested_scale(peer, response_document):
    questions = dict(QUESTIONS)
    questions["frustration"] = score("Rate it.", ["Calm", "Angry"])
    response_document["answers"]["frustration"]["probabilities"] = {"0": 0.0, "1": 1.0}
    # The recorded 1.06 score is valid on the original scale, but not this one.
    assert_rejected(peer, response_document, questions=questions)


def test_jev_valid_recorded_roundtrip_preserves_evidence(peer):
    client, responses, received = peer
    raw = FIXTURE.read_bytes()
    responses.append(raw)
    evaluation = client.evaluate(STATE, QUESTIONS)
    assert len(received) == 1
    assert received[0][2] == canonical_request(client.model, STATE, QUESTIONS)[0]
    assert set(evaluation.answers) == set(QUESTIONS)
    assert evaluation.answers["is_urgent"].value == 0.95
    assert evaluation.answers["is_urgent"].confidence == 0.95
    assert evaluation.answers["department"].value == "billing"
    assert evaluation.answers["department"].probabilities == {
        "sales": 0.0, "billing": 0.88, "technical": 0.12,
    }
    assert evaluation.answers["department"].confidence == 0.82
    assert evaluation.answers["frustration"].value == 1.06
    assert evaluation.answers["frustration"].legend == {
        "0": "Calm", "1": "Frustrated", "2": "Very angry",
    }
    assert evaluation.request_sha256 == hashlib.sha256(received[0][2]).hexdigest()
    assert evaluation.response_sha256 == hashlib.sha256(raw).hexdigest()
    assert evaluation.request_id == "gen-dec-1789715517-F1dv7b9hjaZx7efS8tmu"
    assert evaluation.usage["input_tokens"] == 427
    entry = json.loads(client.log_path.read_text())
    assert entry["request_sha256"] == evaluation.request_sha256
    assert entry["response_sha256"] == evaluation.response_sha256
    assert entry["request_id"] == evaluation.request_id
    assert "declined" not in client.log_path.read_text()


@pytest.mark.parametrize("probability", [0.0, 0.5, 1.0])
@pytest.mark.parametrize("score_value", [0.0, 1.06, 2.0])
def test_jev_accepts_probability_and_score_boundaries(
    peer, response_document, probability, score_value
):
    client, responses, received = peer
    answers = response_document["answers"]
    answers["is_urgent"]["noul"] = probability
    answers["department"]["confidence"] = probability
    answers["department"]["probabilities"] = {
        "sales": 0.0, "billing": probability, "technical": 1.0 - probability,
    }
    answers["frustration"]["confidence"] = probability
    answers["frustration"]["score"] = score_value
    responses.append(json.dumps(response_document).encode())
    evaluation = client.evaluate(STATE, QUESTIONS)
    assert len(received) == 1
    assert evaluation.answers["is_urgent"].value == probability
    assert evaluation.answers["is_urgent"].confidence == max(probability, 1.0 - probability)
    assert evaluation.answers["department"].confidence == probability
    assert evaluation.answers["frustration"].confidence == probability
    assert evaluation.answers["frustration"].value == score_value


@pytest.mark.parametrize("raw", [b"not json", b"\xff", b"[]"])
def test_jev_invalid_document_preserves_error_digests(peer, raw):
    client, responses, received = peer
    responses.append(raw)
    with pytest.raises(JevRequestError) as raised:
        client.evaluate(STATE, QUESTIONS)
    assert raised.value.request_sha256 == hashlib.sha256(received[0][2]).hexdigest()
    assert raised.value.response_sha256 == hashlib.sha256(raw).hexdigest()
    assert not client.log_path.exists()


@pytest.mark.parametrize(
    "malformation",
    ["empty", "partial", "unknown", "wrong-kind", "undeclared-choice", "nan"],
)
def test_jev_incomplete_response_cli_failure(peer, tmp_path, capsys, malformation):
    client, responses, received = peer
    source = tmp_path / "source.md"
    translation = tmp_path / "translation.md"
    source.write_text("# Guide\n\nUse the current endpoint.\n")
    translation.write_text("# Guide\n\nUse the current endpoint.\n")
    answers = {
        docs.SAME_CLAIMS: {"type": "noul", "noul": 0.99},
        docs.DRIFT_KIND: {
            "type": "choice", "choice": docs.FAITHFUL, "confidence": 0.96,
            "probabilities": {
                key: 0.96 if key == docs.FAITHFUL else 0.01 for key in docs.DRIFT_KINDS
            },
        },
    }
    if malformation == "empty":
        answers.clear()
    elif malformation == "partial":
        del answers[docs.SAME_CLAIMS]
    elif malformation == "unknown":
        answers["extra"] = {"type": "noul", "noul": 0.99}
    elif malformation == "wrong-kind":
        answers[docs.DRIFT_KIND] = {"type": "noul", "noul": 0.99}
    elif malformation == "undeclared-choice":
        answers[docs.DRIFT_KIND]["choice"] = "undeclared"
    elif malformation == "nan":
        answers[docs.SAME_CLAIMS]["noul"] = float("nan")
    responses.append(
        json.dumps({"model": DEFAULT_MODEL, "answers": answers, "id": "recorded-cli"}).encode()
    )
    report = Report(check=docs.CHECK_NAME, revision="recorded-revision", model=client.model)
    evaluate = partial(
        docs.DocsCheck().evaluate_catalog, [(source, [translation])], client, report
    )
    out_dir = tmp_path / "advisory"
    assert cli.evaluate_and_write(evaluate, out_dir) == 1
    captured = capsys.readouterr()
    assert "jev call failed:" in captured.err
    assert captured.out == ""
    assert len(received) == 1
    assert json.loads(received[0][2])["questions"] == docs.QUESTIONS
    assert not out_dir.exists()
    assert not client.log_path.exists()
    assert report.calls == 0
    assert report.findings == []


def test_jev_valid_response_reaches_cli_report(peer, tmp_path, capsys):
    client, responses, received = peer
    source = tmp_path / "source.md"
    translation = tmp_path / "translation.md"
    source.write_text("# Guide\n\nUse the current endpoint.\n")
    translation.write_text("# Guide\n\nUse the current endpoint.\n")
    raw_response = json.dumps(
        {
            "model": DEFAULT_MODEL,
            "id": "recorded-cli-success",
            "answers": {
                docs.SAME_CLAIMS: {"type": "noul", "noul": 0.99},
                docs.DRIFT_KIND: {
                    "type": "choice", "choice": docs.FAITHFUL, "confidence": 0.96,
                    "probabilities": {
                        key: 0.96 if key == docs.FAITHFUL else 0.01
                        for key in docs.DRIFT_KINDS
                    },
                },
            },
            "usage": {"input_tokens": 128, "output_tokens": 24, "cost": 0.00001},
        }
    ).encode()
    responses.append(raw_response)
    report = Report(check=docs.CHECK_NAME, revision="recorded-revision", model=client.model)
    evaluate = partial(
        docs.DocsCheck().evaluate_catalog, [(source, [translation])], client, report
    )
    out_dir = tmp_path / "advisory"
    assert cli.evaluate_and_write(evaluate, out_dir) == 0
    captured = capsys.readouterr()
    assert captured.err == ""
    assert captured.out.strip() == str(out_dir / "docs.md")
    assert len(received) == 1
    written = json.loads((out_dir / "docs.json").read_text())
    assert written["calls"] == 1
    assert written["findings"] == []
    assert written["revision"] == "recorded-revision"
    assert written["input_tokens"] == 128
    entry = json.loads(client.log_path.read_text())
    assert entry["request_sha256"] == hashlib.sha256(received[0][2]).hexdigest()
    assert entry["response_sha256"] == hashlib.sha256(raw_response).hexdigest()
