#!/usr/bin/env python3
"""Start the real launchpad processes over a private, migrated database."""
import hashlib
import json
import os
from pathlib import Path
import pwd
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.request
import urllib.error
import urllib.parse
from contextlib import ExitStack
from email.utils import parsedate_to_datetime


def ports_for(roles):
    # Keep every listener bound until the entire set has been selected.
    with ExitStack() as listeners:
        ports = {}
        for role in roles:
            listener = listeners.enter_context(socket.socket())
            listener.bind(('127.0.0.1', 0))
            ports[role] = listener.getsockname()[1]
        return ports


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def ready(owner, child, url, timeout=90):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if child.poll() is not None:
            raise owner.readiness_error(child, 'exited before readiness: ' + url)
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except OSError:
            time.sleep(.1)
    raise owner.readiness_error(child, 'readiness timed out: ' + url)


def record_stream_readiness(owner, answer):
    answer['time'] = time.time()
    record = json.dumps(answer) + '\n'
    (owner.output / 'joined-stream-readiness.json').write_text(record)
    with (owner.output / 'joined-stream-readiness.ndjson').open('a') as stream:
        stream.write(record)


def projection_ready(owner, child, database, pool, timeout=90):
    deadline = time.monotonic() + timeout
    log = owner.output / 'joined-projection-readiness.ndjson'
    while time.monotonic() < deadline:
        if child.poll() is not None:
            raise owner.readiness_error(child, 'exited before market projection readiness')
        result = subprocess.run([
            'psql', database, '-At', '-v', 'ON_ERROR_STOP=1',
            '-v', 'topic=market.trades.' + pool.lower()],
            input="SELECT EXISTS (SELECT 1 FROM api_stream_dev.stream_projection "
                  "WHERE topic=:'topic' AND sequence>0 AND frame IS NOT NULL);\n",
            capture_output=True, text=True, timeout=5)
        with log.open('a') as stream:
            stream.write(json.dumps({'time': time.time(), 'exit': result.returncode,
                'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
        if result.returncode:
            raise owner.readiness_error(child, 'market projection probe failed: ' + str(log))
        if result.stdout.strip() == 't':
            return
        time.sleep(1)
    raise owner.readiness_error(child, 'market projection readiness timed out: ' + str(log))


def chart_subscription_ready(owner, child, redis_url, timeout=30):
    """Subscribe before the indexer publishes the fresh chain's backfill."""
    deadline = time.monotonic() + timeout
    log = owner.output / 'joined-chart-subscription.ndjson'
    while time.monotonic() < deadline:
        if child.poll() is not None:
            raise owner.readiness_error(child, 'exited before chart subscription readiness')
        result = subprocess.run(
            ['redis-cli', '-u', redis_url, '--json', 'PUBSUB', 'NUMSUB', 'indexer:swap'],
            capture_output=True, text=True, timeout=5)
        with log.open('a') as stream:
            stream.write(json.dumps({'time': time.time(), 'exit': result.returncode,
                'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
        if result.returncode:
            raise owner.readiness_error(child, 'chart subscription probe failed: ' + str(log))
        subscribers = json.loads(result.stdout)
        if subscribers == ['indexer:swap', 1]:
            return
        time.sleep(.1)
    raise owner.readiness_error(child, 'chart subscription readiness timed out: ' + str(log))


def stats_row_ready(owner, stats, forwarder, database, pool, timeout=90):
    log = owner.output / 'joined-stats-row-readiness.ndjson'
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for child in (stats, forwarder):
            if child.poll() is not None:
                raise owner.readiness_error(child, 'exited before the indexed pool statistics row existed')
        result = subprocess.run(['psql', database, '-At', '-v', 'ON_ERROR_STOP=1',
                                 '-v', 'pool=' + pool.lower()],
            input="SELECT EXISTS (SELECT 1 FROM stats.pool_stats WHERE lower(pool_address)=:'pool');\n",
            capture_output=True, text=True, timeout=5)
        with log.open('a') as stream:
            stream.write(json.dumps({'time': time.time(), 'exit': result.returncode,
                'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
        if result.returncode:
            raise owner.readiness_error(stats, 'pool statistics probe failed: ' + str(log))
        if result.stdout.strip() == 't':
            return
        time.sleep(1)
    raise owner.readiness_error(stats, 'indexed pool statistics row timed out: ' + str(log))


def pool_state_ready(owner, indexer, database, pool, timeout=90):
    log = owner.output / 'joined-pool-state-readiness.ndjson'
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if indexer.poll() is not None:
            raise owner.readiness_error(indexer, 'exited before the chain reserve event was indexed')
        result = subprocess.run(['psql', database, '-At', '-v', 'ON_ERROR_STOP=1',
                                 '-v', 'pool=' + pool.lower()],
            input="SELECT EXISTS (SELECT 1 FROM indexer.pools p JOIN indexer.pool_state_snapshots s "
                  "ON s.pool_id=p.pool_id WHERE lower(p.pool_address)=:'pool');\n",
            capture_output=True, text=True, timeout=5)
        with log.open('a') as stream:
            stream.write(json.dumps({'time': time.time(), 'exit': result.returncode,
                'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
        if result.returncode:
            raise owner.readiness_error(indexer, 'indexed pool state probe failed: ' + str(log))
        if result.stdout.strip() == 't':
            return
        time.sleep(1)
    raise owner.readiness_error(indexer, 'indexed pool state timed out: ' + str(log))


def stream_ready(owner, child, origin, pool, browser_origin, timeout=90):
    url = origin + '/v2/snapshots?' + urllib.parse.urlencode({'topics': 'market.trades.' + pool.lower()})
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if child.poll() is not None:
            raise owner.readiness_error(child, 'exited before stream readiness: ' + url)
        request = urllib.request.Request(url, headers={
            'X-Schema-Version': '2', 'Origin': browser_origin})
        retry_delay = 1
        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                body = response.read()
                record_stream_readiness(owner, {
                    'url': url, 'status': response.status, 'body': body.decode(),
                    'retry_after': response.headers.get('Retry-After')})
                if response.status == 200:
                    return
        except urllib.error.HTTPError as error:
            body = error.read().decode()
            record_stream_readiness(owner, {
                'url': url, 'status': error.code, 'body': body,
                'retry_after': error.headers.get('Retry-After')})
            retry_after = error.headers.get('Retry-After')
            if retry_after:
                try:
                    retry_delay = max(1, float(retry_after))
                except ValueError:
                    retry_delay = max(1, parsedate_to_datetime(retry_after).timestamp() - time.time())
            if error.code == 404 and body.strip() == '404 page not found':
                raise owner.readiness_error(child, 'required public snapshot route is not mounted: '
                                            + url + '; HTTP 404: ' + body.strip())
            if error.code == 404:
                try:
                    envelope = json.loads(body)
                except ValueError:
                    envelope = {}
                if envelope.get('code') != 'NOT_FOUND':
                    raise owner.readiness_error(child, 'unexpected public snapshot 404: ' + body)
            if error.code == 503:
                disabled = [line for line in (owner.output / 'api.log').read_text().splitlines()
                            if 'development stream disabled' in line]
                (owner.output / 'joined-stream-disabled.log').write_text('\n'.join(disabled) + '\n')
                raise owner.readiness_error(child, 'public market snapshot answered 503; ' + str(owner.output / 'joined-stream-disabled.log'))
        except OSError as error:
            record_stream_readiness(owner, {'url': url, 'transportError': str(error)})
        time.sleep(min(retry_delay, max(0, deadline - time.monotonic())))
    raise owner.readiness_error(child, 'stream readiness timed out: ' + url)


def prepare_fixture(owner):
    web = Path(__file__).resolve().parents[3]
    root = web.parents[2]
    services = root / 'services/launchpad'
    fixture = json.loads(Path(owner.env['L27_PHASE2_FIXTURE']).read_text())['phase2']
    if not fixture['ownedFixture'] or not fixture['rpcUrl'].startswith('http://127.0.0.1:'):
        raise ValueError('Joined services require the owned creation deployment')
    # The development pool detail requires an indexed reserve event. Its
    # creation and swaps alone do not emit PoolStateUpdated.
    owner.run(['cast', 'send', '--unlocked', '--from', fixture['chartAccount'],
               '--rpc-url', fixture['rpcUrl'], fixture['pool'], 'syncReserves()'],
              'joined-sync-owned-pool')
    output = owner.output / 'joined'
    output.mkdir(mode=0o700)
    # Each run gets a fresh database server; no existing database or Redis is reset.
    pg_bin = Path(subprocess.check_output(['pg_config', '--bindir'], text=True).strip())
    data = Path(tempfile.mkdtemp(prefix='launchpad-postgres-'))
    prefix = []
    if os.geteuid() == 0:
        user = pwd.getpwnam('postgres')
        os.chown(data, user.pw_uid, user.pw_gid)
        prefix = ['runuser', '-u', 'postgres', '--']
    username = 'postgres' if prefix else pwd.getpwuid(os.geteuid()).pw_name
    owner.run([*prefix, str(pg_bin / 'initdb'), '-D', str(data), '--auth-local=trust',
               '--auth-host=trust', '--no-locale', '--encoding=UTF8'], 'joined-initdb')
    database_ports = ports_for(['postgres', 'redis', 'nats', 'nats-monitor'])
    pg_port, redis_port = database_ports['postgres'], database_ports['redis']
    postgres = owner.start([*prefix, str(pg_bin / 'postgres'), '-D', str(data), '-h', '127.0.0.1',
                            '-p', str(pg_port), '-k', str(data)], 'joined-postgres')
    admin = f'postgresql://{username}@127.0.0.1:{pg_port}/postgres?sslmode=disable'
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        probe = subprocess.run([str(pg_bin / 'pg_isready'), '-d', admin], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if probe.returncode == 0:
            break
        if postgres.poll() is not None:
            raise owner.readiness_error(postgres, 'exited before PostgreSQL readiness')
        time.sleep(.1)
    else:
        raise owner.readiness_error(postgres, 'PostgreSQL readiness timed out')
    owner.run(['psql', admin, '-v', 'ON_ERROR_STOP=1', '-c', 'CREATE DATABASE launchpad_joined'], 'joined-createdb')
    database = admin.replace('/postgres?', '/launchpad_joined?')
    redis = owner.start(['redis-server', '--bind', '127.0.0.1', '--port', str(redis_port),
                         '--save', '', '--appendonly', 'no', '--dir', str(output)], 'joined-redis')
    redis_url = f'redis://127.0.0.1:{redis_port}/0'
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        result = subprocess.run(['redis-cli', '-u', redis_url, 'PING'], capture_output=True, text=True)
        if result.returncode == 0 and result.stdout.strip() == 'PONG':
            break
        if redis.poll() is not None:
            raise owner.readiness_error(redis, 'exited before Redis readiness')
        time.sleep(.1)
    else:
        raise owner.readiness_error(redis, 'Redis readiness timed out')
    nats_binary = owner.env['NATS_SERVER_BINARY']
    nats_url = f"nats://127.0.0.1:{database_ports['nats']}"
    broker = owner.start([nats_binary, '--addr', '127.0.0.1', '--port', str(database_ports['nats']),
                          '--jetstream', '--store_dir', str(output / 'nats'),
                          '--http_port', str(database_ports['nats-monitor'])], 'joined-nats')
    ready(owner, broker, f"http://127.0.0.1:{database_ports['nats-monitor']}/healthz")
    roles = fixture['authorityDeployment']['roles']
    proof = output / 'private-deployment.json'
    proof.write_text(json.dumps({'schemaVersion': 1, 'realm': 'private_fixture',
        'deployment': fixture['authorityDeployment'], 'rpcUrl': fixture['rpcUrl'],
        'genesisHash': fixture['genesisHash'], 'creationTransactions': fixture['creationTransactions']}) + '\n')
    signing_key = secrets.token_hex(32)
    ports = ports_for(['browser', 'gateway', 'api', 'trading-charts', 'indexer', 'stats'])
    origins = {role: f'http://127.0.0.1:{value}' for role, value in ports.items()}
    # The identity loader uses URL.Host as the schema's hostname-only domain.
    # Reuse the owned TLS issuer's standard HTTPS origin, without a port.
    public = json.loads(Path(owner.env['L25_QUALIFICATION_CONFIG_FILE']).read_text())['identityOrigin']
    common = {**owner.env, 'GOWORK': 'off', 'NODE_ENV': 'test', 'DEPLOYMENT_REALM': 'private_fixture',
        'PRIVATE_FIXTURE_MANIFEST_FILE': str(proof), 'PRIVATE_FIXTURE_MANIFEST_SHA256': digest(proof),
        'DATABASE_URL': database, 'REDIS_URL': redis_url, 'REDIS_BULL_URL': redis_url,
        'NATS_URL': nats_url,
        'RPC_URL': fixture['rpcUrl'], 'RPC_URLS': fixture['rpcUrl'], 'NETWORK_RPC_URL': fixture['rpcUrl'], 'CHAIN_ID': '125',
        'CORS_ALLOWED_ORIGINS': public, 'BIND_ADDRESS': '127.0.0.1', 'LOG_LEVEL': 'info',
        'WEBHOOK_HMAC_SECRET': secrets.token_hex(32), 'STREAM_SIGNING_KEY': signing_key,
        'IDENTITY_SIGNING_SECRET': secrets.token_hex(32), 'IDENTITY_LIMITER_KEY': secrets.token_hex(32),
        'IDENTITY_SERVICE_TOKEN': secrets.token_hex(32), 'GATEWAY_JWT_SECRET': secrets.token_hex(32),
        'PUBLIC_URL': public, 'SOCIAL_HTTP_URL': origins['browser'],
        'SOCIAL_WS_URL': origins['browser'].replace('http:', 'ws:'), 'METADATA_UPLOAD_URL': origins['browser'],
        'S3_ENDPOINT': origins['browser'], 'S3_ACCESS_KEY_ID': secrets.token_hex(16),
        'S3_SECRET_ACCESS_KEY': secrets.token_hex(32), 'METADATA_BUCKET': 'qualification',
        'IDENTITY_SERVICE_URL': origins['gateway'], 'INDEXER_START_BLOCK': '0',
        'INDEXER_LOG_SOURCE': 'rpc-getlogs', 'INDEXER_HEAD_SOURCE': 'rpc',
    }
    # The development API owns REST reads and realtime on one origin. Never
    # inherit a production role from the invoking qualification environment.
    common.pop('API_ROLE', None)
    for name, variable in {'EventEmitter': 'EVENT_EMITTER_ADDRESS', 'ProtocolConfig': 'PROTOCOL_CONFIG_ADDRESS',
        'Router': 'ROUTER_ADDRESS', 'Quoter': 'QUOTER_ADDRESS', 'SidioraFactory': 'FACTORY_ADDRESS',
        'PoolRegistry': 'POOL_REGISTRY_ADDRESS', 'FeeAccumulator': 'FEE_ACCUMULATOR_ADDRESS',
        'SidioraNFT': 'SIDIORA_NFT_ADDRESS', 'FeesRouter': 'FEES_ROUTER_ADDRESS', 'PoolBeacon': 'POOL_BEACON_ADDRESS'}.items():
        common[variable] = roles[name]['address']
    # Indexer and trading-charts migrate at development startup; gateway owns
    # its identity migrations. The API does not migrate the stats schema.
    processes = [{'role': 'nats', 'ready_url': f"http://127.0.0.1:{database_ports['nats-monitor']}/healthz",
                  'pid': broker.pid, 'executable_sha256': digest(nats_binary)}]
    # Redis pub/sub does not retain the backfill. Start both consumers before
    # the development indexer publishes the owned chain's canonical events.
    for role, service, entry in [('trading-charts', 'trading-charts', 'chartsd'),
                                  ('stats', 'stats-workers', 'statsd'),
                                  ('indexer', 'indexer', 'indexerd'),
                                  ('gateway', 'gateway', 'mediagatewayd'), ('api', 'api', 'apid')]:
        binary = output / role
        build_env = owner.env
        owner.env = common
        try:
            owner.run(['go', 'build', '-o', str(binary), './cmd/' + entry], 'joined-build-' + role, cwd=services / service)
        finally:
            owner.env = build_env
        env = {**common, 'PORT': str(ports[role])}
        if role == 'trading-charts':
            owner.run(['psql', database, '-v', 'ON_ERROR_STOP=1', '-c',
                       'CREATE SCHEMA charts_migrations'], 'joined-charts-migration-schema')
            env['DATABASE_URL'] = database + '&search_path=charts_migrations'
        if role == 'stats':
            owner.run(['psql', database, '-v', 'ON_ERROR_STOP=1', '-c',
                       'CREATE SCHEMA stats_migrations'], 'joined-stats-migration-schema')
            migration = output / 'stats-migrate'
            owner.run(['go', 'build', '-o', str(migration), './cmd/stats-migrate'],
                      'joined-build-stats-migrate', cwd=services / 'stats-workers')
            env['DATABASE_URL'] = database + '&search_path=stats_migrations'
            log = owner.output / 'joined-stats-migrate.log'
            with log.open('w') as stream:
                result = subprocess.run([str(migration)], env=env,
                    stdout=stream, stderr=subprocess.STDOUT, timeout=90)
            (owner.output / 'joined-stats-migrate.json').write_text(json.dumps({
                'command': [str(migration)], 'exit': result.returncode, 'log': str(log),
                'executable_sha256': digest(migration)}) + '\n')
            if result.returncode:
                raise RuntimeError(f'stats-migrate: exit {result.returncode}; {log}\n'
                                   + '\n'.join(log.read_text().splitlines()[-40:]))
        if role == 'api':
            env.update(API_RATE_LIMIT_MAX='1000', API_RATE_LIMIT_WINDOW_SEC='60')
        if role != 'gateway':
            env['CORS_ALLOWED_ORIGINS'] = origins['browser']
        if role == 'api':
            env['CORS_ALLOWED_ORIGINS'] = origins['browser'] + ',' + public
        child = owner.start([str(binary)], role, env=env)
        url = origins[role] + '/health/ready'
        ready(owner, child, url)
        if role == 'trading-charts':
            chart_subscription_ready(owner, child, redis_url)
        if role == 'stats':
            stats = child
            forward_ready = output / 'stats-forwarder-ready'
            forwarder = owner.start(['python3', str(web / 'src/widgets/trading/forward-indexer-stats.py')],
                'joined-stats-forwarder', env={**common, 'STATS_WEBHOOK_URL': origins['stats'] + '/webhooks/events',
                    'STATS_FORWARDER_READY': str(forward_ready)})
            deadline = time.monotonic() + 15
            while not forward_ready.exists() and time.monotonic() < deadline:
                if forwarder.poll() is not None:
                    raise owner.readiness_error(forwarder, 'exited before subscribing to indexer events')
                time.sleep(.1)
            if not forward_ready.exists():
                raise owner.readiness_error(forwarder, 'indexer event subscription timed out')
            processes.append({'role': 'stats-forwarder', 'ready_url': str(forward_ready),
                              'pid': forwarder.pid})
        if role == 'indexer':
            pool_state_ready(owner, child, database, fixture['pool'])
        if role == 'api':
            stats_row_ready(owner, stats, forwarder, database, fixture['pool'])
            projection_ready(owner, child, database, fixture['pool'])
            stream_ready(owner, child, origins[role], fixture['pool'], public)
            ready(owner, child, origins[role] + '/v2/pools/' + fixture['pool'].lower())
        processes.append({'role': role, 'ready_url': url, 'pid': child.pid,
                          'executable_sha256': digest(binary), 'env': env})
    browser_env = {**common, 'NODE_ENV': 'development',
        'NEXT_PUBLIC_RPC_URL': fixture['rpcUrl'],
        'NEXT_PUBLIC_DATA_API_URL': origins['api'], 'NEXT_PUBLIC_GATEWAY_URL': origins['gateway'],
        'NEXT_PUBLIC_METADATA_API_URL': origins['gateway'], 'NEXT_PUBLIC_PNL_API_URL': origins['api'],
        'NEXT_PUBLIC_STREAM_ORIGIN': origins['api'].replace('http:', 'ws:'),
        'WEB_API_UPSTREAM': origins['api'], 'WEB_GATEWAY_UPSTREAM': origins['gateway'],
        'WEB_SOCKETS_UPSTREAM': origins['api'], 'WEB_CHARTS_UPSTREAM': origins['trading-charts']}

    browser = owner.start(['pnpm', 'exec', 'next', 'dev', '--turbopack', '--hostname', '127.0.0.1',
                           '--port', str(ports['browser'])], 'joined-browser', cwd=web, env=browser_env)
    ready(owner, browser, origins['browser'], timeout=180)
    ready(owner, browser, origins['browser'] + '/coin/' + fixture['pool'], timeout=180)
    processes.append({'role': 'browser', 'ready_url': origins['browser'], 'pid': browser.pid})
    chain = output / 'chain.json'
    chain.write_text(json.dumps({'realm': 'private-anvil125', 'productionActivation': False,
        'manifest': fixture['authorityDeployment'], 'genesisHash': fixture['genesisHash'],
        'account': fixture['chartAccount'], 'pool': fixture['pool'], 'token': fixture['token']}) + '\n')
    configuration = output / 'configuration.json'
    configuration.write_text(json.dumps({'schema_version': 1, 'browser_origin': origins['browser'],
        'stream_application_origin': public,
        'api_origin': origins['api'], 'rpc_url': fixture['rpcUrl'], 'processes': processes}, indent=2) + '\n')
    browser_configuration = output / 'browser-configuration.json'
    browser_configuration.write_text(json.dumps({'realm': 'private-anvil125',
        'browser_origin': origins['browser'], 'rpc_url': fixture['rpcUrl'],
        'playwright_module': str(web / 'node_modules/playwright')}) + '\n')
    owner.env.update(PHASE2_CONFIG_FILE=str(browser_configuration),
        CHART_BROWSER_OUTPUT_DIR=str(output / 'chart-browser'))
    owner.env.update(JOINED_CONFIG_FILE=str(configuration), JOINED_CONFIG_SHA256=digest(configuration),
        JOINED_CHAIN_FILE=str(chain), JOINED_ALLOW_CHAIN_MUTATION='1')
