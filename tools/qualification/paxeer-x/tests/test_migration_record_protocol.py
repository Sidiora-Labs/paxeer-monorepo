"""Exercise the production file-record protocol without running a native toolchain."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat

import pytest


ENTRY = Path(__file__).resolve().parents[1] / 'human-service-contract.py'


def load_contract():
    spec = importlib.util.spec_from_file_location('human_service_contract_22_33_853', ENTRY)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.fixture
def contract():
    return load_contract()


@pytest.fixture
def records(contract, tmp_path):
    """Publish source-file fixtures through the real JSON writer and private filesystem."""
    args = contract.parse_args(['--evidence-dir', str(tmp_path / 'operator evidence')])
    service, clock = contract.record_paths(args.evidence_dir, create=True)
    artifacts = {}
    # The proof covers artifact identity, not native compilation. These are verbatim
    # committed producer sources, never executed or presented as compiled binaries.
    sources = {
        'service': ENTRY,
        'boundary': contract.ROOT / 'services/human/crates/layerx-human-service/tests/server_browser_boundary.rs',
        'clock': contract.ROOT / 'tools/runtime/run-with-clock.sh',
    }
    for name, source in sources.items():
        artifact = service.parent / (name + '.recorded-source')
        shutil.copyfile(source, artifact)
        artifact.chmod(0o700)
        artifacts[name] = {'path': str(artifact), 'sha256': contract.digest(artifact)}
    contract.publish_service_record(service, contract.source_hashes(),
                                    {name: row['path'] for name, row in artifacts.items()
                                     if name in ('service', 'boundary')})
    first = contract.load_service_record(service)
    contract.publish_clock_record(clock, contract.clock_source_hashes(), first['artifacts'],
                                  artifacts['clock']['path'], contract.digest(service))
    second = contract.load_clock_record(service, clock)
    return service, clock, first, second


@pytest.mark.parametrize('mode', [[], ['--build'], ['--build-clock']])
def test_f033_operator_options_cover_every_mode(contract, tmp_path, mode):
    directory = tmp_path / 'selected evidence'
    target = tmp_path / 'selected target'
    cargo = str(tmp_path / 'selected toolchain' / 'cargo')
    args = contract.parse_args(mode + ['--evidence-dir', str(directory),
                                      '--cargo', cargo, '--target-dir', str(target)])
    assert args.evidence_dir == directory
    assert args.build == ('--build' in mode)
    assert args.build_clock == ('--build-clock' in mode)
    service, clock = contract.record_paths(args.evidence_dir, create=True)
    assert service == directory / 'task-104.13.24-artifacts.json'
    assert clock == directory / 'task-104.13.25-artifacts.json'
    assert stat.S_IMODE(directory.stat().st_mode) == 0o700
    command = contract.compiler_command(args.cargo, clock=args.build_clock)
    package = 'layerx-runtime-clock' if args.build_clock else 'layerx-human-service'
    workspace = 'platform' if args.build_clock else 'human'
    assert command == [cargo, 'build', '--locked', '--manifest-path',
                       str(contract.ROOT / 'services' / workspace / 'Cargo.toml'),
                       '-p', package, '--bin', package,
                       *([] if args.build_clock else ['--test', 'server_browser_boundary']),
                       '--message-format=json-render-diagnostics']
    assert contract.build_environment(args.target_dir)['CARGO_TARGET_DIR'] == str(target)


def test_f033_configured_toolchain_and_target_are_preserved(contract, monkeypatch, tmp_path):
    monkeypatch.setenv('CARGO', '/operator/toolchain/cargo')
    monkeypatch.setenv('CARGO_TARGET_DIR', str(tmp_path / 'environment target'))
    monkeypatch.setenv('CARGO_BUILD_JOBS', '2')
    args = contract.parse_args(['--evidence-dir', str(tmp_path)])
    assert args.cargo == '/operator/toolchain/cargo'
    for clock in (False, True):
        assert contract.compiler_command(args.cargo, clock=clock)[0] == args.cargo
    env = contract.build_environment(args.target_dir)
    assert env['CARGO_TARGET_DIR'] == os.environ['CARGO_TARGET_DIR']
    assert env['CARGO_BUILD_JOBS'] == '2'
    override = contract.parse_args(['--evidence-dir', str(tmp_path), '--cargo', 'chosen-cargo',
                                   '--target-dir', str(tmp_path / 'explicit target')])
    assert override.cargo == 'chosen-cargo'
    assert contract.build_environment(override.target_dir)['CARGO_TARGET_DIR'] == str(override.target_dir)
    assert os.environ['CARGO_TARGET_DIR'] == str(tmp_path / 'environment target')


def test_f033_unset_environment_uses_path_and_cargo_configuration(contract, monkeypatch, tmp_path):
    for key in ('CARGO', 'CARGO_TARGET_DIR', 'CARGO_BUILD_JOBS'):
        monkeypatch.delenv(key, raising=False)
    args = contract.parse_args(['--evidence-dir', str(tmp_path)])
    assert args.cargo == 'cargo'
    assert args.target_dir is None
    env = contract.build_environment(args.target_dir)
    assert 'CARGO_TARGET_DIR' not in env
    assert env['CARGO_BUILD_JOBS'] == '4'


def test_f033_cli_documents_bindings_and_requires_evidence(contract, capsys):
    with pytest.raises(SystemExit) as missing:
        contract.main([])
    assert missing.value.code == 2
    assert '--evidence-dir' in capsys.readouterr().err
    with pytest.raises(SystemExit) as help_result:
        contract.main(['--help'])
    assert help_result.value.code == 0
    help_text = capsys.readouterr().out
    for binding in ('--evidence-dir', '--cargo', 'CARGO', 'PATH', '--target-dir', 'CARGO_TARGET_DIR'):
        assert binding in help_text


def test_f033_modes_remain_exclusive(contract, tmp_path):
    with pytest.raises(SystemExit) as result:
        contract.parse_args(['--evidence-dir', str(tmp_path), '--build', '--build-clock'])
    assert result.value.code == 2


def test_f033_real_record_producer_transport_consumer_reopens_selected_directory(contract, records):
    service, clock, first, second = records
    assert service.parent.name == 'operator evidence'
    assert first['schema'] == 'paxeer-x.human-service-contract.v1'
    assert second['schema'] == 'paxeer-x.human-service-contract.v2'
    assert set(first['artifacts']) == {'service', 'boundary'}
    assert set(second['artifacts']) == {'service', 'boundary', 'clock'}
    assert json.loads(service.read_bytes()) == first
    assert json.loads(clock.read_bytes()) == second
    for path in (service, clock):
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
        assert path.stat().st_uid == os.geteuid()
    assert contract.load_service_record(service) == first
    assert contract.load_clock_record(service, clock) == second
    reopened = load_contract()
    selected_service, selected_clock = reopened.record_paths(service.parent)
    assert reopened.load_service_record(selected_service) == first
    assert reopened.load_clock_record(selected_service, selected_clock) == second
    assert second['predecessor_digest'] == reopened.digest(service)
    assert set(second['sources']) == set((*reopened.SOURCES, *reopened.CLOCK_SOURCES))


@pytest.mark.parametrize('mode', [0o755, 0o770, 0o707])
def test_f033_rejects_nonprivate_evidence_directory(contract, tmp_path, mode):
    directory = tmp_path / 'exposed'
    directory.mkdir()
    directory.chmod(mode)
    for create in (False, True):
        with pytest.raises(RuntimeError, match='private owned evidence directory required'):
            contract.record_paths(directory, create=create)
    assert stat.S_IMODE(directory.stat().st_mode) == mode
    assert list(directory.iterdir()) == []


def test_f033_refuses_symlink_directory_and_missing_verification_evidence(contract, tmp_path):
    private = tmp_path / 'private'
    private.mkdir(mode=0o700)
    link = tmp_path / 'linked'
    link.symlink_to(private, target_is_directory=True)
    with pytest.raises(RuntimeError, match='private owned evidence directory required'):
        contract.record_paths(link)
    missing = tmp_path / 'missing'
    with pytest.raises(FileNotFoundError):
        contract.main(['--evidence-dir', str(missing)])
    assert not missing.exists()


@pytest.mark.parametrize('which', ['service', 'clock'])
@pytest.mark.parametrize('damage', ['public', 'empty', 'oversize', 'symlink', 'directory'])
def test_f033_preserves_protected_record_checks(contract, records, which, damage):
    service, clock, _, _ = records
    path = service if which == 'service' else clock
    if damage == 'public':
        path.chmod(0o644)
    elif damage == 'empty':
        path.write_bytes(b'')
    elif damage == 'oversize':
        path.write_bytes(b' ' * 65537)
    else:
        original = path.with_suffix('.original')
        path.rename(original)
        if damage == 'symlink':
            path.symlink_to(original)
        else:
            path.mkdir(mode=0o700)
    with pytest.raises(RuntimeError, match='protected compiler artifact record required'):
        contract.protected_record(path)
    if which == 'service':
        with pytest.raises(RuntimeError, match='protected compiler artifact record required'):
            contract.load_service_record(service)
    else:
        with pytest.raises(RuntimeError, match='protected compiler artifact record required'):
            contract.load_clock_record(service, clock)


@pytest.mark.parametrize('which', ['service', 'clock'])
def test_f033_save_record_refuses_symlink_destination(contract, records, which):
    service, clock, first, second = records
    path, value = (service, first) if which == 'service' else (clock, second)
    original = path.with_suffix('.original')
    path.rename(original)
    before = original.read_bytes()
    path.symlink_to(original)
    with pytest.raises(OSError):
        contract.save_record(path, value)
    assert original.read_bytes() == before


@pytest.mark.parametrize('which', ['service', 'clock'])
@pytest.mark.parametrize('damage', ['schema', 'source', 'missing-source', 'extra-artifact',
                                    'missing-artifact', 'digest', 'relative', 'nonexecutable', 'missing-file'])
def test_f033_preserves_source_compiler_digest_and_closed_profile_checks(contract, records, which, damage):
    service, clock, first, second = records
    path, value = (service, first) if which == 'service' else (clock, second)
    if damage == 'schema':
        value['schema'] = 'unrecognised'
    elif damage == 'source':
        value['sources'][contract.SOURCES[0]] = '00' * 32
    elif damage == 'missing-source':
        del value['sources'][contract.SOURCES[0]]
    elif damage == 'extra-artifact':
        value['artifacts']['unexpected'] = dict(value['artifacts']['service'])
    elif damage == 'missing-artifact':
        del value['artifacts']['service']
    elif damage == 'digest':
        Path(value['artifacts']['service']['path']).write_bytes(b'changed artifact bytes')
    elif damage == 'relative':
        value['artifacts']['service']['path'] = 'relative-artifact'
    elif damage == 'nonexecutable':
        Path(value['artifacts']['service']['path']).chmod(0o600)
    else:
        Path(value['artifacts']['service']['path']).unlink()
    contract.save_record(path, value)
    with pytest.raises(RuntimeError):
        if which == 'service':
            contract.load_service_record(service)
        else:
            contract.load_clock_record(service, clock)


def test_f033_predecessor_digest_still_binds_selected_service_record(contract, records):
    service, clock, first, _ = records
    first['schema'] = 'changed predecessor'
    contract.save_record(service, first)
    with pytest.raises(RuntimeError, match='service predecessor record changed'):
        contract.load_clock_record(service, clock)


def test_f033_legacy_gate_digest_does_not_exempt_other_service_sources(contract, records):
    service, _, first, _ = records
    first['sources']['tools/qualification/paxeer-x/human-service-contract.py'] = contract.V1_GATE_DIGEST
    contract.save_record(service, first)
    assert contract.load_service_record(service) == first
    first['sources'][contract.SOURCES[0]] = '00' * 32
    contract.save_record(service, first)
    with pytest.raises(RuntimeError, match='prior service and boundary sources changed'):
        contract.load_service_record(service)
