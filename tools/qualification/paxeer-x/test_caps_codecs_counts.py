import importlib.util
import json
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]
definition = importlib.util.spec_from_file_location(
    'caps_codecs', Path(__file__).with_name('caps-codecs.py'))
caps_codecs = importlib.util.module_from_spec(definition)
definition.loader.exec_module(caps_codecs)
TEST_SOURCES = {
    'client': ROOT / 'services/agent/crates/layerx-client/tests/committed_caps.rs',
    'agentd': ROOT / 'services/agent/crates/layerx-agentd/tests/budget_schema.rs',
}
VECTORS = ROOT / 'tests/vectors/native-budget-records.json'


@pytest.mark.parametrize('name', ['client', 'agentd'])
def test_counts_match_test_sources_at_head(name):
    vectors = json.loads(VECTORS.read_text())
    counts = caps_codecs.qualification_counts(caps_codecs.TEST_SOURCES, vectors)
    assert caps_codecs.TEST_SOURCES[name] == TEST_SOURCES[name]
    expected = TEST_SOURCES[name].read_text().count('#[test]')
    assert expected > 0
    assert counts['test_groups'][name] == expected


def test_native_count_matches_committed_vectors():
    vectors = json.loads(VECTORS.read_text())
    counts = caps_codecs.qualification_counts(TEST_SOURCES, vectors)
    assert len(vectors) > 0
    assert counts['native_vectors'] == len(vectors)


def test_native_count_matches_vector_array():
    vectors = list(json.loads(VECTORS.read_text()).values())
    counts = caps_codecs.qualification_counts(TEST_SOURCES, vectors)
    assert len(vectors) > 0
    assert counts['native_vectors'] == len(vectors)
