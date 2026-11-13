#!/usr/bin/env python3
"""Deliver the development indexer's real Redis events to statsd's webhook."""
import hashlib
import hmac
import json
import os
from pathlib import Path
import socket
import sys
import time
import urllib.error
import urllib.request
from urllib.parse import urlparse


CHANNELS = ('indexer:market_created', 'indexer:swap', 'indexer:pool_state_updated')


def read_resp(stream):
    line = stream.readline()
    if not line:
        raise ConnectionError('Redis subscription closed')
    kind, value = line[:1], line[1:-2]
    if kind == b'*':
        return [read_resp(stream) for _ in range(int(value))]
    if kind == b'$':
        size = int(value)
        if size < 0:
            return None
        payload = stream.read(size)
        if len(payload) != size or stream.read(2) != b'\r\n':
            raise ConnectionError('Incomplete Redis event')
        return payload
    if kind == b':':
        return int(value)
    if kind == b'-':
        raise RuntimeError('Redis refused subscription: ' + value.decode())
    raise ValueError('Unexpected Redis reply: ' + repr(line))


def deliver(event, url, secret):
    body = json.dumps({'events': [event]}, separators=(',', ':')).encode()
    deadline = time.monotonic() + 30
    while True:
        timestamp = str(int(time.time()))
        signature = 'sha256=' + hmac.new(secret.encode(), timestamp.encode() + b'.' + body,
                                          hashlib.sha256).hexdigest()
        request = urllib.request.Request(url, data=body, headers={
            'Content-Type': 'application/json', 'X-Sidiora-Timestamp': timestamp,
            'X-Sidiora-Signature': signature})
        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                if response.status == 200:
                    return
                raise RuntimeError('statsd webhook answered ' + str(response.status))
        except urllib.error.HTTPError as error:
            if error.code != 503 or time.monotonic() >= deadline:
                raise RuntimeError('statsd webhook answered ' + str(error.code)
                                   + ': ' + error.read().decode()) from error
        time.sleep(1)


def main():
    parsed = urlparse(os.environ['REDIS_URL'])
    if parsed.scheme != 'redis' or parsed.hostname != '127.0.0.1' or not parsed.port:
        raise ValueError('Owned loopback Redis required')
    with socket.create_connection((parsed.hostname, parsed.port), timeout=5) as connection:
        connection.settimeout(30)
        stream = connection.makefile('rb')
        command = (f'*{len(CHANNELS) + 1}\r\n$9\r\nSUBSCRIBE\r\n'
                   + ''.join(f'${len(channel)}\r\n{channel}\r\n' for channel in CHANNELS))
        connection.sendall(command.encode())
        for _ in CHANNELS:
            answer = read_resp(stream)
            if answer[:1] != [b'subscribe']:
                raise RuntimeError('Redis did not subscribe to indexer events')
        Path(os.environ['STATS_FORWARDER_READY']).write_text('subscribed\n')
        print('Subscribed to real development indexer events', flush=True)
        connection.settimeout(None)
        while True:
            answer = read_resp(stream)
            if answer[0] != b'message' or answer[1].decode() not in CHANNELS:
                raise RuntimeError('Unexpected Redis publication')
            event = json.loads(answer[2])
            if event.get('eventName') not in ('MarketCreated', 'Swap', 'PoolStateUpdated'):
                raise ValueError('Indexer channel and event disagree')
            deliver(event, os.environ['STATS_WEBHOOK_URL'], os.environ['WEBHOOK_HMAC_SECRET'])
            print('Delivered ' + event['eventName'] + ' ' + event.get('txHash', ''), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print('stats forwarder: ' + str(error), file=sys.stderr, flush=True)
        raise
