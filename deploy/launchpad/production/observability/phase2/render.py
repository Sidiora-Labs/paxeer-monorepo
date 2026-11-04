from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import tempfile
from urllib.parse import urlsplit

from contracts import canonical, load, safe_diagnostics, sha, validate_binding
from contracts import require


def safe_directory(path):
    path = Path(path).absolute()
    for component in (path, *path.parents):
        require(not component.is_symlink(), 'configuration directory symlink denied')
    return path


def atomic_write(path, raw):
    path = safe_directory(Path(path).parent) / Path(path).name
    require(not path.is_symlink(), 'configuration file symlink denied')
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    descriptor, temporary = tempfile.mkstemp(prefix='.observability-', dir=path.parent)
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, 'wb') as output:
            output.write(raw)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def http_client(destination):
    result = {'authorization': {'type': 'Bearer', 'credentials_file': destination['token_file']}}
    if destination['ca_file'] is not None:
        result['tls_config'] = {'ca_file': destination['ca_file']}
    return result


def configuration_files(binding, config):
    root = Path(__file__).resolve().parents[1]
    rules = load(root/'prometheus-rules.json')
    # Ownership in the admitted inventory also governs the rendered rule labels.
    alerts = {alert['name']: alert for alert in config['alerts']}
    for group in rules['groups']:
        for rule in group['rules']:
            if 'alert' in rule:
                declared = alerts[rule['alert']]
                rule['labels'].update({k: declared[k] for k in ('owner', 'commander', 'severity')})
    scrape_configs = [{'job_name':s['service'], 'metrics_path':'/metrics', 'scheme':urlsplit(s['base_url']).scheme, 'static_configs':[{'targets':[urlsplit(s['base_url']).netloc]}]} for s in binding['services']]
    synthetic = binding['synthetic']
    exporter = urlsplit(synthetic['url'])
    scrape_configs.append({'job_name': 'synthetic-journeys', 'metrics_path': exporter.path,
                           'scheme': exporter.scheme, 'honor_labels': True,
                           'static_configs': [{'targets': [exporter.netloc]}],
                           **http_client(synthetic)})
    manager = binding['alertmanager']
    destination = urlsplit(manager['url'])
    prometheus = {'global': {'scrape_interval':'15s', 'evaluation_interval':'15s', 'external_labels':{'deployment_digest':binding['deployment_digest']}}, 'rule_files':['prometheus-rules.json'], 'scrape_configs':scrape_configs,
                  'alerting': {'alertmanagers': [{'scheme': destination.scheme,
                                                'path_prefix': destination.path or '/',
                                                'api_version': 'v2',
                                                'static_configs': [{'targets': [destination.netloc]}],
                                                **http_client(manager)}]}}
    routes, receivers = [], []
    for owner, target in sorted(manager['receivers'].items()):
        responder = binding['responders'][owner]
        name = owner + '.' + responder
        routes.append({'matchers': ['owner="' + owner + '"'], 'receiver': name})
        receivers.append({'name': name, 'webhook_configs': [
            {'url': target['url'], 'send_resolved': True, 'http_config': http_client(target)}]})
    alertmanager = {'route': {'receiver': receivers[0]['name'],
                             'group_by': ['alertname', 'journey', 'environment', 'zone', 'deployment_digest'],
                             'group_wait': '0s', 'group_interval': '30s', 'repeat_interval': '5m',
                             'routes': routes}, 'receivers': receivers}
    mounts = []
    def mount(component, path, read_only):
        value = {'component': component, 'path': path, 'read_only': read_only}
        if value not in mounts:
            mounts.append(value)
    mount('probes', synthetic['textfile_directory'], False)
    mount('synthetic-exporter', synthetic['textfile_directory'], True)
    for component, target, keys in (
        ('prometheus', synthetic, ('token_file', 'ca_file')),
        ('synthetic-exporter', synthetic, ('token_file', 'cert_file', 'key_file')),
        ('prometheus', manager, ('token_file', 'ca_file')),
    ):
        for key in keys:
            if target[key] is not None:
                mount(component, target[key], True)
    for target in manager['receivers'].values():
        for key in ('token_file', 'ca_file'):
            if target[key] is not None:
                mount('alertmanager', target[key], True)
    transport = {'version': '1.0', 'mounts': mounts,
                 'probes': {'output': synthetic['textfile_directory'], 'interval_seconds': 30},
                 'synthetic_exporter': {'configuration': 'synthetic-exporter.json'},
                 'alertmanager': {'configuration': 'alertmanager.json', 'url': manager['url'],
                                  'authentication': 'bound-bearer-gateway'},
                 'responders': {owner: binding['responders'][owner] for owner in manager['receivers']}}
    return {'prometheus.json':canonical(prometheus), 'prometheus-rules.json':canonical(rules),
            'alertmanager.json': canonical(alertmanager),
            'synthetic-exporter.json': canonical(synthetic),
            'transport.json': canonical(transport),
            'otel-collector.yaml':(root/'otel-collector.yaml').read_bytes(),
            'diagnostics.json':canonical(safe_diagnostics(binding))}


def render(binding, config, output):
    validate_binding(binding, config)
    output = safe_directory(output)
    files = configuration_files(binding, config)
    for name, raw in files.items():
        atomic_write(output/name, raw)
    manifest = {'version':'2.0', 'configuration_sha256':sha(canonical(config)), 'binding_sha256':sha(canonical(binding)), 'files':{name:sha(raw) for name,raw in files.items()}, 'secret_references':config['exporter'], 'activation_allowed':False}
    atomic_write(output/'render-manifest.json', canonical(manifest))
    return manifest


def verify_bundle(binding, config, target):
    validate_binding(binding,config)
    target = safe_directory(target)
    manifest = load(target/'render-manifest.json')
    require(manifest['binding_sha256'] == sha(canonical(binding)) and manifest['configuration_sha256'] == sha(canonical(config)), 'activation bundle identity differs')
    expected = set(configuration_files(binding, config))
    require(set(manifest['files']) == expected and {p.name for p in target.iterdir()} == expected | {'render-manifest.json'}, 'activation bundle inventory differs')
    for name, expected_hash in manifest['files'].items():
        file = target/name
        require(file.is_file() and not file.is_symlink() and sha(file.read_bytes()) == expected_hash, 'activation bundle changed')
    for name,raw in configuration_files(binding,config).items():
        require(manifest['files'][name] == sha(raw), 'activation configuration differs from bound source')
    require(load(target/'diagnostics.json') == safe_diagnostics(binding), 'activation diagnostics changed')
    require(manifest['secret_references'] == config['exporter'] and manifest['activation_allowed'] is False, 'activation bundle authority differs')
    return manifest


def immutable_bundle(binding, config, output):
    output = safe_directory(output)
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    target = output/('revision-'+str(config['revision'])+'-'+sha(canonical(binding)))
    if target.exists():
        return target, verify_bundle(binding, config, target)
    temporary = Path(tempfile.mkdtemp(prefix='.activation-', dir=output))
    try:
        render(binding, config, temporary)
        manifest = verify_bundle(binding, config, temporary)
        os.rename(temporary, target)
        directory = os.open(output, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
        return target, manifest
    finally:
        if temporary.exists():
            for file in temporary.iterdir():
                file.unlink()
            temporary.rmdir()


def main():
    p=argparse.ArgumentParser();p.add_argument('--binding', required=True);p.add_argument('--config', required=True);p.add_argument('--output', required=True);a=p.parse_args()
    print(json.dumps(render(load(a.binding),load(a.config),a.output)))


if __name__ == '__main__':
    main()
