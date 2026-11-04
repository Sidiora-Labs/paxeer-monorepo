"""Bounded owner path proof and hosted explorer deployment contract."""
from io import StringIO
from pathlib import Path
import re
import sys
import unittest

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path.insert(0, str(HERE))


def test_TestRepairM_1402_OwnerTestCurrentSuccess():
    import test_owner_native as owner_tests

    suite = unittest.TestSuite()
    loader = unittest.TestLoader()
    for case in (owner_tests.MergedOwnerSuccessTests, owner_tests.GuardianBindingConsumptionTests):
        suite.addTests(loader.loadTestsFromTestCase(case))
    # The existing subprocess CLI case remains in its original suite; this gate
    # requires only in-process production components and loopback TLS peers.
    for name in (
        'test_missing_native_inputs_name_the_file_and_publish_nothing',
        'test_native_config_rejects_duplicate_and_unknown_fields',
        'test_reader_refuses_truncation_bounds_and_trailing_bytes',
        'test_real_native_receipt_signature_and_tamper_refusals',
    ):
        suite.addTest(owner_tests.NativeProducerRefusalTests(name))
    output = StringIO()
    result = unittest.TextTestRunner(stream=output, verbosity=2).run(suite)
    assert result.testsRun == 11, output.getvalue()
    assert not result.skipped and not result.expectedFailures and not result.unexpectedSuccesses, output.getvalue()
    assert result.wasSuccessful(), output.getvalue()


def test_TestRepairM_1402_WebExplorerRelay():
    source = (HERE / 'web-deployment.yaml').read_text()
    documents = list(yaml.safe_load_all(source))
    deployment = next(d for d in documents if d['kind'] == 'Deployment')
    pod = deployment['spec']['template']['spec']
    containers = {c['name']: c for c in pod['containers']}
    assert set(containers) == {'web', 'web-boundary'}
    env = {e['name']: e for e in containers['web']['env']}
    assert env['PAXEER_X_EXPLORER_BASE_URL']['valueFrom']['configMapKeyRef'] == {
        'name': 'layerx-human-web-explorer', 'key': 'base-url',
    }
    for retired in ('layerx-explorer-index', 'LAYERX_EXPLORER_API_ORIGIN',
                    'LAYERX_EXPLORER_PROGRAM_API_ORIGIN', 'LAYERX_EXPLORER_PROGRAM_BEARER_TOKEN',
                    'LAYERX_EXPLORER_NAMING_PROGRAM', 'LAYERX_EXPLORER_INDEX_URL',
                    'LAYERX_EXPLORER_PROGRAM_RELAY_PORT'):
        assert retired not in source
    assert env['NODE_EXTRA_CA_CERTS']['value'] == '/run/layerx/trust/ca.crt'
    trust = next(v for v in pod['volumes'] if v['name'] == 'trust')
    assert trust['secret']['secretName'] == 'layerx-internal-ca'
    assert trust['secret']['items'] == [{'key': 'ca.crt', 'path': 'ca.crt'}]
    assert 'optional' not in trust['secret']
    assert {'name': 'trust', 'mountPath': '/run/layerx/trust', 'readOnly': True} in containers['web']['volumeMounts']
    policies = {d['metadata']['name']: d['spec'] for d in documents if d['kind'] == 'NetworkPolicy'}
    outbound = policies['layerx-human-web']
    assert outbound['ingress'] == [] and set(outbound['policyTypes']) == {'Ingress', 'Egress'}
    assert {'to': [{'podSelector': {'matchLabels': {'app': 'paxeer-x-explorer-backend'}}}],
            'ports': [{'protocol': 'TCP', 'port': 443}]} in outbound['egress']
    inbound = policies['layerx-human-web-explorer']
    assert inbound['podSelector'] == {'matchLabels': {'app': 'paxeer-x-explorer-backend'}}
    assert inbound['ingress'] == [{'from': [{'podSelector': {'matchLabels': {'app': 'layerx-human-web'}}}],
                                  'ports': [{'protocol': 'TCP', 'port': 443}]}]
    assert 'layerx-program-registry' not in source

    # Step 11 records interface.8.err-core-program-read-refused as the current
    # contract. Deployment qualification must preserve that explicit refusal;
    # a successful program read belongs to the domain 10/14 wiring request in
    # repair_m1402_test.go, not to an invented /v1/programs response here.
    client = (ROOT / 'apps/web/src/explorer/client.ts').read_text()
    program = re.search(r'export async function programRecord\([^\n]+\{\n(.*?)\n\}', client, re.S)
    assert program is not None, 'Domain 10/14 must publish the web program-read consumer contract'
    assert program.group(1).strip() == '\n'.join((
        'if (!validExplorerIdentifier(identifier)) throw new TypeError("Invalid program identifier");',
        '  explorerBaseUrl();',
        '  throw new ExplorerInterfaceUnavailableError("program-read");',
    )), 'Reconcile changes to the documented program-read refusal with domains 10 and 14'
