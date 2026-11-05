"""Replay recorded JavaScript codec vectors through the production projection."""

import json
from pathlib import Path

import pytest

import hyperfide_projection as projection


FIXTURES = Path(__file__).with_name('fixtures')
VECTORS = json.loads((FIXTURES / 'projection_numeric_vectors.json').read_text())['vectors']
ACCEPTED = [vector for vector in VECTORS if vector['accepted']]
REJECTED = [vector for vector in VECTORS if not vector['accepted']]
SOURCE = '''[meta]
slug = "numeric-vectors"
[feature.1]
title = "Numeric projection"
[req.1]
ac_1 = "Projection preserves the recorded numeric bytes."
[acceptance.1.1]
req = "1.1"
[task.1.1]
status = "pending"
wave = 0
requires = []
touches = ["tools/spec"]
reqs = ["1.1"]
build_cmd = "python3 -m py_compile tools/spec/hyperfide_projection.py"
verify_cmd = "python3 -m pytest -q tools/spec/test_repair_22_33_781.py"
time_limit_minutes = 1
'''


def load_profile():
    path = projection.ROOT / 'spec' / projection.SPEC_SLUG / 'hyperfide' / 'profile.example.json'
    profile, placeholders = projection.load_profile(path, allow_placeholders=True)
    assert placeholders
    return profile


@pytest.mark.parametrize('vector', ACCEPTED, ids=lambda vector: vector['name'])
def test_projection_json_number_recorded_digest_round_trip(vector):
    source = SOURCE + 'numeric_metadata = ' + vector['json'] + '\n'
    result = projection.project(source, load_profile())
    text1 = projection.render(result['doc1'])
    text2 = projection.render(result['doc2'])
    errors, summary = projection.self_check(result, text1, text2)
    assert errors == []
    assert summary['errors'] == 0
    assert summary['gen1_tasks'] == summary['gen2_tasks'] == 1

    for text in (source, text1, text2):
        fields = dict(projection.parse_kvx(text))['task.1.1']
        canonical = projection.canonical(fields['numeric_metadata'])
        assert canonical == vector['canonical']
        assert projection.digest(canonical) == vector['digest']


@pytest.mark.parametrize('vector', REJECTED, ids=lambda vector: vector['name'])
@pytest.mark.parametrize('container', ['%s', '[%s]', '{"payload":[%s]}'])
def test_projection_json_number_rejects_noncanonical_source(vector, container):
    source = SOURCE + 'numeric_metadata = ' + container % vector['json'] + '\n'
    with pytest.raises(projection.SpecError, match=r'(noncanonical number|unsafe integer) at line'):
        projection.project(source, load_profile())


@pytest.mark.parametrize('vector', REJECTED, ids=lambda vector: vector['name'])
@pytest.mark.parametrize('boundary', [projection.canonical, projection.emit_value])
def test_projection_json_number_rejects_direct_codec_inputs(vector, boundary):
    value = json.loads(vector['json'])
    with pytest.raises(projection.SpecError, match=r'(noncanonical number|unsafe integer) at'):
        boundary({'payload': [value]})


@pytest.mark.parametrize('raw', ['[NaN]', '[Infinity]', '[-Infinity]', '1e9999', '-1e9999'])
def test_projection_json_number_keeps_nonfinite_refusal(raw):
    with pytest.raises(projection.SpecError, match='non-finite number at line'):
        projection.parse_kvx(SOURCE + 'numeric_metadata = ' + raw + '\n')


def test_projection_json_number_strings_are_not_numeric_inputs():
    source = SOURCE + 'numeric_metadata = ["1.0", "-0.0", "1e21", "NaN", "Infinity"]\n'
    sections = projection.parse_kvx(source)
    value = dict(sections)['task.1.1']['numeric_metadata']
    assert value == ['1.0', '-0.0', '1e21', 'NaN', 'Infinity']
    assert dict(projection.parse_kvx(projection.render(sections)))['task.1.1']['numeric_metadata'] == value
