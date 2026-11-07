from __future__ import annotations

import argparse
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import ssl
import stat
from urllib.parse import urlsplit

from contracts import Invalid, load, require, validate_binding
from render import safe_directory, verify_bundle
from runtime import prometheus_samples


def private_bytes(path, limit):
    path = safe_directory(Path(path).parent) / Path(path).name
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        metadata = os.fstat(descriptor)
        require(stat.S_ISREG(metadata.st_mode) and metadata.st_uid == os.geteuid()
                and metadata.st_mode & 0o077 == 0 and metadata.st_size <= limit,
                'synthetic mount must be a private regular file')
        with os.fdopen(descriptor, 'rb', closefd=False) as stream:
            raw = stream.read(limit + 1)
        require(len(raw) <= limit, 'synthetic mount exceeds bound')
        return raw
    finally:
        os.close(descriptor)


def bearer_token(path):
    token = private_bytes(path, 8192).strip()
    require(token and all(33 <= value <= 126 for value in token), 'invalid bearer mount')
    return token


def metrics_server(binding, config):
    validate_binding(binding, config)
    synthetic = binding['synthetic']
    target = urlsplit(synthetic['url'])

    class MetricsHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path != target.path:
                self.respond(404, b'not found\n')
                return
            try:
                expected = b'Bearer ' + bearer_token(synthetic['token_file'])
                supplied = self.headers.get_all('Authorization', [])
                if len(supplied) != 1 or not hmac.compare_digest(
                    supplied[0].encode('utf-8'), expected
                ):
                    self.respond(401, b'authentication required\n')
                    return
                raw = private_bytes(Path(synthetic['textfile_directory'])/'synthetic.prom', 65536)
                samples = prometheus_samples(raw, config)
                for (name, labels), value in samples.items():
                    labels = dict(labels)
                    require(name in ('synthetic_journey_expected', 'synthetic_journey_success',
                                     'synthetic_journey_last_observed_timestamp_seconds')
                            and set(labels) == {'journey', 'environment', 'zone'}
                            and labels['environment'] == binding['environment']
                            and labels['zone'] == synthetic['zone'], 'foreign synthetic series')
                    if name != 'synthetic_journey_last_observed_timestamp_seconds':
                        require(value in (0, 1), 'invalid synthetic state')
                self.respond(200, raw)
            except (Invalid, OSError, ValueError):
                self.respond(503, b'synthetic metrics unavailable\n')

        def respond(self, status, raw):
            self.send_response(status)
            self.send_header('Content-Type', 'text/plain; version=0.0.4; charset=utf-8')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer((target.hostname, target.port), MetricsHandler)
    try:
        if target.scheme == 'https':
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.minimum_version = ssl.TLSVersion.TLSv1_2
            context.load_cert_chain(synthetic['cert_file'], synthetic['key_file'])
            server.socket = context.wrap_socket(server.socket, server_side=True)
        return server
    except BaseException:
        server.server_close()
        raise


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binding', required=True)
    parser.add_argument('--config', required=True)
    parser.add_argument('--bundle', required=True)
    arguments = parser.parse_args()
    binding, config = load(arguments.binding), load(arguments.config)
    verify_bundle(binding, config, arguments.bundle)
    with metrics_server(binding, config) as server:
        server.serve_forever()


if __name__ == '__main__':
    main()
