"""Exercise production bootstrap and protected-copy code without native binaries."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import sys

import pytest


ROOT = Path(__file__).resolve().parents[3]
ENTRYPOINT = ROOT / 'docker/human-service/entrypoint.sh'
BOOTSTRAP = ROOT / 'docker/human-service/journal_bootstrap.py'
RECORD = ROOT / 'tools/qualification/audit_repairs/repair_22_33_1534.kvx'


def load_bootstrap():
    spec = importlib.util.spec_from_file_location('human_journal_bootstrap', BOOTSTRAP)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def put(path, raw, mode=0o600):
    path.write_bytes(raw)
    path.chmod(mode)


def journal_fixture(tmp_path):
    seed = tmp_path / 'seed'
    private = tmp_path / 'private'
    seed.mkdir(mode=0o700)
    private.mkdir(mode=0o700)
    recorded = json.loads((ROOT / 'services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json').read_text())
    assert 'Unmodified signed deploy receipt' in recorded['provenance']
    # The bootstrap treats these committed producer bytes as opaque records.
    receipt = bytes.fromhex(recorded['receipt_hex'])
    prefix = hashlib.sha256(receipt).hexdigest()
    put(seed / (prefix + '.admission'), bytes.fromhex(recorded['header_hex']))
    put(seed / (prefix + '.deployment'), receipt)
    return seed, private, prefix


def snapshot(directory):
    return {p.name: (p.read_bytes(), p.stat().st_ino, p.stat().st_mtime_ns, stat.S_IMODE(p.stat().st_mode))
            for p in directory.iterdir()}


def heredoc(name):
    match = re.search(r"<<'" + name + r"'\n(.*?)\n" + name + r"\n", ENTRYPOINT.read_text(), re.S)
    assert match, name
    return compile(match[1], str(ENTRYPOINT) + ':' + name, 'exec')


def execute_copy(source, target):
    previous = sys.argv
    try:
        sys.argv = ['-', str(source), str(target)]
        exec(heredoc('PY_COPY_MATERIAL'), {'__name__': '__main__'})
    finally:
        sys.argv = previous


def test_finding_2_restart_preserves_advanced_journal(tmp_path):
    seed, private, prefix = journal_fixture(tmp_path)
    first = load_bootstrap()
    first.bootstrap_journal(seed, private)
    journal = private / 'journal'
    marker = json.loads((journal / first.MARKER).read_bytes())
    assert marker == {'version': 1, 'seeds': {
        p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in seed.iterdir()
    }}
    advanced = ROOT / 'programs/crates/layerx-programs-registry/tests/fixtures/maintained-head'
    acknowledged = (advanced / 'maintenance.receipt').read_bytes()
    assert acknowledged != (seed / (prefix + '.deployment')).read_bytes()
    put(journal / (prefix + '.deployment'), acknowledged)
    next_prefix = hashlib.sha256(acknowledged).hexdigest()
    put(journal / (next_prefix + '.admission'), (advanced / 'header').read_bytes())
    put(journal / (next_prefix + '.deployment'), acknowledged)
    before = snapshot(journal)
    # All bootstrap descriptors are closed. Reopen the disk state in a fresh module.
    del first
    reopened = load_bootstrap()
    reopened.bootstrap_journal(seed, private)
    assert snapshot(journal) == before
    assert (journal / (prefix + '.deployment')).read_bytes() == acknowledged
    assert stat.S_IMODE(journal.stat().st_mode) == 0o700
    assert not (private / '.journal-bootstrap').exists()


@pytest.mark.parametrize('change', ['bytes', 'added-pair', 'removed-pair'])
def test_finding_2_changed_seed_refused_without_mutation(tmp_path, change):
    seed, private, prefix = journal_fixture(tmp_path)
    module = load_bootstrap()
    module.bootstrap_journal(seed, private)
    before = snapshot(private / 'journal')
    if change == 'bytes':
        put(seed / (prefix + '.deployment'), b'conflicting fixture value')
    elif change == 'added-pair':
        for path in list(seed.iterdir()):
            put(seed / ('a' * 64 + path.suffix), path.read_bytes())
    else:
        for path in seed.iterdir():
            path.unlink()
    with pytest.raises(ValueError, match='conflict|count'):
        load_bootstrap().bootstrap_journal(seed, private)
    assert snapshot(private / 'journal') == before


def test_finding_2_adopts_identical_unmarked_records_and_preserves_extras(tmp_path):
    seed, private, _ = journal_fixture(tmp_path)
    journal = private / 'journal'
    journal.mkdir(mode=0o700)
    for path in seed.iterdir():
        put(journal / path.name, path.read_bytes())
    put(journal / 'acknowledged', (next(seed.iterdir())).read_bytes())
    before = snapshot(journal)
    module = load_bootstrap()
    module.bootstrap_journal(seed, private)
    assert {name: value for name, value in snapshot(journal).items() if name != module.MARKER} == before
    assert json.loads((journal / module.MARKER).read_text())['version'] == 1


@pytest.mark.parametrize('conflict', ['changed', 'missing'])
def test_finding_2_unmarked_conflicting_journal_is_preserved(tmp_path, conflict):
    seed, private, prefix = journal_fixture(tmp_path)
    journal = private / 'journal'
    journal.mkdir(mode=0o700)
    for path in seed.iterdir():
        put(journal / path.name, path.read_bytes())
    record = journal / (prefix + '.deployment')
    if conflict == 'changed':
        put(record, b'newer acknowledged data')
    else:
        record.unlink()
    before = snapshot(journal)
    with pytest.raises((ValueError, OSError)):
        load_bootstrap().bootstrap_journal(seed, private)
    assert snapshot(journal) == before
    assert not (journal / '.bootstrap.json').exists()


@pytest.mark.parametrize('fault', ['symlink', 'hardlink', 'empty', 'oversize', 'mode', 'missing-pair', 'bad-name'])
def test_finding_2_invalid_seeds_never_publish(tmp_path, fault):
    seed, private, prefix = journal_fixture(tmp_path)
    path = seed / (prefix + '.deployment')
    raw = path.read_bytes()
    if fault in ('symlink', 'hardlink'):
        retained = tmp_path / 'retained'
        put(retained, raw)
        path.unlink()
        if fault == 'symlink':
            path.symlink_to(retained)
        else:
            os.link(retained, path)
    elif fault == 'empty':
        put(path, b'')
    elif fault == 'oversize':
        put(path, b'x' * (load_bootstrap().MAX_RECORD + 1))
    elif fault == 'mode':
        path.chmod(0o644)
    elif fault == 'missing-pair':
        path.unlink()
    else:
        path.rename(seed / 'unexpected')
    with pytest.raises((OSError, ValueError)):
        load_bootstrap().bootstrap_journal(seed, private)
    assert not (private / 'journal').exists()
    assert not (private / '.journal-bootstrap').exists()


@pytest.mark.parametrize('fault', ['marker', 'record-symlink', 'directory-symlink', 'directory-mode'])
def test_finding_2_invalid_retained_state_refused(tmp_path, fault):
    seed, private, prefix = journal_fixture(tmp_path)
    module = load_bootstrap()
    module.bootstrap_journal(seed, private)
    journal = private / 'journal'
    if fault == 'marker':
        put(journal / module.MARKER, b'{"version":0}')
    elif fault == 'record-symlink':
        path = journal / (prefix + '.deployment')
        path.unlink()
        path.symlink_to(seed / path.name)
    elif fault == 'directory-symlink':
        journal.rename(private / 'retained')
        journal.symlink_to(private / 'retained', target_is_directory=True)
    else:
        journal.chmod(0o755)
    before = snapshot(journal)
    with pytest.raises((OSError, ValueError)):
        load_bootstrap().bootstrap_journal(seed, private)
    assert snapshot(journal) == before


def test_finding_2_interrupted_staging_requires_reconciliation(tmp_path):
    seed, private, _ = journal_fixture(tmp_path)
    staging = private / '.journal-bootstrap'
    staging.mkdir(mode=0o700)
    record = next(seed.iterdir())
    put(staging / record.name, record.read_bytes())
    before = snapshot(staging)
    with pytest.raises(FileExistsError):
        load_bootstrap().bootstrap_journal(seed, private)
    assert snapshot(staging) == before
    assert not (private / 'journal').exists()


def test_finding_1_security_startup_provisions_current_authority_without_retired_file(tmp_path):
    material = tmp_path / 'material'
    private = tmp_path / 'private'
    material.mkdir(mode=0o700)
    private.mkdir(mode=0o700)
    raw = bytes.fromhex((ROOT / 'programs/fuzz/corpus/trust-root/one-validator.hex').read_text())
    assert raw.startswith(b'LXTR\x01')
    put(material / 'consensus-authority', raw)
    # Execute the production copy body, with its actual source and destination argv.
    execute_copy(material / 'consensus-authority', private / 'consensus-authority')
    assert (private / 'consensus-authority').read_bytes() == raw
    assert stat.S_IMODE((private / 'consensus-authority').stat().st_mode) == 0o600
    assert not (material / 'trust-history').exists()
    assert not (private / 'trust-history').exists()
    security = ENTRYPOINT.read_text().split('    security)\n', 1)[1].split('    service)', 1)[0]
    assert 'trust-history' not in security
    assert 'while ' not in security and 'sleep ' not in security
    assert security.index('copy_material consensus-authority') < security.index('export LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY="$private/consensus-authority"')
    assert security.index('export LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY=') < security.index('exec /usr/local/bin/layerx-runtime-clock')
    assert '/usr/local/bin/layerx-human-security-provider "$@"' in security
    assert "(0, 4020, 0o440)" in security
    assert "path = '/run/human-material/consensus-authority'" in security


@pytest.mark.parametrize('fault', ['missing', 'symlink', 'empty', 'mode', 'destination-symlink'])
def test_finding_1_authority_copy_fails_closed(tmp_path, fault):
    source = tmp_path / 'consensus-authority'
    target = tmp_path / 'private-authority'
    put(source, b'protected authority')
    if fault == 'missing':
        source.unlink()
    elif fault == 'symlink':
        source.rename(tmp_path / 'retained')
        source.symlink_to(tmp_path / 'retained')
    elif fault == 'empty':
        put(source, b'')
    elif fault == 'mode':
        source.chmod(0o644)
    else:
        retained = tmp_path / 'retained'
        put(retained, b'retained authority')
        target.symlink_to(retained)
    with pytest.raises(SystemExit, match='Human protected material copy refused'):
        execute_copy(source, target)
    if fault == 'destination-symlink':
        assert retained.read_bytes() == b'retained authority'
    else:
        assert not target.exists()


def test_finding_1_receipt_keys_follow_consensus_proof_consumer():
    provider = ROOT / 'services/human/crates/layerx-human-security-provider/src'
    config = (provider / 'transport.rs').read_text()
    state = (provider / 'state.rs').read_text()
    recovery = (provider / 'recovery.rs').read_text()
    assert 'path("LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY")?' in config
    assert 'Store::open(&config.state_root, &config.trust_history)?' in config
    assert 'TrustRoot::decode(&bytes)' in state
    assert 'root.validators.validators().is_empty() || root.validators.total_power() == 0' in state
    assert 'chain_id: root.chain_id.clone()' in state
    assert 'validators: root.validators.clone()' in state
    assert '.verify_head(&header, &commit)' in state
    assert 'evidence.verify(&verifier, &head, &header, &receipt, protocol)?' in state
    for guard in ('key_proof.store_name != "sequencer" || key_proof.key != [3]',
                  'if key_header != *header',
                  'verify_state(head, "sequencer", &[3], keys, &key_proof.proof_ops)',
                  'SequencerAuthorization::new(key_id, public_key, from, to)',
                  'verify_sequencer_signature(receipt_bytes, pins.as_slice())'):
        assert guard in recovery


def test_shell_syntax_image_and_durable_publication_wiring():
    parsed = subprocess.run(['bash', '-n', str(ENTRYPOINT)], capture_output=True, text=True, check=False)
    assert parsed.returncode == 0, parsed.stderr
    entrypoint = ENTRYPOINT.read_text()
    agent = entrypoint.split('    agent)\n', 1)[1].split('    identity)', 1)[0]
    bootstrap = 'python3 /usr/local/lib/layerx-human/journal_bootstrap.py /run/human-material/journal "$private"'
    assert bootstrap in agent
    assert agent.index(bootstrap) < agent.index('exec /usr/local/bin/layerx-runtime-clock')
    assert 'install -m 0600' not in agent
    assert 'set -eu' in entrypoint and 'umask 077' in entrypoint
    docker = (ROOT / 'docker/human-service/Dockerfile').read_text()
    copies = [shlex.split(line) for line in docker.splitlines() if line.startswith('COPY ')]
    assert ['COPY', 'docker/human-service/journal_bootstrap.py', '/usr/local/lib/layerx-human/journal_bootstrap.py'] in copies
    assert ['COPY', '--chmod=0555', 'docker/human-service/entrypoint.sh', '/usr/local/bin/human-entrypoint'] in copies
    assert 'python3' in next(line for line in docker.splitlines() if line.startswith('RUN apt-get') and 'ca-certificates' in line)
    assert 'USER 4020:4020' in docker
    source = BOOTSTRAP.read_text()
    assert 'fcntl.flock(parent, fcntl.LOCK_EX)' in source
    assert 'os.O_EXCL | os.O_NOFOLLOW' in source
    assert 'os.fsync(stream.fileno())' in source
    assert source.index('create_record(journal, MARKER, manifest)') < source.index('os.rename(staging, target)')
    assert 'os.fsync(journal)' in source and 'os.fsync(parent)' in source
    consumer = (ROOT / 'services/agent/crates/layerx-agentd/src/main.rs').read_text()
    assert 'paths.retain(|path| path.extension().is_some_and(|value| value == "admission"))' in consumer


def test_repair_record_maps_findings_and_records_proof_limits():
    record = RECORD.read_text()
    for required in ('[finding.2]', '[finding.1]', 'test_finding_2_restart_preserves_advanced_journal',
                     'test_finding_1_security_startup_provisions_current_authority_without_retired_file',
                     'wiring_request', 'baseline', 'review_only', 'verified_revision'):
        assert required in record
    assert 'services/platform/hosted/human/history_material.py' in record
    assert 'services/human/crates/layerx-human-security-provider/src/transport.rs' in record
    assert '04c1e9407e75f7ac92c260452f2232ab07bf3ef1' in record
