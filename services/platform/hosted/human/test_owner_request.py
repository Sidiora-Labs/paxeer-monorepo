import json
import os
import subprocess

import pytest

import provision
from test_provision import identity_provider_binary


def test_owner_email_must_be_operator_protected_file(tmp_path):
    secrets = tmp_path / 'secrets'
    secrets.mkdir(mode=0o700)
    (tmp_path / 'human-evidence-input').mkdir(mode=0o700)
    email = secrets / 'owner-email'
    with pytest.raises(provision.Refused, match='owner-email'):
        provision.owner_request(tmp_path, secrets)
    email.write_text('operator@example.invalid\n')
    email.chmod(0o644)
    with pytest.raises(provision.Refused, match='owner-email'):
        provision.owner_request(tmp_path, secrets)
    email.chmod(0o600)
    alias = secrets / 'linked-email'
    os.link(email, alias)
    with pytest.raises(provision.Refused, match='owner-email'):
        provision.owner_request(tmp_path, secrets)
    alias.unlink()
    email.unlink()
    email.symlink_to(alias)
    with pytest.raises(provision.Refused, match='owner-email'):
        provision.owner_request(tmp_path, secrets)


def test_generated_request_is_accepted_by_real_provider(tmp_path):
    secrets = tmp_path / 'secrets'
    secrets.mkdir(mode=0o700)
    inputs = tmp_path / 'human-evidence-input'
    inputs.mkdir(mode=0o700)
    email = secrets / 'owner-email'
    email.write_text('operator@example.invalid\n')
    email.chmod(0o600)
    provision.owner_request(tmp_path, secrets)
    request = inputs / 'owner-request.json'
    policy = inputs / 'recovery-policy.json'
    provision.write_json(policy, {'root': list(os.urandom(32)), 'threshold': 2, 'delay_seconds': 60})
    provision.job_input(tmp_path)
    binary = identity_provider_binary()
    result = subprocess.run([str(binary), 'provision-owner'], input=request.read_bytes(),
        capture_output=True, env=dict(os.environ,
            LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT=str(tmp_path / 'state'),
            LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE=str(policy)))
    assert result.returncode == 0, result.stderr.decode()
    owner = json.loads(result.stdout)
    expected = provision.protected_json(policy)
    assert set(owner) == {'principal', 'did', 'recovery_root', 'recovery_threshold',
                          'recovery_delay_seconds'}
    assert owner['principal'] and owner['did'] == 'did:layerx:' + owner['principal']
    assert owner['recovery_root'] == expected['root']
    assert owner['recovery_threshold'] == expected['threshold']
    assert owner['recovery_delay_seconds'] == expected['delay_seconds']
    output = tmp_path / 'human-owner-result.json'
    provision.write_json(output, owner)
    assert provision.owner_result(tmp_path, output) == owner
    replay = subprocess.run([str(binary), 'provision-owner'], input=request.read_bytes(),
        capture_output=True, env=dict(os.environ,
            LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT=str(tmp_path / 'state'),
            LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE=str(policy)))
    assert replay.returncode == 0, replay.stderr.decode()
    assert json.loads(replay.stdout) == owner
    assert request.stat().st_mode & 0o777 == 0o600
    with pytest.raises(provision.Refused):
        provision.owner_request(tmp_path, secrets)
