import argparse
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
if 'ai_evidence' in sys.modules:
    evidence = sys.modules['ai_evidence']
else:
    spec = importlib.util.spec_from_file_location('ai_evidence', Path(__file__).parents[1] / 'ai_evidence.py')
    evidence = importlib.util.module_from_spec(spec)
    sys.modules['ai_evidence'] = evidence
    spec.loader.exec_module(evidence)

AUTHORITY = None


def authority_fixture(directory):
    root = Path(__file__).resolve().parents[3]

    def write(name, value):
        raw = json.dumps(value, sort_keys=True).encode()
        path = directory / name
        with path.open('xb') as stream:
            stream.write(raw)
        path.chmod(0o600)
        return str(path), hashlib.sha256(raw).hexdigest()

    def files(paths):
        return [{'path': name, 'sha256': hashlib.sha256((root / name).read_bytes()).hexdigest()}
                for name in paths]

    policy_path, policy_sha = write('policy.json', {
        'limits': evidence.LIMITS, 'read_scope': evidence.READ_SCOPE,
        'network': False, 'web': False, 'mcp': False, 'apps': False,
    })
    runtime_path, runtime_sha = write('runtime.json', {
        'runtime_scope_result': 'PASS', 'full_sandbox_pass': False,
        'infrastructure_probe_exit': 1,
        'infrastructure_probe_qualification': 'UNRUN_UNCHANGED',
    })
    log_path, log_sha = write('runtime.log', {'runtime_result_sha256': runtime_sha})
    owner = {
        'manager': 'px26-fm1', 'writer': 'isolated-cli-author-W-CORE-WORKSPACE',
        'codify_attempt': '3b57d71f2384b9f82b2612d78da0c854062402a195b3112775f14b2460f3e4d1',
        'fence': 2, 'paths': list(evidence.PATHS), 'exclusive': True,
    }
    contracts = files('spec/layerx-merge/contracts/' + name
                      for name in evidence.CONTRACT_PATHS)
    overlay = files(evidence.PATHS)
    admission = {
        **evidence.IDENTITY, 'schema': 'paxai.admission.v1',
        'selector': evidence.SELECTORS[0], 'issued_at_unix': 1791454712,
        'candidate': {'checkout': str(root),
                      'base_revision': 'ce269bfe23c1db984a8ae62ce13aa36c238ce2fd',
                      'branch': 'px26/fm1/ai-workspace'},
        'contract': {'version': 1, 'files': contracts},
        'toolchain': {
            'python': {'executable': sys.executable, 'version': '3.12.3'},
            'rustc': {'executable': '/usr/local/bin/rustc', 'version': '1.91.1',
                      'release_identity': 'ed61e7d7e 2025-11-07'},
            'cargo': {'executable': '/usr/local/bin/cargo', 'version': '1.91.1',
                      'release_identity': 'ea2d97820 2025-10-10'},
            'installed_targets': ['wasm32-unknown-unknown', 'x86_64-unknown-linux-gnu'],
        },
        'dependencies': {
            'validator': 'python-standard-library-only', 'new_dependencies': [],
            'programs_lock_sha256': 'f20c66cb7d488269d52a8d5a14a99f2fc23ec1b57dfbe697497a2d7744d67354',
            'programs_lock_evidence': 'Root verified actual Programs lock SHA256 before dispatch; author may not read unapproved lockfile.',
        },
        'ownership': owner,
        'access': {
            'policy_path': policy_path, 'policy_sha256': policy_sha,
            'runtime_result_path': runtime_path, 'runtime_result_sha256': runtime_sha,
            'runtime_scope_result': 'PASS', 'infrastructure_probe_exit': 1,
            'infrastructure_probe_qualification': 'UNRUN', 'full_sandbox_pass': False,
            'limits': evidence.LIMITS, 'author_read_scope': evidence.READ_SCOPE,
            'author_write_scope': 'exclusive private workspace-output subtree only; controller materializes exact two product files',
            'allowed_tools': ['shell for approved reads/private output', 'apply_patch within private workspace-output'],
            'network': False, 'web': False, 'mcp': False, 'apps': False,
            'ignore_user_config': True, 'ignore_rules': True,
            'project_doc_max_bytes': 0, 'ephemeral': True,
        },
        'evidence': {
            'directory': str(directory), 'approval_path': str(directory / 'approval.json'),
            'runtime_log_path': log_path, 'runtime_log_sha256': log_sha,
            'gate_log_path': str(directory / 'gate.log'),
            'gate_log_status': 'not-yet-run-root-Codify-owns-gate',
        },
        'model': {'requested': 'gpt-6.1-sol',
                  'backend_identity': 'not-independently-emitted-by-CLI-events',
                  'runtime_probe_exit': 0},
        'authorization': {
            'metadata_authority': 'Root explicitly delegated private actual workspace manifests from supplied verified facts; no runtime/deployment success may be invented.',
            'product_authority': 'Only isolated CLI authors the two exact files; root owns canonical qualification/publication.',
        },
        'overlay': {'status': 'READY', 'files': overlay}, 'other_selector_records': {},
    }
    admission_path, admission_sha = write('admission.json', admission)
    approval = {
        **evidence.IDENTITY, 'schema': 'paxai.approval.v1',
        'selector': evidence.SELECTORS[0], 'admission_path': admission_path,
        'admission_sha256': admission_sha,
        'base_revision': admission['candidate']['base_revision'],
        'branch': admission['candidate']['branch'], 'contract_version': 1,
        'contract_files': copy.deepcopy(contracts), 'ownership': copy.deepcopy(owner),
        'policy_sha256': policy_sha, 'runtime_result_sha256': runtime_sha,
        'runtime_log_sha256': log_sha, 'evidence_directory': str(directory),
        'overlay_files': copy.deepcopy(overlay), 'status': 'READY',
    }
    write('approval.json', approval)
    return admission, approval


class EvidenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture = tempfile.TemporaryDirectory(prefix='paxai-structure-', dir=Path.home())
        cls.addClassCleanup(cls.fixture.cleanup)
        cls.authority = (AUTHORITY if AUTHORITY is not None else
                         authority_fixture(Path(cls.fixture.name)))

    def setUp(self):
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.temporary = tempfile.TemporaryDirectory(
            prefix='paxai-negative-', dir=self.admission['evidence']['directory'])
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)

    def write(self, name, raw, mode=0o600):
        path = self.directory / name
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(raw)
        return path

    def refuse_structure(self):
        with self.assertRaises(evidence.Invalid):
            evidence.validate_structure(self.admission, self.approval)

    def test_actual_approved_authority(self):
        contracts, overlay = evidence.validate_structure(*self.authority)
        self.assertEqual(set(contracts), {
            'spec/layerx-merge/contracts/' + name for name in evidence.CONTRACT_PATHS})
        self.assertEqual(len(contracts), 4)
        root = Path(self.authority[0]['candidate']['checkout'])
        for name, digest in contracts.items():
            self.assertEqual(hashlib.sha256((root / name).read_bytes()).hexdigest(), digest)
        self.assertEqual(set(overlay), set(evidence.PATHS))
        self.assertEqual(self.authority[0]['overlay']['status'], 'READY')
        self.assertFalse(self.authority[0]['access']['full_sandbox_pass'])
        self.assertEqual(self.authority[0]['access']['infrastructure_probe_qualification'], 'UNRUN')

    def test_selector_rejection(self):
        for selector in ('', 'workspace', 'unknown', ' workspace-admission',
                         'workspace-admission ', 'WORKSPACE-ADMISSION', None, {}, 1):
            with self.subTest(selector=selector), self.assertRaises(evidence.Invalid):
                evidence.validate_selector(selector)

    def test_cli_argument_rejection(self):
        for args in ([], ['--selector', ''], ['--selector', 'unknown'],
                     ['--selector', 'workspace-admission', '--extra'],
                     ['--select', 'workspace-admission']):
            parser = evidence.Parser(add_help=False, allow_abbrev=False)
            parser.add_argument('--selector', required=True, choices=evidence.SELECTORS)
            with self.assertRaises(evidence.Invalid):
                parser.parse_args(args)

    def test_missing_files(self):
        with self.assertRaises(evidence.Invalid):
            evidence.load_authority(str(self.directory / 'absent'), str(self.directory / 'approval'))

    def test_duplicate_and_nonobject_json(self):
        for raw in (b'{"schema":1,"schema":2}', b'{"x":{"y":1,"y":2}}',
                    b'{} trailing', b'[]', b'null', b'', b'{"x":NaN}', b'\xff'):
            with self.subTest(raw=raw), self.assertRaises(evidence.Invalid):
                evidence.parse(raw)

    def test_missing_unknown_fields(self):
        for which in ('admission', 'approval'):
            for change in ('missing', 'unknown'):
                self.admission, self.approval = copy.deepcopy(self.authority)
                record = getattr(self, which)
                if change == 'missing':
                    del record['schema']
                else:
                    record['unexpected'] = True
                self.refuse_structure()
        for section in ('candidate', 'contract', 'toolchain', 'dependencies', 'ownership',
                        'access', 'evidence', 'model', 'authorization', 'overlay'):
            self.admission, self.approval = copy.deepcopy(self.authority)
            self.admission[section]['unexpected'] = True
            self.refuse_structure()

    def test_strict_types(self):
        cases = (('issued_at_unix', None, True), ('ownership', 'fence', True),
                 ('ownership', 'exclusive', 1), ('access', 'network', 0),
                 ('access', 'full_sandbox_pass', 0), ('access', 'project_doc_max_bytes', False),
                 ('contract', 'version', True), ('model', 'runtime_probe_exit', False))
        for section, key, value in cases:
            self.admission, self.approval = copy.deepcopy(self.authority)
            if key is None:
                self.admission[section] = value
            else:
                self.admission[section][key] = value
            self.refuse_structure()

    def test_identity_and_stale_base(self):
        cases = (('task_id', None, 'ai.P-OTHER'), ('decision_id', None, 'other'),
                 ('selector', None, 'deployment-admission'),
                 ('candidate', 'base_revision', '0' * 40),
                 ('candidate', 'branch', 'main'), ('contract', 'version', 2))
        for section, key, value in cases:
            self.admission, self.approval = copy.deepcopy(self.authority)
            if key is None:
                self.admission[section] = value
            else:
                self.admission[section][key] = value
            self.refuse_structure()

    def test_contract_hash_and_coverage(self):
        self.admission['contract']['files'][0]['sha256'] = '0' * 64
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['contract']['files'].pop()
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['contract']['files'][0]['path'] = 'spec.kvx'
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['contract']['files'][0] = self.admission['contract']['files'][1]
        self.refuse_structure()

    def test_changed_bytes_and_pin(self):
        raw = evidence.read_bytes(self.approval['admission_path'])
        path = self.write('copied-admission.json', raw)
        pinned = hashlib.sha256(raw).hexdigest()
        self.assertEqual(evidence.pinned_file(str(path), pinned), raw)
        with path.open('ab') as stream:
            stream.write(b' ')
        with self.assertRaises(evidence.Invalid):
            evidence.pinned_file(str(path), pinned)
        self.approval['admission_sha256'] = '0' * 64
        with self.assertRaisesRegex(evidence.Invalid, '^admission bytes pin: mismatch$'):
            evidence.validate_workspace(self.admission, self.approval,
                self.approval['admission_path'],
                self.admission['evidence']['approval_path'], raw)

    def test_ownership_and_fence(self):
        for name, value in (('manager', 'other'), ('writer', 'native-controller'),
                            ('codify_attempt', '0' * 64), ('fence', 3), ('exclusive', False)):
            self.admission, self.approval = copy.deepcopy(self.authority)
            self.admission['ownership'][name] = value
            self.refuse_structure()

    def test_sandbox_and_parent_limits(self):
        for name, value in (('runtime_scope_result', 'FAIL'), ('full_sandbox_pass', True),
                            ('infrastructure_probe_exit', 0),
                            ('infrastructure_probe_qualification', 'PASS'),
                            ('limits', []), ('allowed_tools', ['all']), ('network', True),
                            ('ignore_user_config', False)):
            self.admission, self.approval = copy.deepcopy(self.authority)
            self.admission['access'][name] = value
            self.refuse_structure()

    def test_evidence_and_log_identity(self):
        self.admission['evidence']['runtime_log_sha256'] = '0' * 64
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['access']['policy_sha256'] = '0' * 64
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['access']['runtime_result_sha256'] = '0' * 64
        self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['evidence']['runtime_log_path'] = '/outside/runtime.log'
        self.refuse_structure()

    def test_overlay_pending_partial_wrong_paths(self):
        for value in ('PENDING_COMPLETE_AUTHOR_OUTPUT', 'PASS', '', True):
            self.admission, self.approval = copy.deepcopy(self.authority)
            self.admission['overlay']['status'] = value
            self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['overlay']['files'].pop()
        self.refuse_structure()
        for value in ('tools/paxeer-x/other.py', '../ai_evidence.py', '/ai_evidence.py'):
            self.admission, self.approval = copy.deepcopy(self.authority)
            self.admission['overlay']['files'][0]['path'] = value
            self.refuse_structure()
        self.admission, self.approval = copy.deepcopy(self.authority)
        self.admission['overlay']['files'][0]['sha256'] = '0' * 64
        self.refuse_structure()

    def test_symlink_public_hardlink_files(self):
        actual = self.write('real.json', b'{}')
        symlink = self.directory / 'link.json'
        symlink.symlink_to(actual)
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(symlink))
        public = self.write('public.json', b'{}', 0o644)
        os.chmod(public, 0o644)
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(public))
        hard = self.directory / 'hard.json'
        os.link(actual, hard)
        for path in (actual, hard):
            with self.assertRaises(evidence.Invalid):
                evidence.read_bytes(str(path))
        link_directory = self.directory / 'link-directory'
        link_directory.symlink_to(self.directory, target_is_directory=True)
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(link_directory / 'public.json'))

    def test_nonregular_and_ownership_files(self):
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(self.directory))
        fifo = self.directory / 'fifo'
        os.mkfifo(fifo, 0o600)
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(fifo))
        if os.geteuid() == 0:
            path = self.write('foreign-owner', b'{}')
            os.chown(path, 65534, 65534)
            try:
                with self.assertRaises(evidence.Invalid):
                    evidence.read_bytes(str(path))
            finally:
                os.chown(path, 0, 0)
        else:
            with self.assertRaises(evidence.Invalid):
                evidence.read_bytes('/dev/null')

    def test_secret_escape_and_public_directory(self):
        for value in ('relative.json', str(self.directory) + '/../file',
                      str(self.directory / '.env'), str(self.directory / '.env.local'),
                      str(self.directory / '.ssh' / 'id_rsa')):
            with self.assertRaises(evidence.Invalid):
                evidence.safe_path(value)
        with self.assertRaises(evidence.Invalid):
            evidence.inside('/outside/file', self.directory)
        os.chmod(self.directory, 0o755)
        with self.assertRaises(evidence.Invalid):
            evidence.private_directory(str(self.directory))
        os.chmod(self.directory, 0o700)

    def test_real_contract_overlay_runtime_file_tampering(self):
        sources = [(Path(self.admission['candidate']['checkout']) / row['path'], row['sha256'], False)
                   for row in self.admission['overlay']['files']]
        sources.extend((Path(self.admission['access'][path]), self.admission['access'][sha], True)
                       for path, sha in (('policy_path', 'policy_sha256'),
                                         ('runtime_result_path', 'runtime_result_sha256')))
        common = evidence.git(self.admission['candidate']['checkout'], 'rev-parse',
                              '--path-format=absolute', '--git-common-dir')
        row = self.admission['contract']['files'][0]
        sources.append((Path(common).parent / row['path'], row['sha256'], False))
        for index, (source, expected, private) in enumerate(sources):
            raw = evidence.read_bytes(str(source), private=private)
            copied = self.write('tampered-%d' % index, raw + b'\n')
            with self.assertRaises(evidence.Invalid):
                evidence.pinned_file(str(copied), expected)

    def test_future_selector_missing_or_empty_evidence(self):
        for selector in evidence.SELECTORS[1:]:
            self.admission['other_selector_records'] = {}
            with self.assertRaises(evidence.Invalid):
                evidence.validate_future(selector, self.admission)
            self.admission['other_selector_records'] = {selector: {}}
            with self.assertRaises(evidence.Invalid):
                evidence.validate_future(selector, self.admission)
            path = self.write(selector + '.json', b'{}')
            self.admission['other_selector_records'] = {selector: {
                'path': str(path), 'sha256': hashlib.sha256(b'{}').hexdigest()}}
            with self.assertRaises(evidence.Invalid):
                evidence.validate_future(selector, self.admission)

    def test_bounds_and_hash_types(self):
        for value in (True, -1, 2**63, '1', 1.5):
            with self.assertRaises(evidence.Invalid):
                evidence.integer(value, 'test')
        for value in ('', '0' * 63, 'A' * 64, 1, None):
            with self.assertRaises(evidence.Invalid):
                evidence.digest(value)
        oversized = self.write('oversized', b'x' * (evidence.MAX_BYTES + 1))
        with self.assertRaises(evidence.Invalid):
            evidence.read_bytes(str(oversized))

    def test_future_fact_refusal(self):
        for selector in evidence.SELECTORS[1:4]:
            name = evidence.FUTURE_FIELDS[selector][0]
            with self.assertRaises(evidence.Invalid):
                evidence.validate_facts(selector, {name: {}}, {})
        for value in (True, 'PASS', 0):
            with self.assertRaises(evidence.Invalid):
                evidence.fact_value(value, 'positive')
        for value in (['same', 'same'], [], ['one']):
            with self.assertRaises(evidence.Invalid):
                evidence.fact_value(value, 'operators')

    def test_safe_refusal_labels(self):
        self.assertEqual(evidence.refusal_reason(evidence.Invalid('file SHA256 mismatch')),
                         'file-sha256-pin')
        self.assertEqual(evidence.refusal_reason(evidence.Invalid('contract: coverage')),
                         'contract-coverage')
        self.assertEqual(evidence.refusal_reason(evidence.Invalid('private-record-value\n')),
                         'invalid-evidence')
        self.assertEqual(evidence.refusal_reason(OSError('private-host-path')),
                         'filesystem-error')
        self.assertEqual(evidence.refusal_reason(ValueError('private-record-value')),
                         'value-error')
        self.assertEqual(evidence.refusal_reason(evidence.Invalid({'private': 'value'})),
                         'invalid-evidence')

    def test_runtime_producer_control_binding(self):
        # Component fixture only; the workspace gate still consumes pinned real bytes.
        runtime = {'runtime_scope_result': 'PASS', 'full_sandbox_pass': False,
                   'infrastructure_probe_exit': 1,
                   'infrastructure_probe_qualification': 'UNRUN_UNCHANGED'}
        evidence.validate_runtime(runtime)
        for name in runtime:
            missing = dict(runtime)
            del missing[name]
            with self.subTest(missing=name), self.assertRaises(evidence.Invalid):
                evidence.validate_runtime(missing)
        for name, value in (('runtime_scope_result', 'FAIL'),
                            ('full_sandbox_pass', True), ('full_sandbox_pass', 0),
                            ('infrastructure_probe_exit', 0),
                            ('infrastructure_probe_exit', True),
                            ('infrastructure_probe_qualification', 'UNRUN'),
                            ('infrastructure_probe_qualification', 'PASS'),
                            ('infrastructure_probe_qualification', 'unknown')):
            changed = dict(runtime)
            changed[name] = value
            with self.subTest(field=name), self.assertRaises(evidence.Invalid):
                evidence.validate_runtime(changed)
        for value in ({}, [], None):
            with self.assertRaises(evidence.Invalid):
                evidence.validate_runtime(value)

    def test_safe_testcase_failure_identities(self):
        method = 'test_safe_testcase_failure_identities'
        case = EvidenceTests(method)
        result = evidence.SafeTestResult(EvidenceTests, (method,))
        failure = (AssertionError, AssertionError('private-assertion-value'), None)
        error = (OSError, OSError('private-host-path'), None)
        result.addFailure(case, failure)
        result.addError(case, error)
        # Parameters and fixture descriptions must never be rendered.
        subtest = unittest.TestCase()
        result.addSubTest(case, subtest, failure)
        result.addError(unittest.TestCase(), error)
        result.addSkip(case, 'private-skip-reason')
        result.addUnexpectedSuccess(case)
        identity = 'EvidenceTests.' + method
        self.assertIn((identity, 'failure', 'test-exception'), result.diagnostics)
        self.assertIn((identity, 'error', 'filesystem-error'), result.diagnostics)
        self.assertIn((identity, 'subtest-failure', 'test-exception'), result.diagnostics)
        self.assertIn(('fixture-or-loader', 'error', 'filesystem-error'), result.diagnostics)
        self.assertIn((identity, 'skip', 'skip'), result.diagnostics)
        self.assertIn((identity, 'unexpected-success', 'unexpected-success'), result.diagnostics)
        self.assertEqual(result.failures[0][1], 'test-exception')
        self.assertEqual(result.errors[0][1], 'filesystem-error')
        self.assertEqual(result.skipped[0][1], 'skip')
        self.assertFalse(result.wasSuccessful())


class WorkspaceRefusalTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='paxai-workspace-', dir=Path.home())
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.admission, self.approval = authority_fixture(self.directory)

    def validate(self):
        raw = json.dumps(self.admission, sort_keys=True).encode()
        self.approval['admission_sha256'] = hashlib.sha256(raw).hexdigest()
        return evidence.validate_workspace(
            self.admission, self.approval, self.approval['admission_path'],
            self.admission['evidence']['approval_path'], raw)

    def test_current_checkout_refuses_stale_authority(self):
        with self.assertRaisesRegex(evidence.Invalid, '^stale Git HEAD$'):
            self.validate()
        with self.assertRaisesRegex(evidence.Invalid, '^stale Git HEAD$'):
            evidence.load_authority(self.approval['admission_path'],
                                    self.admission['evidence']['approval_path'])

    def test_historical_checkout_enforces_interpreter_and_validator_path(self):
        root = Path(self.admission['candidate']['checkout'])
        checkout = self.directory / 'checkout'
        commands = [
            ['git', 'clone', '--shared', '--no-checkout', str(root), str(checkout)],
            ['git', '-C', str(checkout), 'sparse-checkout', 'set', 'spec/layerx-merge/contracts'],
            ['git', '-C', str(checkout), 'switch', '--create',
             self.admission['candidate']['branch'], self.admission['candidate']['base_revision']],
        ]
        for command in commands:
            result = subprocess.run(command, capture_output=True, text=True, timeout=60)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for row in self.admission['contract']['files'] + self.admission['overlay']['files']:
            destination = checkout / row['path']
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(root / row['path'], destination)
        self.admission['candidate']['checkout'] = str(checkout)
        reason = ('actual Python must be 3.12.3' if sys.version_info[:3] != (3, 12, 3)
                  else 'validator must execute materialized owned path')
        with self.assertRaisesRegex(evidence.Invalid, '^' + reason + '$'):
            self.validate()


class EvidencePathContractTests(unittest.TestCase):
    def test_paths_are_canonical_strings(self):
        path = Path(__file__).resolve()
        self.assertEqual(evidence.safe_path(str(path)), path)
        with self.assertRaises(evidence.Invalid):
            evidence.safe_path(path)
        with self.assertRaises(evidence.Invalid):
            evidence.load_authority(path, str(path))


if __name__ == '__main__':
    unittest.main()
