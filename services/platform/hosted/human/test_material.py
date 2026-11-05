import importlib.util
import inspect
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import yaml

import provision

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
spec = importlib.util.spec_from_file_location('material', HERE / 'material.py')
material = importlib.util.module_from_spec(spec)
spec.loader.exec_module(material)


class MaterialTests(unittest.TestCase):
    def owner_policy(self):
        policy = provision.owner_policy()
        self.assertEqual(policy, json.loads((HERE / 'beta-owner-policy.json').read_text()))
        self.assertRegex(policy['limit']['id'], r'\A[0-9a-f]{32}\Z')
        self.assertNotEqual(int(policy['limit']['id'], 16), 0)
        return policy

    def test_bundled_owner_policy_projects_to_sixteen_byte_limit_id(self):
        policy = self.owner_policy()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / 'evidence'
            evidence.mkdir(mode=0o700)
            config = root / 'agent-config'
            config.mkdir(mode=0o700)
            material.write(evidence, 'authority.json', json.dumps({
                'core-clock-horizon': policy['core-clock-horizon']}))
            material.write(evidence, 'agent.json', json.dumps({
                'HUMAN_LIMIT_ID': policy['limit']['id'],
                'HUMAN_LIMIT_NAME': policy['limit']['name'],
                'HUMAN_LIMIT_CEILING': policy['limit']['ceiling']}))
            status = material.policy_graph_status(
                evidence, root / 'journal', root / 'deployment.json',
                root / 'module-registry.json', root / 'bundle', root / 'registry',
                root / 'authority', 402, 31337)['nodes']['owner-policy']
            self.assertEqual(status['state'], 'ready')
            self.assertIsNone(status['reason'])
            material.write_agent_config(config, material.protected_json(evidence / 'agent.json'))
            projected = config / 'LAYERX_AGENT_HUMAN_LIMIT_ID'
            limit_id = projected.read_text()
            self.assertEqual(limit_id, policy['limit']['id'])
            self.assertRegex(limit_id, r'\A[0-9a-f]{32}\Z')
            self.assertEqual(len(bytes.fromhex(limit_id)), 16)
            self.assertEqual(bytes.fromhex(limit_id).hex(), policy['limit']['id'])
            self.assertEqual(projected.stat().st_mode & 0o777, 0o600)

    def test_owner_policy_refuses_malformed_zero_and_legacy_limit_ids(self):
        policy = self.owner_policy()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'provision.py'
            source.write_text((HERE / 'provision.py').read_text())
            spec = importlib.util.spec_from_file_location('limit_policy_provision', source)
            copied_provision = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(copied_provision)
            policy_path = root / 'beta-owner-policy.json'
            policy_path.write_text(json.dumps(policy))
            self.assertEqual(copied_provision.owner_policy(), policy)
            for limit_id in ('', 'g' * 32, policy['limit']['id'].upper(),
                             policy['limit']['id'][:-1], '0' * 32,
                             policy['limit']['id'] * 2,
                             'ef060d2533e9bcc511e5583aa2802ee44baa120fc7de7b03a55bce6b02cf8b71',
                             None, 1):
                with self.subTest(limit_id=limit_id):
                    policy['limit']['id'] = limit_id
                    policy_path.write_text(json.dumps(policy))
                    with self.assertRaises(copied_provision.Refused) as caught:
                        copied_provision.owner_policy()
                    self.assertEqual(str(caught.exception), f'{policy_path}: invalid limit.id')

    def test_owner_policy_limit_id_is_sixteen_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / 'evidence'
            evidence.mkdir(mode=0o700)
            config = root / 'agent-config'
            config.mkdir(mode=0o700)
            material.write(evidence, 'authority.json', json.dumps({'core-clock-horizon': 60}))
            limit_id = '0123456789abcdef0123456789abcdef'
            agent = {'HUMAN_LIMIT_CEILING': 1000, 'HUMAN_LIMIT_ID': limit_id * 2}
            agent_path = evidence / 'agent.json'
            material.write(evidence, 'agent.json', json.dumps(agent))

            def owner_status():
                return material.policy_graph_status(
                    evidence, root / 'journal', root / 'deployment.json',
                    root / 'module-registry.json', root / 'bundle', root / 'registry',
                    root / 'authority', 402, 31337)['nodes']['owner-policy']

            refused = owner_status()
            self.assertEqual(refused['state'], 'refused')
            self.assertEqual(refused['reason'],
                             material.BUNDLE_REFUSED + 'owner policy horizon or limit binding')
            agent['HUMAN_LIMIT_ID'] = limit_id
            agent_path.write_text(json.dumps(agent))
            ready = owner_status()
            self.assertEqual(ready['state'], 'ready')
            self.assertIsNone(ready['reason'])
            material.write_agent_config(config, material.protected_json(agent_path))
            projected = config / 'LAYERX_AGENT_HUMAN_LIMIT_ID'
            self.assertEqual(projected.read_text(), limit_id)
            self.assertEqual(len(bytes.fromhex(projected.read_text())), 16)
            self.assertEqual(projected.stat().st_mode & 0o777, 0o600)

    def test_registry_material_runs_no_handover_verify(self):
        self.assertNotIn('layerx-handover', inspect.getsource(material.hosted_registry_material_produce))
        self.assertNotIn('/usr/local/bin/layerx-handover', (HERE / 'material.py').read_text())

    def test_generated_bootstrap_and_protected_inputs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'human'
            root.mkdir(mode=0o700)
            for name in ('components', 'kms', 'config', 'agent-config'):
                (root / name).mkdir(mode=0o700)
            (root.parent / 'receipt-authority-replica-id').write_text('6c61796572782d626574612d726563656970742d617574686f726974792d3031')
            subprocess.run([sys.executable, str(HERE / 'material.py'), str(root), '402', '31337', ''], check=True)
            self.assertEqual((root / 'agent-config/LAYERX_AGENT_MODE').read_text(), 'human-owner')
            for path in root.rglob('*'):
                if path.is_file():
                    self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            protected = Path(directory) / 'policy.json'
            protected.write_text('{}')
            protected.chmod(0o600)
            self.assertEqual(material.protected_json(protected), {})
            alias = Path(directory) / 'alias.json'
            alias.symlink_to(protected)
            with self.assertRaises(ValueError):
                material.protected_json(alias)
            protected.chmod(0o640)
            with self.assertRaises(ValueError):
                material.protected_json(protected)
            protected.chmod(0o600)
            os.link(protected, Path(directory) / 'hardlink.json')
            with self.assertRaises(ValueError):
                material.protected_json(protected)
            with self.assertRaisesRegex(ValueError, r'protected producer file unavailable: components\.json$'):
                material.assemble_policy(Path(directory), Path(directory) / 'missing-deployment.json',
                                         Path(directory) / 'missing-registry.json',
                                         root / 'policy.json', 402, 31337)
            self.assertFalse((root / 'policy.json').exists())

    def test_ledger_funding_binds_current_bundle_and_retained_state(self):
        from test_repair_22_31_348 import merged_environment
        from merged_owner import LEDGER, PAYMENT
        with tempfile.TemporaryDirectory() as directory:
            with merged_environment(Path(directory)) as e:
                policy = provision.protected_json(e.bundle / 'policy.json')
                self.assertEqual(policy['movement'], {
                    'LEDGER_ASSET_ID': '0x' + e.asset,
                    'LEDGER_OWNER_ACCOUNT': '0x' + e.registration['owner_account'],
                    'PAXEER_LEDGER_CONTRACT': LEDGER, 'PAXEER_PAYMENT_CONTRACT': PAYMENT})
                self.assertEqual(provision.funding_evidence(e.work)['funding_source'], e.source)
                self.assertEqual(material.verify_bundle(e.bundle, 125, 125)['records'], 2)
                for name in ('PAXEER_EXIT_CONTRACT', 'PAXEER_WITHDRAWAL_CLAIMS_CONTRACT'):
                    self.assertNotIn(name, policy['components'])
                for name in ('PAXEER_VAULT', 'PAXEER_CLAIMS_CONTRACT', 'PAXEER_EXIT_CONTRACT',
                             'PAXEER_CHECKPOINT_REGISTRY', 'CUSTODY_REFERENCE'):
                    self.assertNotIn(name, policy['movement'])
                self.assertNotIn('0x0000000000000000000000000000000000001013', json.dumps(policy))
                self.assertFalse((e.inputs / 'movement-source.json').exists())
                before = {p.relative_to(e.bundle): p.read_bytes() for p in e.bundle.rglob('*') if p.is_file()}
                provision.assemble(e.work, e.registry, e.asset, e.work / 'registry-journal')
                material.assemble_policy(e.work / 'human-evidence', e.deployment, e.registry,
                                         e.bundle / 'policy.json', 125, 125)
                self.assertEqual(before, {p.relative_to(e.bundle): p.read_bytes()
                                         for p in e.bundle.rglob('*') if p.is_file()})

    def test_payment_components_ignore_retired_addresses_and_refuse_custody_policy(self):
        from test_repair_22_31_348 import merged_environment
        from merged_owner import LEDGER, PAYMENT
        with tempfile.TemporaryDirectory() as directory:
            with merged_environment(Path(directory)) as e:
                inputs = {name: (e.bundle / 'inputs' / name).read_bytes()
                          for name in material.EVIDENCE_INPUTS.values()}
                inputs.update({'producer-records/' + name: (e.bundle / 'inputs/producer-records' / name).read_bytes()
                               for name in material.PRODUCER_FILES})
                inputs.update({'deployment.json': e.deployment.read_bytes(),
                               'module-registry.json': e.registry.read_bytes()})
                records = provision.journal_records(e.work / 'registry-journal')
                for addresses in ({}, {'vault': '0x' + '20' * 20,
                        'withdrawal_claims': '0x' + '21' * 20, 'emergency_exit': '0x' + '22' * 20,
                        'checkpoint_registry': '0x' + '23' * 20}):
                    candidate = dict(inputs, **{'deployment.json': provision.encode_json(
                        dict(network_id=125, chain_id=125, addresses=addresses))})
                    policy = material.policy_from_inputs(candidate, records, 125, 125)
                    self.assertEqual(policy['components']['PAXEER_LEDGER_CONTRACT'], LEDGER)
                    self.assertEqual(policy['movement']['PAXEER_PAYMENT_CONTRACT'], PAYMENT)
                    for address in addresses.values():
                        self.assertNotIn(address, json.dumps(policy))
                movement = json.loads(inputs['movement-policy.json'])
                for changes in ({'CUSTODY_REFERENCE': 'old'}, {'PAXEER_CHECKPOINT_REGISTRY': '0x' + '23' * 20},
                                {'LEDGER_ASSET_ID': '0x' + 'ff' * 32},
                                {'LEDGER_OWNER_ACCOUNT': '0x' + 'ff' * 32}):
                    bad = dict(movement, **changes)
                    with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, 'ledger funding projection binding'):
                        material.policy_from_inputs(dict(inputs, **{'movement-policy.json': provision.encode_json(bad)}),
                                                    records, 125, 125)
                for changes in ({'network_id': 126}, {'chain_id': 126}):
                    deployment = dict(network_id=125, chain_id=125)
                    deployment.update(changes)
                    with self.assertRaisesRegex(ValueError, 'deployment network, chain'):
                        material.policy_from_inputs(dict(inputs, **{'deployment.json': provision.encode_json(deployment)}),
                                                    records, 125, 125)

    def test_passkey_relying_party_follows_the_deployed_web_origin(self):
        self.assertEqual(material.passkey_relying_party(''),
                         ('paxportwallet.com', 'https://paxportwallet.com'))
        self.assertEqual(material.passkey_relying_party('https://paxportwallet.com'),
                         ('paxportwallet.com', 'https://paxportwallet.com'))
        for refused in ('http://paxportwallet.com', 'https://localhost:19457',
                        'https://127.0.0.1', 'https://paxportwallet.com/',
                        'https://Human.Testnet.Layerx.Network', 'https://paxportwallet.com?a=1'):
            with self.assertRaises(ValueError):
                material.passkey_relying_party(refused)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'human'
            root.mkdir(mode=0o700)
            for name in ('components', 'kms', 'config', 'agent-config'):
                (root / name).mkdir(mode=0o700)
            (root.parent / 'receipt-authority-replica-id').write_text('6c61796572782d626574612d726563656970742d617574686f726974792d3031')
            subprocess.run([sys.executable, str(HERE / 'material.py'), str(root), '402', '31337', '',
                            'https://paxportwallet.com'], check=True)
            self.assertEqual((root / 'config/LAYERX_HUMAN_ORIGIN').read_text(), 'https://paxportwallet.com')
            self.assertEqual((root / 'config/LAYERX_HUMAN_RP_ID').read_text(), 'paxportwallet.com')

    def test_bring_up_publishes_the_web_origin_the_ceremony_configuration_uses(self):
        cluster = (ROOT / 'services/platform/hosted/tests/beta-cluster.sh').read_text()
        material_source = (ROOT / 'services/platform/hosted/human/material.sh').read_text()
        self.assertIn('HUMAN_WEB_HOST=paxportwallet.com\n', cluster)
        self.assertIn('HUMAN_WEB_URL="https://$HUMAN_WEB_HOST"\n', cluster)
        self.assertIn('"$LAYERX_BETA_HUMAN_POLICY_FILE" "${HUMAN_WEB_URL:-}"\n', material_source)
        node = yaml.safe_load_all((ROOT / 'services/platform/hosted/node/deployment.yaml').read_text())
        human = next(c for d in node if d['kind'] == 'StatefulSet'
                     for c in d['spec']['template']['spec']['containers'] if c['name'] == 'human')
        origin = next(e['value'] for e in human['env'] if e['name'] == 'LAYERX_HUMAN_WEB_ORIGIN')
        self.assertEqual(origin, 'https://paxportwallet.com')
        web = next(c for d in yaml.safe_load_all((ROOT / 'services/platform/hosted/human/web-deployment.yaml').read_text())
                   if d['kind'] == 'Deployment'
                   for c in d['spec']['template']['spec']['containers'] if c['name'] == 'web')
        self.assertEqual(next(e['value'] for e in web['env'] if e['name'] == 'LAYERX_HUMAN_WEB_ORIGIN'), origin)
        self.assertEqual(material.passkey_relying_party(origin)[1], origin)

    def test_explorer_read_principal_is_generated_admitted_and_published(self):
        cluster = (ROOT / 'services/platform/hosted/tests/beta-cluster.sh').read_text()
        provision = (ROOT / 'services/platform/hosted/human/provision.sh').read_text()
        self.assertEqual(cluster.count('    explorer_read_principal_generate "$d"\n'), 1)
        self.assertIn('openssl genpkey -algorithm ed25519 -out "$d/explorer-read.key"', cluster)
        self.assertNotIn('LAYERX_BETA_EXPLORER_READ', cluster)
        self.assertNotIn('LAYERX_BETA_EXPLORER_READ', provision)
        published = next(line for line in cluster.replace('\\\n', '').splitlines()
                         if 'apply_secret "$ns" layerx-explorer-index ' in line)
        self.assertIn('--from-file=program-token="$s/explorer-program.token"', published)
        self.assertIn('--from-file=read-key="$s/explorer-read.seed.hex"', published)
        self.assertIn('--from-file=sequencer-public-key="$s/sequencer-public-key"', published)
        self.assertIn('explorer_did="did:layerx:$explorer_public"\n', provision)
        self.assertIn('cat "$input/owner-admission.txt" > "$input/genesis-admission.txt"\n', provision)
        self.assertIn("    ' < \"$input/genesis-admission.txt\"\n", provision)
        self.assertNotIn("' < \"$input/owner-admission.txt\"", provision)
        self.assertIn('--actor "$explorer_did"', provision)
        self.assertIn('fail "explorer-read.pub.hex: the explorer read principal key is missing from $SECRETS_DIR"', provision)
        registry = list(yaml.safe_load_all((ROOT / 'services/platform/hosted/registry/deployment.yaml').read_text()))
        pod = next(d for d in registry if d['kind'] == 'StatefulSet')['spec']['template']['spec']
        index = next(c for c in pod['containers'] if c['name'] == 'explorer-index')
        env = {e['name']: e for e in index['env']}
        mount = next(m for m in index['volumeMounts'] if m['name'] == 'explorer-read')
        self.assertTrue(mount['readOnly'])
        volume = next(v for v in pod['volumes'] if v['name'] == 'explorer-read')['secret']
        self.assertEqual(volume['secretName'], 'layerx-explorer-index')
        self.assertNotIn('optional', volume)
        self.assertEqual({item['key'] for item in volume['items']}, {'read-key', 'sequencer-public-key'})
        paths = {mount['mountPath'] + '/' + item['path'] for item in volume['items']}
        self.assertEqual({env['LAYERX_EXPLORER_READ_KEY_FILE']['value'],
                          env['LAYERX_EXPLORER_READ_SEQUENCER_PUBLIC_KEY_FILE']['value']}, paths)
        self.assertEqual(env['LAYERX_EXPLORER_READ_ENDPOINT']['value'],
                         'https://layerx-pending-core.layerx-testnet.svc.cluster.local:9443')
        self.assertEqual(env['LAYERX_EXPLORER_READ_CA_DER']['value'], env['LAYERX_EXPLORER_AUTHORITY_CA_DER']['value'])
        self.assertEqual(env['LAYERX_EXPLORER_READ_NETWORK_ID']['valueFrom']['configMapKeyRef'],
                         {'name': 'layerx-node-config', 'key': 'network-id'})
        self.assertEqual(env['LAYERX_EXPLORER_READ_FEE_LIMIT']['value'], '50000000')
        self.assertIn('[ -s "$LAYERX_EXPLORER_OBSERVATION_DIR/naming-program" ]', index['args'][0])
        self.assertIn('export LAYERX_EXPLORER_NAMING_PROGRAM\n', index['args'][0])
        bridge = next(d for d in registry if d['kind'] == 'NetworkPolicy' and d['metadata']['name'] == 'layerx-registry-lni-bridge')
        self.assertEqual(bridge['spec']['podSelector'], {'matchLabels': {'app': 'layerx-node'}})
        self.assertIn({'protocol': 'TCP', 'port': 9443}, bridge['spec']['ingress'][0]['ports'])

    def test_bootstrap_has_no_unpublished_human_secret_dependencies(self):
        deployment = ROOT / 'services/platform/hosted/node/deployment.yaml'
        declared = next(d for d in yaml.safe_load_all(deployment.read_text()) if d['kind'] == 'StatefulSet')
        self.assertFalse(any(c['name'].startswith('guarantor')
                             for c in declared['spec']['template']['spec']['containers']))
        expected = {c['name'] for c in declared['spec']['template']['spec']['containers']
                    if c['name'] not in {'human', 'components'} and not c['name'].startswith('human-')}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'node.yaml'
            subprocess.run([sys.executable, str(HERE / 'bootstrap.py'),
                            str(deployment), str(output)], check=True)
            stateful = next(d for d in yaml.safe_load_all(output.read_text()) if d['kind'] == 'StatefulSet')
            pod = stateful['spec']['template']['spec']
            self.assertIn('paxd', {c['name'] for c in pod['containers']})
            self.assertNotIn('layerxd', {c['name'] for c in pod['containers']})
            self.assertEqual({c['name'] for c in pod['containers']}, expected)
            self.assertFalse(any(c['name'].startswith('guarantor') for c in pod['containers']))
            self.assertFalse(any(c['name'].startswith('human') for c in pod['containers']))
            self.assertFalse(any(v.get('secret', {}).get('secretName', '').startswith('layerx-human-')
                                 for v in pod['volumes']))
            self.assertTrue(any(v.get('persistentVolumeClaim', {}).get('claimName') == 'layerx-human-state'
                                for v in pod['volumes']))

    def test_real_boundary_certificate_names_and_kms_client_issuance(self):
        cluster = (ROOT / 'services/platform/hosted/tests/beta-cluster.sh').read_text()
        issue = 'issue_cert() {' + cluster.split('issue_cert() {', 1)[1].split('\n}\n', 1)[0] + '\n}\n'
        call = 'issue_cert paxeer-boundary' + cluster.split('issue_cert paxeer-boundary', 1)[1].split('\n    issue_client_identity', 1)[0]
        with tempfile.TemporaryDirectory() as directory:
            env = dict(os.environ, CA_DIR=directory, TESTNET_NAMESPACE='layerx-testnet')
            script = ('set -euo pipefail\nsvc=layerx-testnet.svc.cluster.local\n' + issue
                      + 'openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes '
                      '-keyout "$CA_DIR/ca.key" -out "$CA_DIR/ca.crt" -days 1 -subj /CN=test-ca '
                      '-addext basicConstraints=critical,CA:TRUE >/dev/null 2>&1\n'
                      + call + '\nissue_cert executor executor clientAuth ""\n')
            subprocess.run(['bash', '-c', script], env=env, check=True)
            certificate = Path(directory) / 'paxeer-boundary/cert.pem'
            for host in ('paxeer-boundary.layerx-testnet.svc.cluster.local',
                         'paxeer-observer-boundary.layerx-testnet.svc.cluster.local'):
                subprocess.run(['openssl', 'verify', '-CAfile', str(Path(directory) / 'ca.crt'),
                                '-verify_hostname', host, str(certificate)], check=True, capture_output=True)
            refused = subprocess.run(['openssl', 'verify', '-CAfile', str(Path(directory) / 'ca.crt'),
                                      '-verify_hostname', 'unlisted.layerx-testnet.svc.cluster.local',
                                      str(certificate)], capture_output=True)
            self.assertNotEqual(refused.returncode, 0)
            subprocess.run(['openssl', 'verify', '-CAfile', str(Path(directory) / 'ca.crt'),
                            '-purpose', 'sslclient', str(Path(directory) / 'executor/cert.pem')],
                           check=True, capture_output=True)

    def test_real_observer_topology_refuses_missing_egress(self):
        source = (ROOT / 'services/platform/hosted/tests/topology-check.sh').read_text()
        code = source.split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
        module = {}
        exec(compile(code.rsplit('sys.exit(main())', 1)[0], 'topology-check.sh', 'exec'), module)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'paxeer.yaml'
            path.write_text((ROOT / 'services/platform/hosted/paxeer/deployment.yaml').read_text())
            topology = module['Topology']()
            for manifest in (ROOT / 'services/platform/hosted/node/deployment.yaml', path):
                for document in yaml.safe_load_all(manifest.read_text()):
                    topology.add(document, str(manifest), 'default')
            node = next(w for w in topology.workloads if w['name'] == 'layerx-node')
            paxeer = next(w for w in topology.workloads if w['name'] == 'paxeer')
            service = topology.services[('layerx-testnet', 'paxeer-observer-boundary')]
            self.assertEqual(service['ports'][0]['port'], '9443')
            self.assertEqual(service['ports'][0]['targetPort'], 'observer-https')
            self.assertEqual(paxeer['ports']['observer-https'], ('9444', 'TCP'))
            def admits():
                return topology.egress_admits(node, paxeer['ns'], paxeer['labels'], '9444', 'TCP', paxeer['ports'])[0]
            self.assertTrue(admits())
            self.assertTrue(topology.ingress_admits(paxeer, '9444', 'TCP', node)[0])
            policy = next(p for p in topology.policies if p['name'] == 'layerx-node-egress')
            policy['egress'] = [r for r in policy['egress'] if not any(str(p['port']) == '9444' for p in r.get('ports', []))]
            self.assertFalse(admits())
            self.assertTrue(topology.egress_admits(node, paxeer['ns'], paxeer['labels'], '9443', 'TCP', paxeer['ports'])[0])


if __name__ == '__main__':
    unittest.main()
