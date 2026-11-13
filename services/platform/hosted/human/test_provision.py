#!/usr/bin/env python3
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import provision

IDENTITY_PROVIDER_VARIABLE = 'LAYERX_HUMAN_IDENTITY_PROVIDER_BIN'
IDENTITY_PROVIDER_DEFAULT = (Path(__file__).resolve().parents[4]
                             / 'services/human/target/debug/layerx-human-identity-provider')


def identity_provider_binary():
    binary = Path(os.environ.get(IDENTITY_PROVIDER_VARIABLE) or IDENTITY_PROVIDER_DEFAULT)
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise FileNotFoundError(
            f'required real provider binary: {binary}; build it with '
            f'`cargo build --locked --manifest-path services/human/Cargo.toml '
            f'-p layerx-human-identity-provider`, or set {IDENTITY_PROVIDER_VARIABLE} '
            f'to a built layerx-human-identity-provider')
    return binary


def generated_catalog(directory, binary):
    import yaml
    root = Path(directory).resolve()
    repo = Path(__file__).resolve().parents[4]
    registry = json.loads((repo / 'services/interop/deploy/gateway/module-registry.example.json').read_text())
    node = list(yaml.safe_load_all((repo / 'services/platform/hosted/node/deployment.yaml').read_text()))
    config = next(d for d in node if d['kind'] == 'ConfigMap' and 'asset-id' in d.get('data', {}))
    asset = config['data']['asset-id']
    registry['schema_version'] = 2
    registry['network_id'] = int(config['data']['network-id'])
    registry['assets'] = [{'asset': asset}]
    provision.write_json(root / 'registry.json', registry)
    for name in ('treasury', 'sequencer'):
        key = root / (name + '.key')
        subprocess.run(['openssl', 'genpkey', '-algorithm', 'ED25519', '-out', str(key)],
                       check=True, capture_output=True)
        key.chmod(0o600)
        public = subprocess.run(['openssl', 'pkey', '-in', str(key), '-pubout', '-outform', 'DER'],
                                check=True, capture_output=True).stdout[-32:].hex()
        result = subprocess.run([str(binary), 'provision-account'],
                                input=json.dumps({'did': 'did:layerx:' + public}),
                                text=True, capture_output=True, check=True)
        provision.write_json(root / (name + '.json'), json.loads(result.stdout))
    result = provision.purpose_catalog(Path(provision.__file__).with_name('beta-purpose-catalog.json'),
                                      root / 'registry.json', root / 'treasury.json', root / 'sequencer.json', asset)
    provision.write_json(root / 'purpose-catalog.json', result)
    return result


class RegistrationInputTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name).resolve()
        self.input = self.root / 'human-evidence-input'
        self.input.mkdir(mode=0o700)
        self.path = self.input / 'owner-registration.json'

    def write(self, data):
        self.path.write_text(data)
        self.path.chmod(0o600)

    def refused(self):
        with self.assertRaises(provision.Refused) as caught:
            provision.owner_registration(self.root)
        self.assertIn(str(self.path), str(caught.exception))
        self.assertFalse((self.root / 'human-evidence').exists())

    def test_provisioning_makes_no_custody_deposit(self):
        here = Path(provision.__file__).resolve().parent
        for name in ('owner_custody.py', 'test_owner_custody.py'):
            self.assertFalse(os.path.lexists(here / name), name)
        script = (here / 'provision.sh').read_text()
        for retired in ('human_custody_step', 'explorer_read_principal_fund', 'owner_custody.py'):
            self.assertNotIn(retired, script)
        result = subprocess.run(
            ['bash', '--noprofile', '--norc', '-c',
             'set -euo pipefail\n'
             'source "$REPO_ROOT/services/platform/hosted/human/provision.sh"\n'
             'declare -F\n'],
            env=dict(os.environ, REPO_ROOT=str(here.parents[3])),
            text=True, capture_output=True, check=True)
        functions = {line.split()[-1] for line in result.stdout.splitlines()}
        self.assertTrue({'human_native_owner_prepare', 'human_native_provision',
                         'human_evidence_provision'} <= functions)
        self.assertTrue({'human_custody_step', 'explorer_read_principal_fund'}.isdisjoint(functions))
        for flag in ('--prepare-explorer-read-funding', '--sign-explorer-read-credit',
                     '--submit-explorer-read-credit'):
            with self.subTest(flag=flag):
                result = subprocess.run(
                    [sys.executable, str(here / 'provision.py'), '--validate-owner-registration',
                     '--work-dir', str(self.root), flag],
                    text=True, capture_output=True, check=False)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertIn(f'unrecognized arguments: {flag}', result.stderr)
                self.assertEqual(result.stdout, '')
        self.assertEqual(list(self.root.iterdir()), [self.input])
        self.assertEqual(list(self.input.iterdir()), [])

    def test_owner_policy_has_no_movement_intervals(self):
        policy = provision.owner_policy()
        self.assertNotIn('movement', policy)
        source = Path(provision.__file__)
        self.assertEqual(policy, json.loads(source.with_name('beta-owner-policy.json').read_text()))
        copied_source = self.root / 'provision.py'
        copied_source.write_text(source.read_text())
        spec = importlib.util.spec_from_file_location('policy_provision', copied_source)
        copied_provision = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(copied_provision)
        policy_path = self.root / 'beta-owner-policy.json'
        policy_path.write_text(json.dumps(policy))
        self.assertEqual(copied_provision.owner_policy(), policy)
        policy['movement'] = {
            'PAXEER_CONFIRMATIONS': 12,
            'CHECKPOINT_INTERVAL_SECONDS': 60,
            'PAXEER_BLOCK_SECONDS': 2,
            'REMINDER_INTERVAL_SECONDS': 300,
        }
        policy_path.write_text(json.dumps(policy))
        with self.assertRaises(copied_provision.Refused) as caught:
            copied_provision.owner_policy()
        self.assertEqual(str(caught.exception), f'{policy_path}: invalid owner policy')

    def test_missing_wrong_mode_symlink_hardlink_and_directory_refuse(self):
        self.refused()
        self.write('{}')
        self.path.chmod(0o644)
        self.refused()
        self.path.chmod(0o600)
        linked = self.input / 'linked.json'
        os.link(self.path, linked)
        self.refused()
        self.path.unlink()
        self.path.symlink_to(linked)
        self.refused()
        self.path.unlink()
        self.path.mkdir(mode=0o600)
        self.refused()

    def test_duplicate_nonfinite_unknown_and_missing_fields_refuse(self):
        for data in ('{}', '[]', '{"owner_account":1,"owner_account":2}',
                     '{"owner_account":NaN}', '{"unexpected":true}'):
            with self.subTest(data=data):
                self.write(data)
                if data == '[]':
                    self.assertEqual(provision.protected_json(self.path), [])
                    with self.assertRaises(provision.Refused) as caught:
                        provision.fields(provision.protected_json(self.path),
                                         'owner_account authority identity', self.path, 'owner registration')
                    self.assertIn(str(self.path), str(caught.exception))
                    with self.assertRaisesRegex(AttributeError, "'list' object has no attribute 'get'"):
                        provision.owner_registration(self.root)
                    result = self.run_cli()
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(result.stdout, '')
                    self.assertIn("'list' object has no attribute 'get'", result.stderr)
                    self.assertFalse((self.root / 'human-evidence').exists())
                else:
                    self.refused()

    def test_real_owner_output_is_not_a_protocol_registration(self):
        binary = identity_provider_binary()
        self.assertTrue(binary.is_file(), f'required real provider binary: {binary}')
        policy = self.root / 'recovery-policy.json'
        policy.write_text(json.dumps({'root': list(os.urandom(32)), 'threshold': 1,
                                      'delay_seconds': 86400}))
        policy.chmod(0o600)
        provision.write_json(self.input / 'recovery-policy.json', json.loads(policy.read_text()))
        env = dict(os.environ,
                   LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT=str(self.root / 'state'),
                   LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE=str(policy))
        request = json.dumps({'email': 'owner@example.com', 'display_name': 'Owner',
                              'idempotency_key': 'registration-contract', 'now': 1})
        result = subprocess.run([str(binary), 'provision-owner'], input=request,
                                text=True, capture_output=True, env=env, check=True)
        owner = json.loads(result.stdout)
        self.assertEqual(set(owner), {'principal', 'did', 'recovery_root', 'recovery_threshold',
                                      'recovery_delay_seconds'})
        self.assertIsInstance(owner['recovery_root'], list)
        self.assertEqual(len(owner['recovery_root']), 32)
        self.assertRegex(owner['principal'], r'\Aact_[0-9a-f]{64}\Z')
        self.assertEqual(owner['did'], 'did:layerx:' + owner['principal'])
        result_path = self.root / 'owner-result.json'
        provision.write_json(result_path, owner)
        self.assertEqual(provision.owner_result(self.root, result_path), owner)
        changed = dict(owner, recovery_threshold=2)
        result_path.unlink()
        provision.write_json(result_path, changed)
        with self.assertRaises(provision.Refused):
            provision.owner_result(self.root, result_path)
        self.assertEqual(owner['recovery_root'], json.loads(policy.read_text())['root'])
        self.assertEqual(owner['recovery_threshold'], 1)
        self.assertEqual(owner['recovery_delay_seconds'], 86400)
        replay = subprocess.run([str(binary), 'provision-owner'], input=request,
                                text=True, capture_output=True, env=env, check=True)
        self.assertEqual(json.loads(replay.stdout), owner)
        self.write(result.stdout)
        self.refused()
        self.assertNotIn(owner['did'], self.run_cli().stderr)

    def test_binding_refuses_actual_request_without_tenant(self):
        request = self.input / 'request.json'
        response = self.input / 'response.json'
        output = self.input / 'binding.json'
        provision.write_json(request, {'sub': 'owner', 'allowed_signer_public_keys': []})
        provision.write_json(response, {'sub': 'owner'})
        with self.assertRaises(provision.Refused) as caught:
            provision.preserve_binding(request, response, output)
        self.assertIn(str(request), str(caught.exception))
        self.assertFalse(output.exists())

    def test_binding_requires_matching_response_tenant_and_principal(self):
        request = self.input / 'request.json'
        response = self.input / 'response.json'
        output = self.input / 'binding.json'
        provision.write_json(request, {'tenant': 'beta', 'sub': 'owner'})
        for value in ({'sub': 'owner'}, {'tenant': 'other', 'sub': 'owner'},
                      {'tenant': 'beta', 'sub': 'other'}):
            provision.write_json(response, value)
            with self.assertRaises(provision.Refused):
                provision.preserve_binding(request, response, output)
            self.assertFalse(output.exists())
            response.unlink()
        provision.write_json(response, {'tenant': 'beta', 'sub': 'owner'})
        provision.preserve_binding(request, response, output)
        self.assertEqual(provision.protected_json(output), {'tenant': 'beta', 'principal': 'owner'})

    def test_catalog_refuses_missing_registry_before_output(self):
        template = Path(provision.__file__).with_name('beta-purpose-catalog.json')
        with self.assertRaises(provision.Refused) as caught:
            provision.purpose_catalog(template, self.input / 'module-registry.json',
                                      self.input / 'treasury.json', self.input / 'faucet.json', '')
        self.assertIn(str(self.input / 'module-registry.json'), str(caught.exception))
        self.assertFalse((self.root / 'human-evidence').exists())

    def test_protected_writer_never_overwrites_existing_files(self):
        provision.write_json(self.path, {'a': 1})
        self.assertEqual(self.path.stat().st_mode & 0o777, 0o600)
        with self.assertRaises(FileExistsError):
            provision.write_json(self.path, {'a': 2})
        self.assertEqual(provision.protected_json(self.path), {'a': 1})

    def test_job_mount_matches_runtime_identity_state(self):
        import yaml
        here = Path(provision.__file__).parent
        job = yaml.safe_load((here / 'provision-owner-job.yaml').read_text())
        node = next(d for d in yaml.safe_load_all((here.parent / 'node/deployment.yaml').read_text())
                    if d['kind'] == 'StatefulSet')
        pod = node['spec']['template']['spec']
        self.assertEqual([c['name'] for c in pod['containers']], ['paxd'])
        self.assertEqual([c['name'] for c in pod['initContainers']], ['native-home'])
        job_pod = job['spec']['template']['spec']
        container = next(c for c in job_pod['containers'] if c['name'] == 'provision-owner')
        state_var = 'LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT'
        state_root = next(e['value'] for e in container['env'] if e['name'] == state_var)
        self.assertEqual(state_root, '/var/lib/layerx/human/identity')
        job_mount = next(m for m in container['volumeMounts'] if m['name'] == 'state')
        self.assertEqual(job_mount, {'name': 'state', 'mountPath': '/var/lib/layerx/human',
                                     'subPath': 'identity'})
        self.assertEqual(Path(state_root).relative_to(job_mount['mountPath']), Path('identity'))
        state_volume = next(v for v in job_pod['volumes'] if v['name'] == 'state')
        self.assertEqual(state_volume['persistentVolumeClaim']['claimName'], 'layerx-human-state')
        self.assertFalse(any(v.get('persistentVolumeClaim', {}).get('claimName') == 'layerx-human-state'
                             for v in pod['volumes']))
        self.assertIn('exec /usr/local/bin/layerx-human-identity-provider provision-owner', container['args'][0])
        script = (here / 'provision.sh').read_text()
        self.assertIn("any(c['name'].startswith('human') for c in containers)", script)
        self.assertIn('Human runtime must not be enabled during owner provisioning', script)
        self.assertIn('Human runtime pod must be stopped before owner provisioning', script)
        self.assertEqual(job['spec']['backoffLimit'], 0)
        self.assertFalse(job_pod['automountServiceAccountToken'])

    def test_generated_catalog_uses_real_keys_and_production_registry_modules(self):
        binary = identity_provider_binary()
        catalog = generated_catalog(self.root, binary)
        self.assertEqual(len(catalog['presets'][0]['counterparties']), 2)
        self.assertNotEqual(*catalog['presets'][0]['counterparties'])
        self.assertEqual(provision.protected_json(self.root / 'purpose-catalog.json'), catalog)
        template = Path(provision.__file__).with_name('beta-purpose-catalog.json')
        for preset, configured in zip(catalog['presets'], json.loads(template.read_text())['presets']):
            self.assertEqual(preset['call_selectors'], configured['call_selectors'])
            self.assertNotIn('activity_types', preset)
            accounts = [provision.protected_json(self.root / (name + '.json'))['account']
                        for name in ('treasury', 'sequencer')]
            self.assertEqual(preset['counterparties'], [list(bytes.fromhex(a)) for a in accounts])
            asset = provision.protected_json(self.root / 'registry.json')['assets'][0]['asset']
            self.assertEqual(preset['budget_asset'], list(bytes.fromhex(asset)))
            self.assertEqual(preset['assets'], [list(bytes.fromhex(asset))])
        registry = self.root / 'registry.json'
        value = provision.protected_json(registry)
        del value['schema_version']
        registry.unlink()
        provision.write_json(registry, value)
        with self.assertRaisesRegex(
                ValueError, r'\AHuman owner bundle refused: canonical network-bound module registry\Z') as caught:
            provision.purpose_catalog(Path(provision.__file__).with_name('beta-purpose-catalog.json'),
                                      registry, self.root / 'treasury.json', self.root / 'sequencer.json',
                                      value['assets'][0]['asset'])
        self.assertIs(type(caught.exception), ValueError)

    def test_generated_catalog_refuses_invalid_selectors(self):
        catalog = generated_catalog(self.root, identity_provider_binary())
        template = json.loads(Path(provision.__file__).with_name('beta-purpose-catalog.json').read_text())
        template_path = self.root / 'catalog-template.json'
        asset = bytes(catalog['presets'][0]['budget_asset']).hex()
        invalid = (None, '', {}, [], [None], [1], [''], ['a9059cbb'], ['0Xa9059cbb'],
                   ['0xa9059c'], ['0xa9059cbb00'], ['0xa9059cbg'], ['0xa9059cbb\n'],
                   ['0xa9059cbb', '0xa9059cbb'], ['0xa9059cbb', '0xA9059CBB'])
        for selectors in invalid:
            with self.subTest(selectors=selectors):
                template['presets'][0]['call_selectors'] = selectors
                template_path.write_text(json.dumps(template))
                with self.assertRaises(provision.Refused) as caught:
                    provision.purpose_catalog(template_path, self.root / 'registry.json',
                                              self.root / 'treasury.json', self.root / 'sequencer.json', asset)
                self.assertIn(str(template_path), str(caught.exception))
        del template['presets'][0]['call_selectors']
        template_path.write_text(json.dumps(template))
        with self.assertRaises(provision.Refused):
            provision.purpose_catalog(template_path, self.root / 'registry.json',
                                      self.root / 'treasury.json', self.root / 'sequencer.json', asset)
        template['presets'][0]['call_selectors'] = ['0xA9059CBB']
        template_path.write_text(json.dumps(template))
        result = provision.purpose_catalog(template_path, self.root / 'registry.json',
                                           self.root / 'treasury.json', self.root / 'sequencer.json', asset)
        self.assertEqual(result['presets'][0]['call_selectors'], ['0xA9059CBB'])

    def test_journal_and_complete_assembly_fail_closed_without_upstream(self):
        with self.assertRaises(provision.Refused):
            provision.journal_records(None)
        with self.assertRaises(provision.Refused):
            provision.journal_records(self.root)
        with self.assertRaises(provision.Refused) as caught:
            provision.assemble(self.root, self.root / 'registry.json', '', None)
        self.assertIn('human-owner-result.json', str(caught.exception))
        self.assertFalse((self.root / 'human-evidence').exists())
        self.assertFalse(list(self.root.glob('.human-evidence-*')))

    def test_journal_export_selects_only_canonical_pairs(self):
        script = Path(provision.__file__).with_name('provision.sh').read_text()
        source = '$pod:/var/lib/layerx-registry-journal/pairs/.'
        self.assertIn(source, script)
        self.assertNotIn('$pod:/var/lib/layerx-registry-journal/.', script)

    def test_peer_binding_uses_named_fields_and_preserves_did_colons(self):
        path = self.input / 'source-binding.json'
        binding = {'tenant': 'beta_owner', 'principal': 'did:layerx:owner'}
        self.assertEqual(provision.peer_binding(binding, path),
                         'uid=4020;tenant=beta_owner;principal=did:layerx:owner')

    def test_named_peer_binding_preserves_identity_did_and_refuses_delimiters(self):
        request = self.input / 'request.json'
        response = self.input / 'response.json'
        output = self.input / 'binding.json'
        binding = {'tenant': 'beta', 'sub': 'did:layerx:beta:owner'}
        provision.write_json(request, binding)
        provision.write_json(response, binding)
        provision.preserve_binding(request, response, output)
        self.assertEqual(provision.peer_binding(provision.protected_json(output), output),
                         'uid=4020;tenant=beta;principal=did:layerx:beta:owner')
        for principal in ('owner', 'did::owner', 'did:layerx:', 'did:layerx:a;b',
                          'did:layerx:a,b', 'did:layerx:a b', 'did:layerx:a\x00b',
                          'did:layerx:' + 'a' * 246):
            with self.assertRaises(provision.Refused):
                provision.peer_binding({'tenant': 'beta', 'principal': principal}, output)
        for tenant in ('beta.prod', 'beta;prod', 'beta,prod', 'beta prod', ''):
            with self.assertRaises(provision.Refused):
                provision.peer_binding({'tenant': tenant, 'principal': binding['sub']}, output)

    def test_evidence_preflight_names_missing_registration_before_job(self):
        with self.assertRaises(provision.Refused) as caught:
            provision.evidence_inputs(self.root, self.root / 'module-registry.json', None)
        self.assertIn(str(self.path), str(caught.exception))
        self.assertFalse((self.root / 'human-owner-result.json').exists())
        self.assertFalse((self.root / 'human-evidence').exists())

    def test_cluster_sources_producer_and_runs_validation_after_production(self):
        cluster = Path(provision.__file__).parents[1] / 'tests/beta-cluster.sh'
        self.assertIn('source "$REPO_ROOT/services/platform/hosted/human/provision.sh"', cluster.read_text())
        script = Path(provision.__file__).with_name('provision.sh').read_text()
        body = script.split('human_evidence_provision() (', 1)[1].split('\n)\n', 1)[0]
        produced = body.index('human_native_provision')
        self.assertGreater(body.index('--validate-owner-registration'), produced)
        self.assertGreater(body.index('--validate-evidence-inputs'), produced)

    def test_custody_source_missing_field_is_not_derived_from_address(self):
        paxeer = self.root / 'paxeer'
        paxeer.mkdir()
        provision.write_json(paxeer / 'deployment.json', {'addresses': {}})
        with self.assertRaises(provision.Refused) as caught:
            provision.funding_evidence(self.root)
        self.assertIn('owner-registration.json', str(caught.exception))
        self.assertFalse((self.input / 'movement-source.json').exists())

    def run_cli(self):
        return subprocess.run(['python3', str(Path(provision.__file__).resolve()),
                               '--validate-owner-registration', '--work-dir', str(self.root)],
                              text=True, capture_output=True, check=False)

    def test_cli_refuses_with_exact_path_without_values(self):
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(str(self.path), result.stderr)
        self.assertEqual(result.stdout, '')


if __name__ == '__main__':
    unittest.main()
