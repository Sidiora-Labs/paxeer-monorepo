"""Documentation facts and the independent Actions authority's real HTTP path."""

import contextlib
import http.client
import json
import re
import shlex
import threading
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

from ci_tools import TOOLS
from controller import GitHubChecks
from policy import Rejected, admission


DIRECTORY = Path(__file__).resolve().parent
ROOT = DIRECTORY.parents[3]
WORKFLOW_PATH = '.github/workflows/offchain-images.yml'
CHECK_NAMES = ('verify', 'contracts', 'frontend', 'policy', 'scan', 'signing')


def workflow_jobs():
    """Read job boundaries without interpreting GitHub expression syntax as YAML."""
    source = (ROOT / WORKFLOW_PATH).read_text()
    matches = list(re.finditer(r'^  ([a-z]+):\n', source.split('\njobs:\n', 1)[1], re.M))
    jobs_source = source.split('\njobs:\n', 1)[1]
    return {
        match[1]: jobs_source[
            match.end():matches[index + 1].start() if index + 1 < len(matches) else None
        ]
        for index, match in enumerate(matches)
    }


def test_supplychain_readme_ci_dependencies_and_publication_authority():
    readme = (DIRECTORY / 'README.md').read_text()
    workflow = (ROOT / WORKFLOW_PATH).read_text()
    jobs = workflow_jobs()
    assert set(jobs) == {'plan', 'images', 'digests'}
    for name, block in jobs.items():
        match = re.search(r'^    needs: \[([^]]*)\]$', block, re.M)
        dependencies = re.findall(r'[a-z]+', match[1]) if match else []
        row = ', '.join(f'`{dependency}`' for dependency in dependencies) or 'none'
        assert f'| `{name}` | {row} |' in readme

    assert re.search(r'^  push:\n    branches: \[main\]$', workflow, re.M)
    assert re.search(r'^  workflow_dispatch:$', workflow, re.M)
    assert not re.search(r'^  pull_request:', workflow, re.M)
    for source_fact, documented_fact in (
        ('      contents: read', '`contents: read`'),
        ('      packages: write', '`packages: write`'),
        ('          registry: ghcr.io', 'logs into GHCR'),
        ('          password: ${{ secrets.GITHUB_TOKEN }}', 'workflow token'),
        ('          push: true', '`push: true`'),
        ('ghcr.io/sidiora-labs/', '`ghcr.io/sidiora-labs`'),
        ('${{ needs.plan.outputs.sha }}', '12-character source revision'),
        ('${{ needs.plan.outputs.describe }}', '`git describe`'),
    ):
        assert source_fact in jobs['images']
        assert documented_fact in readme
    assert "!cancelled() && needs.plan.result == 'success'" in jobs['digests']
    assert "jq -e 'length > 0' digests.json" in jobs['digests']
    assert 'Missing rows remain\nabsent' in readme
    assert 'rejects an empty result' in readme
    assert 'does not\ninvoke the six-check release admission policy' in readme
    assert 'proves neither policy admission nor deployment approval' in readme
    assert "admission policies' independent requirement for push evidence" in readme


def test_supplychain_readme_paths_baseline_and_independent_constraints():
    readme = (DIRECTORY / 'README.md').read_text()
    commands = re.findall(r'```sh\n(.*?)\n```', readme, re.S)
    assert len(commands) == 1
    assert shlex.split(commands[0]) == [
        'python3', '-m', 'unittest', 'discover', '-s',
        str(DIRECTORY.relative_to(ROOT)), '-p', 'test_*.py', '-v',
    ]
    for target in re.findall(r'\]\(([^)]+)\)', readme):
        assert (DIRECTORY / target).resolve().is_file(), target
    assert TOOLS == {
        'go': (['version'], 'go1.26.4'),
        'node': (['--version'], 'v24.16.0'),
        'pnpm': (['--version'], '9.15.0'),
    }
    for version in ('Go 1.26.4', 'Node 24.16.0', 'pnpm 9.15.0'):
        assert version in readme
    for name in CHECK_NAMES:
        assert f'`{name}`' in readme
    for constraint in (
        'Artifact fields cannot establish trust',
        'Evidence lifetime is at most 24 hours',
        'Forks, pull requests, stale evidence',
        'reachable critical findings',
        'A high exception lasts at most seven days',
        'independently approved canonical digest',
        'successful admission alone is not registry publication',
    ):
        assert constraint in readme
    with pytest.raises(Rejected, match='missing or unexpected checks'):
        admission({'checks': {name: {} for name in workflow_jobs()}}, {}, 1)


@contextlib.contextmanager
def actions_server(monkeypatch, run, jobs):
    """Serve Actions JSON over sockets; only route the fixed origin to loopback."""
    received = []
    repository = 'sidiora-labs/paxeer-monorepo'
    run_path = f'/repos/{repository}/actions/runs/42'
    jobs_path = run_path + '/attempts/1/jobs?per_page=100&page=1'
    documents = {run_path: run, jobs_path: {'total_count': len(jobs), 'jobs': jobs}}

    class ActionsHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            received.append((self.path, dict(self.headers)))
            if self.path not in documents:
                self.send_error(404)
                return
            payload = json.dumps(documents[self.path]).encode('utf-8')
            self.send_response(200)
            self.send_header('Content-Type', 'application/json; charset=utf-8')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), ActionsHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)

    class LoopbackHTTPSHandler(urllib.request.HTTPSHandler):
        def https_open(self, request):
            assert request.host == 'api.github.com'

            def connection(host, **kwargs):
                assert host == 'api.github.com'
                return http.client.HTTPConnection(*server.server_address, **kwargs)

            return self.do_open(connection, request)

    original_build_opener = urllib.request.build_opener

    def loopback_opener(*handlers):
        return original_build_opener(
            urllib.request.ProxyHandler({}), LoopbackHTTPSHandler(), *handlers
        )

    thread.start()
    try:
        with monkeypatch.context() as routing:
            routing.setattr(urllib.request, 'build_opener', loopback_opener)
            yield GitHubChecks(repository, token='loopback-test-token'), received
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
    assert received
    for path, headers in received:
        assert path in documents
        assert headers['Host'] == 'api.github.com'
        assert headers['Accept'] == 'application/vnd.github+json'
        assert headers['X-Github-Api-Version'] == '2022-11-28'
        assert headers['User-Agent'] == 'KindleLaunch-release-admission'
        assert headers['Authorization'] == 'Bearer loopback-test-token'


def actions_authority():
    expectation = {
        'run_id': '42', 'run_attempt': 1, 'commit': 'a' * 40,
        'ref': 'refs/heads/main', 'workflow_path': WORKFLOW_PATH,
        'job_names': {name: name for name in CHECK_NAMES},
    }
    run = {
        'id': 42, 'event': 'push', 'head_sha': expectation['commit'],
        'head_branch': 'main', 'path': WORKFLOW_PATH, 'run_attempt': 1,
        'head_repository': {'full_name': 'sidiora-labs/paxeer-monorepo'},
        'repository': {'full_name': 'sidiora-labs/paxeer-monorepo'},
        'status': 'completed', 'conclusion': 'success',
    }
    jobs = [
        {
            'id': index, 'name': name, 'head_sha': expectation['commit'],
            'status': 'completed', 'conclusion': 'success',
        }
        for index, name in enumerate(CHECK_NAMES, 1)
    ]
    receipts = {job['name']: {'id': str(job['id'])} for job in jobs}
    return expectation, run, jobs, receipts


def test_supplychain_readme_publication_jobs_do_not_grant_admission(monkeypatch):
    expectation, run, jobs, receipts = actions_authority()
    published_jobs = [
        {**jobs[index], 'name': name}
        for index, name in enumerate(workflow_jobs())
    ]
    with actions_server(monkeypatch, run, published_jobs) as (client, received):
        with pytest.raises(Rejected, match='Actions check identity mismatch: verify'):
            client.verify(expectation, receipts)
        assert len(received) == 2


def test_supplychain_readme_independent_actions_authority(monkeypatch):
    expectation, run, jobs, receipts = actions_authority()
    with actions_server(monkeypatch, run, jobs) as (client, received):
        assert client.verify(expectation, receipts) == {
            'run_id': '42', 'run_attempt': 1,
            'jobs': {name: receipt['id'] for name, receipt in receipts.items()},
        }
        assert len(received) == 2


@pytest.mark.parametrize(('field', 'value', 'reason'), [
    ('event', 'workflow_dispatch', 'Actions source or event mismatch'),
    ('head_sha', 'b' * 40, 'Actions source or event mismatch'),
    ('head_repository', {'full_name': 'untrusted/fork'}, 'fork Actions authority'),
    ('path', '.github/workflows/untrusted.yml', 'Actions workflow/attempt mismatch'),
    ('run_attempt', 2, 'Actions workflow/attempt mismatch'),
    ('conclusion', 'failure', 'Actions run is not successful'),
])
def test_supplychain_readme_untrusted_actions_cannot_authorize(
    monkeypatch, field, value, reason
):
    expectation, run, jobs, receipts = actions_authority()
    run[field] = value
    with actions_server(monkeypatch, run, jobs) as (client, received):
        with pytest.raises(Rejected, match=reason):
            client.verify(expectation, receipts)
        assert len(received) == 1
