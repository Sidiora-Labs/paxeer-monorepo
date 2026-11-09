import copy
import importlib.util
import json
import os
from pathlib import Path
import sys
import time
import uuid


PRODUCER = Path(__file__).resolve().parents[1] / 'registry-artifacts.py'
spec = importlib.util.spec_from_file_location('registry_artifacts', PRODUCER)
artifacts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(artifacts)


def test_completed_job_history_survives_cleanup_reload(tmp_path):
    directory = tmp_path.resolve()
    directory.chmod(0o700)
    state = {'schema_version': artifacts.VERSION, 'id': uuid.uuid4().hex,
             'owner_uid': os.getuid(), 'directory': str(directory),
             'quota_root': str(directory / 'quota'), 'phase': 'prepared',
             'containers': [], 'jobs': []}
    artifacts.write_json(directory / 'lifecycle.json', state)
    expected_jobs = []
    script = '''import json, resource, sys
from pathlib import Path
print('completed-job-' + sys.argv[1] + '-stdout')
print('completed-job-' + sys.argv[1] + '-stderr', file=sys.stderr)
usage = resource.getrusage(resource.RUSAGE_SELF)
Path(sys.argv[2]).write_text(json.dumps({'user_time': usage.ru_utime,
                                      'system_time': usage.ru_stime,
                                      'max_rss': usage.ru_maxrss}))
sys.exit(int(sys.argv[1]))
'''
    for exit_code in (0, 7):
        observation_path = directory / f'observations-{exit_code}.json'
        command = [sys.executable, '-c', script, str(exit_code), str(observation_path)]
        started = time.monotonic()
        completed = artifacts.run(command, timeout=10, check=False)
        assert completed.returncode == exit_code
        observations = [json.loads(observation_path.read_text())]
        assert observations[0]['max_rss'] > 0
        with artifacts.locked(directory):
            state = artifacts.load_state(directory)
            prior_jobs = copy.deepcopy(state['jobs'])
            state['phase'] = 'running'
            result = artifacts.persist_completed_job(
                directory, state, exit_code=completed.returncode, logs=completed,
                started=started, observations=observations, interrupted=False,
                command=command)
            assert set(result) == {'exit_code', 'stdout', 'stderr', 'command',
                                   'elapsed', 'observations', 'interrupted'}
            assert result['exit_code'] == exit_code
            assert result['stdout'] == f'completed-job-{exit_code}-stdout\n'
            assert result['stderr'] == f'completed-job-{exit_code}-stderr\n'
            assert result['command'] == command
            assert 0 < result['elapsed'] < 10
            assert result['observations'] == observations
            assert result['interrupted'] is False
            expected_jobs.append(copy.deepcopy(result))
            durable = json.loads((directory / 'lifecycle.json').read_text())
            assert durable == state
            assert durable['jobs'] == expected_jobs
            assert durable['jobs'][:-1] == prior_jobs
            assert all(durable['jobs'].count(job) == 1 for job in expected_jobs)
            assert (directory / 'lifecycle.json').stat().st_mode & 0o077 == 0
            assert not list(directory.glob('lifecycle.json.new-*'))

            reloaded = artifacts.load_state(directory)
            assert reloaded == durable
            reloaded['phase'] = 'prepared'
            artifacts.write_json(directory / 'lifecycle.json', reloaded)
            reconciled = artifacts.load_state(directory)
            assert reconciled == {**durable, 'phase': 'prepared'}
            assert reconciled['jobs'] == expected_jobs
            assert reconciled['jobs'][:-1] == prior_jobs
            assert all(reconciled['jobs'].count(job) == 1 for job in expected_jobs)

    code = ("import importlib.util,json,sys; "
            "s=importlib.util.spec_from_file_location('registry_artifacts',sys.argv[1]); "
            "m=importlib.util.module_from_spec(s);s.loader.exec_module(m); "
            "print(json.dumps(m.load_state(sys.argv[2])))")
    recovered = artifacts.run([sys.executable, '-c', code, str(PRODUCER), str(directory)],
                              timeout=10)
    assert json.loads(recovered.stdout) == reconciled
    assert json.loads(recovered.stdout)['jobs'] == expected_jobs
