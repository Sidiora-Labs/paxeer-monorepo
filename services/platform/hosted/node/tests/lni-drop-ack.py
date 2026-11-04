#!/usr/bin/env python3
"""Bounded root JSON-RPC relay which loses one successful native submit response."""
import argparse
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import socket
import sys
import time

NODE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(NODE))
from native_runtime import endpoint, rpc
from reset_state import canonical, decode

MAX_BODY = 1_048_576
MAX_REQUESTS = 32


class LostResponseProxy(HTTPServer):
    def __init__(self, address, upstream, evidence=None):
        endpoint(upstream, loopback=True)
        super().__init__(address, Handler)
        self.upstream = upstream
        self.evidence = Path(evidence) if evidence is not None else None
        self.requests = []
        self.submitted = None
        self.transaction_hash = None
        self.dropped = False
        self.timeout = 1
        self.deadline = time.monotonic() + 30

    def record(self):
        if self.evidence is not None:
            # An exclusive evidence path prevents a prior run being mistaken for this run.
            with self.evidence.open('x') as stream:
                json.dump({'submits': sum(item['method'] == 'eth_sendRawTransaction'
                                          for item in self.requests),
                           'transaction_hash': self.transaction_hash,
                           'dropped': self.dropped,
                           'requests': self.requests}, stream, sort_keys=True)
                stream.write('\n')


class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(5)

    def do_POST(self):
        proxy = self.server
        try:
            if (time.monotonic() >= proxy.deadline or len(proxy.requests) >= MAX_REQUESTS
                    or self.path != '/' or self.headers.get('Transfer-Encoding') is not None
                    or self.headers.get('Content-Type', '').split(';')[0] != 'application/json'):
                raise ValueError('root JSON-RPC request outside relay bounds')
            lengths = self.headers.get_all('Content-Length', [])
            if len(lengths) != 1 or not lengths[0].isdecimal():
                raise ValueError('one bounded content length is required')
            size = int(lengths[0])
            if not 0 < size <= MAX_BODY:
                raise ValueError('JSON-RPC request outside relay bounds')
            raw = self.rfile.read(size)
            if len(raw) != size:
                raise ValueError('incomplete JSON-RPC request')
            request = decode(raw)
            if (not isinstance(request, dict)
                    or set(request) != {'jsonrpc', 'id', 'method', 'params'}
                    or request['jsonrpc'] != '2.0'
                    or not isinstance(request['id'], (str, int)) or isinstance(request['id'], bool)
                    or not isinstance(request['params'], list)):
                raise ValueError('invalid JSON-RPC envelope')
            method, params = request['method'], request['params']
            if method not in ('eth_sendRawTransaction', 'eth_getTransactionReceipt',
                              'eth_getBalance', 'eth_chainId', 'eth_getBlockByNumber'):
                raise ValueError('request outside native submission/recovery path')
            if method == 'eth_sendRawTransaction':
                if proxy.submitted is not None:
                    raise ValueError('indeterminate submission must recover by its signed identity')
                if (len(params) != 1 or not isinstance(params[0], str)
                        or not params[0].startswith('0x') or not bytes.fromhex(params[0][2:])):
                    raise ValueError('one signed native transaction is required')
                proxy.submitted = params[0]
            result = rpc(proxy.upstream, method, params)
            proxy.requests.append(request)
            if method == 'eth_sendRawTransaction':
                proxy.transaction_hash = result
                proxy.dropped = True
                self.close_connection = True
                self.connection.shutdown(socket.SHUT_RDWR)
                return
            body = canonical({'jsonrpc': '2.0', 'id': request['id'], 'result': result})
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        except (OSError, ValueError, KeyError, TypeError) as error:
            self.send_error(400, str(error))

    def log_message(self, *_args):
        pass


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('upstream')
    parser.add_argument('evidence', type=Path)
    parser.add_argument('--port', type=int, default=0)
    args = parser.parse_args()
    if args.evidence.exists() or not 0 <= args.port <= 65535:
        raise ValueError('fresh evidence path and bounded port required')
    with LostResponseProxy(('127.0.0.1', args.port), args.upstream, args.evidence) as proxy:
        print('http://127.0.0.1:' + str(proxy.server_port), flush=True)
        while time.monotonic() < proxy.deadline and len(proxy.requests) < MAX_REQUESTS:
            proxy.handle_request()
            if proxy.dropped and any(item['method'] == 'eth_getTransactionReceipt'
                                     for item in proxy.requests):
                proxy.record()
                return
        raise ValueError('native lost-response recovery did not complete within bounds')


if __name__ == '__main__':
    main()
