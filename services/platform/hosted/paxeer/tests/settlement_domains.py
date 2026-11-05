import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import subprocess
import sys
import threading
import unittest

ROOT = Path(__file__).resolve().parents[1]


class SettlementDomains(unittest.TestCase):
    def test_committed_native_evidence_replaces_domain_writer(self):
        fixture = json.loads((ROOT / 'tests/fixtures/repair_1321/committed_chain.json').read_text())
        requests = []

        class Peer(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                requests.append(request)
                exchange = next(value for value in fixture['rpc_exchanges']
                                if value['method'] == request['method']
                                and value['params'] == request['params'])
                raw = json.dumps({'jsonrpc': '2.0', 'id': request['id'],
                                  'result': exchange['result']}).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, *_args):
                pass

        server = HTTPServer(('127.0.0.1', 0), Peer)
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            endpoint = f'http://127.0.0.1:{server.server_port}/'
            result = subprocess.run([BINARY, 'evidence', endpoint, endpoint],
                                    capture_output=True, text=True, check=True)
            evidence = json.loads(result.stdout)
            self.assertEqual(evidence['chain_id'], 125)
            self.assertEqual(evidence['height'], 23869863)
            self.assertEqual(evidence['consensus_chain_id'], 'hyperpax_125-1')
            validators = next(value['result']['validators'] for value in fixture['rpc_exchanges']
                              if value['method'] == 'validators')
            self.assertEqual(evidence['validators'], validators)
            self.assertEqual(requests[-1]['params'],
                             {'height': '23869863', 'page': '1', 'per_page': '100'})
            self.assertNotIn('guarantor_set', evidence)
            self.assertNotIn('settlement_contract', evidence)
            self.assertFalse((ROOT / 'settlement-domain.py').exists())
            self.assertFalse((ROOT / 'prepare-beta.py').exists())
            result = subprocess.run([BINARY, 'write', 'settlement.json', 'beta'],
                                    capture_output=True, text=True, check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('retired', result.stderr)
            self.assertEqual(result.stdout, '')
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == '__main__':
    BINARY = sys.argv.pop(1)
    unittest.main()
