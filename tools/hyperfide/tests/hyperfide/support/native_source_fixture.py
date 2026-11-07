"""Compile native source modules and run an actor on an owned temporary PostgreSQL server."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from urllib.parse import parse_qs, urlparse

ROOT = Path(__file__).resolve().parents[3]


def default_source_migrations():
    directory = ROOT / 'core/control-plane/migrations'
    series = [path for path in sorted(directory.glob('*source-control*.sql'))
              if re.match(r'^\d+-', path.name) and int(path.name.split('-', 1)[0]) >= 736]
    identifier = r'[a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?'
    dependencies = {}
    for path in series:
        sql = path.read_text()
        creates = set(re.findall(r'\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(' + identifier + r')', sql, re.I))
        references = set(re.findall(r'\bREFERENCES\s+(' + identifier + r')', sql, re.I))
        dependencies[path.name] = (creates, references)
    selected = list(dependencies)
    while True:
        tables = set().union(*(dependencies[name][0] for name in selected))
        eligible = [name for name in selected if dependencies[name][1] <= tables]
        if eligible == selected:
            return selected
        selected = eligible


def source_node_binary():
    override = os.environ.get('HYPERFIDE_NODE_BINARY')
    node = shutil.which(override or 'node')
    if not node:
        raise RuntimeError('CAPABILITY_MISSING: set HYPERFIDE_NODE_BINARY to an executable Node binary or install node on PATH')
    return node


def run_source_actor(actor_path, migrations=None, timeout=120, extra_files=(), actor_env=None):
    database = os.environ.get('DATABASE_URL', '')
    if not database:
        env = {k: v for k, v in os.environ.items() if k != 'DATABASE_URL'}
        return subprocess.run([sys.executable, str(ROOT / 'tests/hyperfide/support/withpg.py'), '--',
                               sys.executable, str(Path(sys.argv[0]).resolve()), *sys.argv[1:]],
                              cwd=ROOT, env=env).returncode
    host = parse_qs(urlparse(database).query).get('host', [''])[0]
    if not host.startswith('/tmp/hyperfide/hf-withpg-'):
        raise RuntimeError('Native source tests require an owned withpg socket')
    output = ROOT / 'build/source-control/actors'
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='native-source-', dir=output) as temporary:
        folder = Path(temporary)
        (folder / 'package.json').write_text('{"type":"module"}\n')
        (folder / 'node_modules').symlink_to(ROOT / 'core/control-plane/node_modules', target_is_directory=True)
        (folder / 'migrations').symlink_to(ROOT / 'core/control-plane/migrations', target_is_directory=True)
        modules = ROOT / 'core/control-plane/src/source_control'
        config = {'compilerOptions': {'target': 'ES2023', 'module': 'NodeNext', 'moduleResolution': 'NodeNext',
                  'rootDir': str(ROOT / 'core/control-plane/src'), 'outDir': str(folder / 'control'),
                  'strict': True, 'noUncheckedIndexedAccess': True, 'exactOptionalPropertyTypes': True,
                  'noEmitOnError': True, 'skipLibCheck': True, 'types': ['node'],
                  'typeRoots': [str(ROOT / 'core/control-plane/node_modules/@types')]},
                  'files': [str(path) for path in sorted(modules.rglob('*.ts'))] +
                           [str(ROOT / path) for path in extra_files]}
        config_path = folder / 'tsconfig.json'
        config_path.write_text(json.dumps(config))
        subprocess.run([str(ROOT / 'core/control-plane/node_modules/.bin/tsc'), '-p', str(config_path)],
                       cwd=ROOT, check=True)
        for source in (ROOT / 'core/control-plane/src').rglob('*.mjs'):
            destination = folder / 'control' / source.relative_to(ROOT / 'core/control-plane/src')
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, destination)
        subprocess.run([str(ROOT / 'core/control-plane/node_modules/.bin/tsc'), '-p',
                        str(ROOT / 'sdk/source-control/tsconfig.json'), '--outDir', str(folder / 'sdk')],
                       cwd=ROOT, check=True)
        selected_migrations = default_source_migrations() if migrations is None else list(migrations)
        if migrations is not None and '0737-source-control-metadata.sql' in selected_migrations and '0758-source-control-event-retention.sql' not in selected_migrations:
            selected_migrations.append('0758-source-control-event-retention.sql')
        if Path(actor_path).name == 'native_source_sparse_fixture.mjs':
            selected_migrations = sorted(set(selected_migrations) | set(default_source_migrations()))
        env = dict(os.environ, **(actor_env or {}), SOURCE_CONTROL_MODULE_ROOT=str(folder / 'control/source_control'),
                   SOURCE_CONTROL_SDK_MODULE_ROOT=str(folder / 'sdk'),
                   SOURCE_CONTROL_MIGRATIONS=json.dumps(selected_migrations))
        return subprocess.run([source_node_binary(), str(Path(actor_path).resolve()), str(folder / 'control/source_control')],
                              cwd=ROOT, env=env, timeout=timeout).returncode
