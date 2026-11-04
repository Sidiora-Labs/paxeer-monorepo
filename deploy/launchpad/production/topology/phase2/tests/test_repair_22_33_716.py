from __future__ import annotations

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import sys
import threading

TOPOLOGY = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(TOPOLOGY))

from phase2 import v1
from phase2.operator import Journal
from phase2.providers import connected_plan
from phase2.tests.test_operator import (
    assert_connected_runtime_bindings,
    connected_inputs,
    test_connected_render_binds_every_runtime_role,
)
from phase2.tests.test_repair_22_31_313 import actual_adapter


def test_topology_connected_test_gap_real_client_to_operator_journal(connected_inputs, tmp_path):
    config, _, _, documents, _, _ = connected_inputs
    produced = connected_plan(*connected_inputs)
    payload = v1.canonical(produced)
    requests = []

    class ConnectedRender(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers['Content-Type']))
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), ConnectedRender)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        client = actual_adapter().LocalHTTP('http://127.0.0.1:' + str(port), {port})
        status, received = client.request('GET', '/topology/connected')
        assert status == 200
        assert received == payload
        assert requests == [('GET', '/topology/connected', 'application/json')]
        consumed = v1.decode(received)
        assert consumed == produced
        assert_connected_runtime_bindings(consumed, config, documents)
        journal_path = tmp_path / 'operator.db'
        journal = Journal(journal_path)
        try:
            proposal = journal.propose(config, consumed, 'connected-render-operator')
            assert proposal['state'] == 'proposed'
            assert proposal['actor'] == 'connected-render-operator'
        finally:
            journal.close()
        reopened = Journal(journal_path)
        try:
            recovered = v1.decode(reopened.proposal(config['revision'])['plan'].encode())
            assert recovered == produced
            assert_connected_runtime_bindings(recovered, config, documents)
        finally:
            reopened.close()
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
    assert not thread.is_alive()
