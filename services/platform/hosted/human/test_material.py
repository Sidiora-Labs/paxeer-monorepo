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

    def test_custody_bindings_resolve_to_the_native_custody_precompile(self):
        import re
        import provision

        precompile = '0x0000000000000000000000000000000000001013'
        self.assertEqual(material.CUSTODY_PRECOMPILE, precompile)

        required_inputs = ('PAXEER_X_HUMAN_EVIDENCE_DIR', 'PAXEER_X_HUMAN_DEPLOYMENT',
                           'PAXEER_X_MODULE_REGISTRY', 'PAXEER_X_HUMAN_GENESIS_DIR',
                           'PAXEER_X_HUMAN_LEGACY_DEPLOYMENT')
        missing = [name for name in required_inputs if not os.environ.get(name)]
        if missing:
            self.skipTest('operator evidence inputs are not set: ' + ' '.join(missing))

        def required_path(name, directory=False):
            raw = os.environ.get(name)
            if not raw:
                self.fail('required genuine producer input is missing: ' + name)
            path = Path(raw)
            try:
                material.protected_file(path, 0o700 if directory else 0o600)
            except (ValueError, OSError):
                self.fail('required genuine producer input is not a canonical protected path: ' + name)
            return path

        source_evidence = required_path('PAXEER_X_HUMAN_EVIDENCE_DIR', directory=True)
        source_work = source_evidence.parent
        material.protected_file(source_work, 0o700)
        deployment = required_path('PAXEER_X_HUMAN_DEPLOYMENT')
        registry = required_path('PAXEER_X_MODULE_REGISTRY')
        genesis = required_path('PAXEER_X_HUMAN_GENESIS_DIR', directory=True)
        legacy_deployment = required_path('PAXEER_X_HUMAN_LEGACY_DEPLOYMENT')
        genesis_binding = material.genesis_binding(genesis)
        asset = (genesis / 'asset-id').read_text().strip()
        if re.fullmatch('[0-9a-f]{64}', asset) is None or int(asset, 16) == 0:
            self.fail('required genuine genesis asset-id is invalid')
        native = provision.protected_json(deployment)
        legacy = provision.protected_json(legacy_deployment)
        network, chain = native['network_id'], native['chain_id']
        if (type(network) is not int or not 0 < network < 2**32
                or type(chain) is not int or not 0 < chain < 2**64
                or legacy.get('network_id') != network or legacy.get('chain_id') != chain):
            self.fail('genuine native and legacy deployment inputs must share the configured network and chain')
        native_deployed = native['addresses']
        deployed = legacy['addresses']
        for name in ('vault', 'withdrawal_claims', 'emergency_exit'):
            value = deployed.get(name)
            if type(value) is not str or re.fullmatch('0x[0-9a-fA-F]{40}', value) is None or int(value, 16) == 0:
                self.fail('genuine legacy deployment input is missing the deployed address: ' + name)
        movement = provision.protected_json(source_work / 'human-evidence-input/movement-source.json')
        source_names = ['identity/source-binding.json', 'human-owner-result.json', 'naming-deployment-result.json']
        source_names += ['human-evidence-input/' + name for name in (
            'recovery-policy.json', 'owner-registration.json', 'treasury.json', 'sequencer.json',
            'movement-source.json', 'account-head-result.json', 'owner-native.json')]
        for name in ('owner-kms.json', 'onboarding-configuration.json'):
            path = source_work / 'human-evidence-input' / name
            if path.exists() or path.is_symlink():
                source_names.append('human-evidence-input/' + name)
        source_names += ['human-evidence-input/owner-native-run/' + label + suffix
                         for label in ('credit', 'identity', 'rotation', 'recovery')
                         for suffix in ('.activity', '.receipt')]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            work = root / 'work'
            work.mkdir(mode=0o700)
            for name in source_names:
                data = provision.protected_bytes(source_work / name)
                target = work / name
                target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                material.write_bytes(target, data)
            journal = work / 'registry-journal'
            journal.mkdir(mode=0o700)
            for name, data in provision.journal_records(source_evidence / 'journal').items():
                material.write_bytes(journal / name, data)
            provision.assemble(work, registry, asset, journal)
            evidence = work / 'human-evidence'
            bundle = root / 'human-policy'
            bundle.mkdir(mode=0o700)
            output = bundle / 'policy.json'
            material.assemble_policy(evidence, deployment, registry, output, network, chain)
            material.verify_bundle(bundle, network, chain)
            self.assertEqual(material.genesis_binding(genesis), genesis_binding)
            policy = provision.protected_json(output)
            for name in ('PAXEER_EXIT_CONTRACT', 'PAXEER_WITHDRAWAL_CLAIMS_CONTRACT'):
                self.assertEqual(policy['components'][name], precompile)
            for name in ('PAXEER_VAULT', 'PAXEER_CLAIMS_CONTRACT', 'PAXEER_EXIT_CONTRACT'):
                self.assertEqual(policy['movement'][name], precompile)
            self.assertEqual(policy['movement']['PAXEER_CHECKPOINT_REGISTRY'], native_deployed['checkpoint_registry'])
            self.assertEqual(policy['movement']['CUSTODY_REFERENCE'], movement['CUSTODY_REFERENCE'])
            written = output.read_text().lower()
            for name in ('vault', 'withdrawal_claims', 'emergency_exit'):
                self.assertNotIn(deployed[name].lower(), written)

    def test_custody_components_resolve_to_the_native_custody_precompile(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            material.protected_file(root, 0o700)
            legacy = {'network_id': 402, 'chain_id': 31337, 'addresses': {
                'checkpoint_registry': '0x0000000000000000000000000000000000001001',
                'vault': '0x0000000000000000000000000000000000002001',
                'withdrawal_claims': '0x0000000000000000000000000000000000002002',
                'emergency_exit': '0x0000000000000000000000000000000000002003'}}
            native = dict(legacy, addresses=dict(legacy['addresses'], **{
                name: material.CUSTODY_PRECOMPILE
                for name in ('vault', 'withdrawal_claims', 'emergency_exit')}))
            for name, deployment in (('native', native), ('legacy', legacy)):
                material.write_bytes(root / (name + '.json'), json.dumps(deployment).encode())
            native = material.protected_json(root / 'native.json')
            legacy = material.protected_json(root / 'legacy.json')
            self.assertEqual((native['network_id'], native['chain_id']),
                             (legacy['network_id'], legacy['chain_id']))
            for name, deployment in (('native', native), ('legacy', legacy)):
                with self.subTest(deployment=name):
                    policy = {'components': {
                        'PAXEER_EXIT_CONTRACT': legacy['addresses']['emergency_exit'],
                        'PAXEER_WITHDRAWAL_CLAIMS_CONTRACT': legacy['addresses']['withdrawal_claims']},
                        'movement': {'PAXEER_VAULT': legacy['addresses']['vault'],
                                     'PAXEER_CLAIMS_CONTRACT': legacy['addresses']['withdrawal_claims'],
                                     'PAXEER_EXIT_CONTRACT': legacy['addresses']['emergency_exit']}}
                    material.resolve_custody_components(policy, deployment)
                    for key in ('PAXEER_EXIT_CONTRACT', 'PAXEER_WITHDRAWAL_CLAIMS_CONTRACT'):
                        self.assertEqual(policy['components'][key], material.CUSTODY_PRECOMPILE)
                    for key in ('PAXEER_VAULT', 'PAXEER_CLAIMS_CONTRACT', 'PAXEER_EXIT_CONTRACT'):
                        self.assertEqual(policy['movement'][key], material.CUSTODY_PRECOMPILE)
                    self.assertEqual(policy['movement']['PAXEER_CHECKPOINT_REGISTRY'],
                                     native['addresses']['checkpoint_registry'])
                    for key in ('vault', 'withdrawal_claims', 'emergency_exit'):
                        self.assertNotIn(legacy['addresses'][key], json.dumps(policy))
            for checkpoint in ('0x' + '0' * 40, 'invalid', None):
                with self.subTest(checkpoint=checkpoint):
                    policy = {'components': {}, 'movement': {}}
                    with self.assertRaisesRegex(ValueError, 'deployed checkpoint registry$'):
                        material.resolve_custody_components(
                            policy, {'addresses': {'checkpoint_registry': checkpoint}})
                    self.assertEqual(policy, {'components': {}, 'movement': {}})

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
            self.assertIn('layerxd', {c['name'] for c in pod['containers']})
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
        cluster = (ROOT / 'services/platform/hosted/tests/beta-cluster.sh').read_text()
        observer = cluster.split("<<'PYOBSERVER'\n", 1)[1].split('\nPYOBSERVER\n', 1)[0]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'paxeer.yaml'
            path.write_text((ROOT / 'services/platform/hosted/paxeer/deployment.yaml').read_text())
            subprocess.run([sys.executable, '-c', observer, str(path)], check=True)
            topology = module['Topology']()
            for manifest in (ROOT / 'services/platform/hosted/node/deployment.yaml', path):
                for document in yaml.safe_load_all(manifest.read_text()):
                    topology.add(document, str(manifest), 'default')
            node = next(w for w in topology.workloads if w['name'] == 'layerx-node')
            paxeer = next(w for w in topology.workloads if w['name'] == 'paxeer')
            service = topology.services[('layerx-testnet', 'paxeer-observer-boundary')]
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
