#!/usr/bin/env python3
import json
import os
from pathlib import Path
import re
import secrets
import socket
import ssl
import stat
import subprocess
import sys
import urllib.error
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        return None


def regular(path, private=False):
    info = path.lstat()
    if (not path.is_absolute() or path.resolve() != path or not stat.S_ISREG(info.st_mode)
            or info.st_nlink != 1 or info.st_uid != os.geteuid()
            or info.st_size == 0 or info.st_size > 65536
            or (private and stat.S_IMODE(info.st_mode) != 0o600)):
        raise ValueError('Production browser input ownership, type or bounds refused')
    return path.read_bytes()


def run(*arguments):
    subprocess.run(arguments, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)


def decode_ramp_response(status, raw, path):
    """Decode the reference ramp contract, keeping refusals distinct from success."""
    if len(raw) > 1048576:
        raise ValueError('Ramp response exceeds the decoding bound')
    document = json.loads(raw)
    if not isinstance(document, dict):
        raise ValueError('Ramp response must be an object')
    if status not in (200, 201, 202):
        if (set(document) != {'error'} or not isinstance(document['error'], str)
                or not document['error']):
            raise ValueError('Malformed ramp refusal')
        return document
    if path == '/readyz':
        journal = document.get('journal')
        if (status != 200 or document.get('ready') is not True
                or document.get('readiness_scope') != 'local-journal'
                or document.get('external_custody') is not True
                or type(document.get('chain_id')) is not int
                or document['chain_id'] <= 0
                or not isinstance(journal, dict) or journal.get('ready') is not True):
            raise ValueError('Ramp local journal is not ready')
    elif path == '/internal/v1/work':
        if (not isinstance(document.get('stage'), str)
                or not isinstance(document.get('presentation'), dict)):
            raise ValueError('Malformed ramp work result')
    elif path.startswith('/internal/v1/rebalances'):
        if (document.get('settlement_domain') != 'paxeer'
                or document.get('status') not in ('submission_planned', 'submitted', 'receipted', 'committed')
                or not isinstance(document.get('idempotency_key'), list)
                or len(document['idempotency_key']) != 32
                or any(type(byte) is not int or not 0 <= byte <= 255
                       for byte in document['idempotency_key'])):
            raise ValueError('Malformed ramp rebalance result')
    else:
        raise ValueError('Unknown ramp response route')
    return document


def ramp_request(opener, service, path='/readyz', body=None, token=None):
    """Read fresh HTTP answers; never retain a readiness or operation result."""
    if path not in ('/readyz', '/internal/v1/work', '/internal/v1/rebalances') and not re.fullmatch(
            r'/internal/v1/rebalances/[0-9a-f]{64}', path):
        raise ValueError('Unknown ramp request route')
    headers = {'Accept': 'application/json', 'Cache-Control': 'no-cache'}
    if token is not None:
        headers['Authorization'] = 'Bearer ' + token
    data = None if body is None else json.dumps(body).encode()
    if data is not None:
        headers['Content-Type'] = 'application/json'
    request = urllib.request.Request(service + path, data=data, headers=headers)
    try:
        response = opener.open(request, timeout=10)
    except urllib.error.HTTPError as refusal:
        response = refusal
    with response:
        if response.geturl() != service + path:
            raise ValueError('Ramp redirect refused')
        raw = response.read(1048577)
        # A 503 readiness answer carries health rather than an error envelope.
        if path == '/readyz' and response.status != 200:
            raise ValueError('Ramp readiness refused')
        return response.status, decode_ramp_response(response.status, raw, path)


def prepare(repo, output):
    cluster = repo / 'build/beta-cluster'
    origin = regular(cluster / 'secrets/human/config/LAYERX_HUMAN_ORIGIN', True).decode()
    match = re.fullmatch(r'https://([a-z0-9]+(?:[.-][a-z0-9]+)*)', origin)
    if match is None:
        raise ValueError('Production browser requires the configured HTTPS application origin')
    host = match.group(1)
    addresses = {entry[4][0] for entry in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)}
    if addresses != {'127.0.0.1'}:
        raise ValueError('Local production application hostname must resolve only to 127.0.0.1')
    entries = [line.split('=', 1) for line in regular(cluster / 'identity').decode().splitlines()]
    identity = dict(entries)
    if len(identity) != len(entries):
        raise ValueError('Duplicate cluster identity field refused')
    revision = subprocess.check_output(['git', '-C', str(repo), 'rev-parse', '--short=12', 'HEAD'], text=True).strip()
    if identity.get('revision') != revision:
        raise ValueError('Production browser and beta cluster must use the same source revision')
    ca = cluster / 'ca/ca.crt'
    regular(ca)
    regular(cluster / 'ca/ca.key', True)
    service = 'https://localhost:19453'
    context = ssl.create_default_context(cafile=str(ca))
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(),
                                        urllib.request.HTTPSHandler(context=context))
    with opener.open(service + '/readyz', timeout=10) as response:
        if response.status != 200 or response.geturl() != service + '/readyz' or len(response.read(1048577)) > 1048576:
            raise ValueError('Actual Human backend readiness refused')
    ramp_service = os.environ.get('LAYERX_RAMP_SERVICE_URL')
    if ramp_service is not None:
        if re.fullmatch(r'https://[a-zA-Z0-9.-]+(?::[0-9]+)?', ramp_service) is None:
            raise ValueError('Ramp readiness requires a bare HTTPS service URL')
        ramp_request(opener, ramp_service)
    key, certificate = output / 'web.key', output / 'web.crt'
    run('openssl', 'genpkey', '-algorithm', 'EC', '-pkeyopt', 'ec_paramgen_curve:P-256', '-out', str(key))
    run('openssl', 'req', '-new', '-key', str(key), '-subj', '/CN=' + host, '-out', str(output / 'web.csr'))
    extension = output / 'web.ext'
    extension.write_text('basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\n'
                         'extendedKeyUsage=serverAuth\nsubjectAltName=DNS:' + host + '\n')
    run('openssl', 'x509', '-req', '-in', str(output / 'web.csr'), '-CA', str(ca),
        '-CAkey', str(cluster / 'ca/ca.key'), '-set_serial', '0x' + secrets.token_hex(16),
        '-days', '1', '-sha256', '-extfile', str(extension), '-out', str(certificate))
    run('openssl', 'verify', '-CAfile', str(ca), '-verify_hostname', host, '-purpose', 'sslserver', str(certificate))
    browser_home = output / 'browser-home'
    database = browser_home / '.pki/nssdb'
    database.mkdir(parents=True, mode=0o700)
    run('certutil', '-N', '--empty-password', '-d', 'sql:' + str(database))
    run('certutil', '-A', '-d', 'sql:' + str(database), '-n', 'LayerX beta browser CA', '-t', 'C,,', '-i', str(ca))
    configuration = output / 'tls.json'
    configuration.write_text(json.dumps(dict(version=1, origin=origin, service=service,
        key=str(key), certificate=str(certificate))))
    for path in (key, certificate, configuration):
        path.chmod(0o600)
    for value in (origin, service, str(ca), str(browser_home), str(configuration)):
        if '\n' in value or '\r' in value:
            raise ValueError('Production browser configuration contains a line break')
        print(value)


if __name__ == '__main__':
    os.umask(0o077)
    prepare(Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve())
