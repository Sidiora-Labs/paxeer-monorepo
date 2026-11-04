#!/usr/bin/env bash
# paxeer-x-services: sequencer platform-receipt-authority
set -euo pipefail
umask 077
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
: "${LAYERX_GENERATION_SOURCE_SNAPSHOT:?isolated complete candidate snapshot required}"
snapshot=$LAYERX_GENERATION_SOURCE_SNAPSHOT
[ "$snapshot" != "$root" ] || [ "${LAYERX_GENERATION_IN_SNAPSHOT:-}" = 1 ] || { echo 'isolated candidate snapshot required' >&2; exit 1; }
if [ "${LAYERX_GENERATION_IN_SNAPSHOT:-}" != 1 ]; then
    export LAYERX_GENERATION_ORIGINAL_ROOT=$root
    export LAYERX_GENERATION_IN_SNAPSHOT=1
    exec bash "$snapshot/tools/paxeer-x/gates/24.20.sh" "${1:-verify}"
fi
cd "$snapshot"
: "${LAYERX_GENERATION_ARTIFACT_MANIFEST:?private generation artifact manifest required}"
case "${1:-verify}" in
build)
    python3 - <<'PREPARE'
import os, stat, subprocess
from pathlib import Path
root=Path.cwd()
assert root.is_absolute() and str(root)==os.environ['LAYERX_GENERATION_SOURCE_SNAPSHOT']
assert not any(p.is_symlink() for p in (root,*root.parents)), 'snapshot symlink refused'
info=root.stat()
assert info.st_uid==os.geteuid() and stat.S_IMODE(info.st_mode)==0o700, 'private owned snapshot required'
assert not subprocess.check_output(['git','status','--porcelain','--untracked-files=all']), 'complete clean candidate required'
for key in ('LAYERX_GENERATION_ARTIFACT_MANIFEST',):
 p=Path(os.environ[key]);assert p.is_absolute() and not p.exists()
 assert not any(item.is_symlink() for item in (p,*p.parents))
 assert p.parent.stat().st_uid==os.geteuid() and not p.parent.stat().st_mode&0o077
for name in ('paxd',):
 assert not (root/'build'/name).exists(), 'fresh source build required: '+name
PREPARE
    revision=$(git rev-parse HEAD)
    make -j2 LXP_REVISION="$revision" PAXEER_GO_JOBS=2 paxd-build
    python3 - <<'RECORD'
import hashlib,json,os,stat,subprocess,time
from pathlib import Path
root=Path.cwd()
paths=['services/platform/hosted/node/'+name for name in ('generation_transport.py','generation_client.py','native_runtime.py','reset_state.py','data_directory.py','supervisor.sh','bootstrap.sh','deployment.yaml','tests/test_generation_transport.py','tests/test_repair_22_31_355.py')]
paths += ['docker/kernel/'+name for name in ('init.sh','kernel_runtime.py','Dockerfile','Dockerfile.dockerignore')]
paths += ['tools/paxeer-x/gates/24.20.sh','tools/qualification/audit_repairs/test_repair_22_31_427.py']
def git(*args): return subprocess.check_output(['git',*args]).decode().strip()
def digest(p):
 with p.open('rb') as f: return hashlib.file_digest(f,'sha256').hexdigest()
assert not git('status','--porcelain','--untracked-files=all'), 'source changed during build'
for name in ('generation_transport.py','generation_client.py','native_runtime.py','reset_state.py','data_directory.py','tests/test_generation_transport.py','tests/test_repair_22_31_355.py'):
 p=root/'services/platform/hosted/node'/name;compile(p.read_bytes(),str(p),'exec')
for name in ('supervisor.sh','bootstrap.sh'):
 subprocess.run(['bash','-n',str(root/'services/platform/hosted/node'/name)],check=True)
subprocess.run(['bash','-n',str(root/'docker/kernel/init.sh')],check=True)
executables={}
for name in ('paxd',):
 p=root/'build'/name;info=p.lstat()
 assert stat.S_ISREG(info.st_mode) and info.st_uid==os.geteuid() and not info.st_mode&0o022 and os.access(p,os.X_OK)
 with p.open('rb') as f: assert f.read(4)==b'\x7fELF'
 executables[name]={'path':str(p),'sha256':digest(p),'mtime_ns':info.st_mtime_ns}
value={'version':1,'source_revision':git('rev-parse','HEAD'),'source_tree':git('rev-parse','HEAD^{tree}'),
 'native_build_source_binding':hashlib.sha256(subprocess.check_output(['git','ls-tree','-r','-z','--full-tree','HEAD'])).hexdigest(),
 'candidate_sources':{name:digest(root/name) for name in paths},'executables':executables,'recorded_ns':time.time_ns()}
p=Path(os.environ['LAYERX_GENERATION_ARTIFACT_MANIFEST'])
fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
with os.fdopen(fd,'w') as f: json.dump(value,f,sort_keys=True);f.write('\n');f.flush();os.fsync(f.fileno())
fd=os.open(p.parent,os.O_RDONLY|os.O_DIRECTORY)
try: os.fsync(fd)
finally: os.close(fd)
RECORD
    ;;
verify)
    python3 - <<'VERIFY'
import hashlib,json,os,stat,subprocess,sys,re
import pytest
from pathlib import Path
root=Path.cwd();p=Path(os.environ['LAYERX_GENERATION_ARTIFACT_MANIFEST']);info=p.lstat()
assert p.is_absolute() and not any(item.is_symlink() for item in (p,*p.parents))
assert stat.S_ISREG(info.st_mode) and info.st_uid==os.geteuid() and stat.S_IMODE(info.st_mode)==0o600 and info.st_nlink==1
value=json.loads(p.read_bytes())
def digest(p):
 with p.open('rb') as f: return hashlib.file_digest(f,'sha256').hexdigest()
def git(*args): return subprocess.check_output(['git',*args]).decode().strip()
assert not git('status','--porcelain','--untracked-files=all')
assert value['version']==1 and value['source_revision']==git('rev-parse','HEAD') and value['source_tree']==git('rev-parse','HEAD^{tree}')
assert value['native_build_source_binding']==hashlib.sha256(subprocess.check_output(['git','ls-tree','-r','-z','--full-tree','HEAD'])).hexdigest()
assert set(value['candidate_sources'])=={
 'services/platform/hosted/node/'+name for name in
 ('generation_transport.py','generation_client.py','native_runtime.py','reset_state.py','data_directory.py','supervisor.sh','bootstrap.sh','deployment.yaml','tests/test_generation_transport.py','tests/test_repair_22_31_355.py')
} | {'docker/kernel/'+name for name in ('init.sh','kernel_runtime.py','Dockerfile','Dockerfile.dockerignore')} | {
 'tools/paxeer-x/gates/24.20.sh','tools/qualification/audit_repairs/test_repair_22_31_427.py'
}
for name,expected in value['candidate_sources'].items(): assert digest(root/name)==expected,'candidate source mismatch: '+name
assert set(value['executables'])=={'paxd'}
for name,row in value['executables'].items():
 target=root/'build'/name
 assert row['path']==str(target) and digest(target)==row['sha256'] and target.stat().st_mtime_ns==row['mtime_ns']<=value['recorded_ns']
sys.dont_write_bytecode=True
sys.path.insert(0,str(root/'services/platform/hosted/node/tests'))

PYTHON_CORPORA = (
    'services/platform/hosted/node/tests/test_generation_transport.py',
    'tools/qualification/audit_repairs/test_repair_22_31_427.py::test_kernel_backend_cold_start_and_reopened_committed_readiness',
    'tools/qualification/audit_repairs/test_repair_22_31_427.py::test_kernel_backend_refuses_uncommitted_or_divergent_peers',
    'tools/qualification/audit_repairs/test_repair_22_31_427.py::test_kernel_backend_preserves_recovery_floor_on_refusal',
    'tools/qualification/audit_repairs/test_repair_22_31_427.py::test_kernel_backend_refuses_untrusted_recovery_record',
)


class ExecutedCases:
    def __init__(self):
        self.collected = []
        self.reports = []
        self.collection_failed = False
        self.deselected = False

    def pytest_collection_finish(self, session):
        self.collected = [item.nodeid for item in session.items]

    def pytest_collectreport(self, report):
        if not report.passed:
            self.collection_failed = True

    def pytest_deselected(self, items):
        self.deselected = True

    def pytest_runtest_logreport(self, report):
        self.reports.append(report)

    def count(self, exit_code):
        if (exit_code != 0 or self.collection_failed or self.deselected
                or not self.collected or len(self.collected) != len(set(self.collected))):
            raise ValueError('exact nonempty Python corpus required')
        expected = {(name, stage) for name in self.collected for stage in ('setup', 'call', 'teardown')}
        observed = [(row.nodeid, row.when) for row in self.reports]
        if (len(observed) != len(expected) or set(observed) != expected
                or any(not row.passed or hasattr(row, 'wasxfail') for row in self.reports)):
            raise ValueError('failed, skipped, xfailed or incomplete Python case refused')
        return len(self.collected)


def executed_counts(output, names):
    events = [json.loads(line) for line in output.splitlines() if line.strip()]
    if not events or events[-1].get('Action') != 'pass' or events[-1].get('Test'):
        raise ValueError('successful package completion required')
    starts = [row['Test'] for row in events if row.get('Test') and row.get('Action') == 'run']
    ends = [row['Test'] for row in events if row.get('Test') and row.get('Action') == 'pass']
    if any(row.get('Action') in ('skip', 'fail') for row in events):
        raise ValueError('failed or skipped Go case refused')
    if (not starts or len(starts) != len(set(starts)) or sorted(starts) != sorted(ends)
            or {name for name in starts if '/' not in name} != set(names)):
        raise ValueError('exact nonempty Go corpus required')
    running = set()
    completed = set()
    for row in events:
        name, action = row.get('Test'), row.get('Action')
        if not name or action not in ('run', 'pass'):
            continue
        if name.split('/')[0] not in names:
            raise ValueError('unrequested Go case refused')
        if action == 'run':
            if name in running or name in completed:
                raise ValueError('duplicate Go case refused')
            running.add(name)
        else:
            if name not in running:
                raise ValueError('Go completion without start refused')
            running.remove(name)
            completed.add(name)
    if running:
        raise ValueError('unfinished Go case refused')
    return len(completed)


cases = ExecutedCases()
code = pytest.main(['-q', '--tb=short', *PYTHON_CORPORA], plugins=[cases])
total = cases.count(code)
names = ('TestRealStateCreditAndClockBounds', 'TestRealStateCreditRefusesMissingAuthority')
pattern = '^(' + '|'.join(re.escape(name) for name in names) + ')$'
result = subprocess.run(['go','test','-json','-count=1','-run',pattern,'./custodyproof'],
                        stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=750)
print(result.stdout,end='',flush=True)
if result.returncode:
    raise SystemExit(result.returncode if result.returncode > 0 else 1)
total += executed_counts(result.stdout,names)
print(f'PAXEER_X_GATE tests={total} skipped=0',flush=True)
VERIFY
    ;;
*) echo 'usage: 24.20.sh build|verify' >&2; exit 2 ;;
esac
