#!/usr/bin/env python3
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import urllib.error
import urllib.request

TOOLCHAIN_IMAGE = "task23-1-toolchain:local"
POSTGRES_IMAGE = "postgres:15-alpine"
CASES = ("history-readiness", "unified-pagination")


class MissingPrerequisite(Exception):
    pass


def require(condition, message):
    if not condition:
        raise AssertionError(message)


KERNEL_ROOT = Path(__file__).resolve().parents[2]


def prerequisites():
    missing = []
    for tool in ("docker", "openssl"):
        if shutil.which(tool) is None:
            missing.append(tool)
    if importlib.util.find_spec("cryptography") is None:
        missing.append("python3 cryptography")
    for name in ("deps", "_build"):
        if not (KERNEL_ROOT / "services/explorer/backend" / name).is_dir():
            missing.append("prebuilt services/explorer/backend/" + name)
    if shutil.which("docker"):
        for image in (TOOLCHAIN_IMAGE, POSTGRES_IMAGE):
            if subprocess.run(["docker", "image", "inspect", image], stdout=subprocess.DEVNULL,
                              stderr=subprocess.DEVNULL).returncode:
                missing.append("docker image " + image)
        for tool in ("mix", "elixir", "erl"):
            probe = subprocess.run(["docker", "run", "--rm", TOOLCHAIN_IMAGE, "sh", "-c", "command -v " + tool],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if probe.returncode:
                missing.append(f"{tool} in {TOOLCHAIN_IMAGE}")
        probe = subprocess.run(["docker", "run", "--rm", POSTGRES_IMAGE, "sh", "-c", "command -v psql"],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if probe.returncode:
            missing.append(f"psql in {POSTGRES_IMAGE}")
    if missing:
        raise MissingPrerequisite(", ".join(missing))


def history_readiness_environment(env, database, port, evm_url):
    clean_env = {key: value for key, value in env.items() if not key.startswith("LAYERX_INDEXER_")}
    return dict(clean_env, LAYERX_INDEXER_DB=str(database),
                LAYERX_INDEXER_LISTEN=f"127.0.0.1:{port}",
                LAYERX_INDEXER_EVM_URL=evm_url,
                LAYERX_INDEXER_POLL_MS="100", LAYERX_INDEXER_TIMEOUT_MS="400",
                LAYERX_INDEXER_STALL_SECS="5", LAYERX_INDEXER_MAX_UNITS_PER_STEP="1")


def history_readiness():
    import re
    import sqlite3
    import socket
    import urllib.parse

    repo = Path(__file__).resolve().parents[2]
    build = Path(os.environ.get("LAYERX_BUILD_DIR", repo / "build")).resolve()
    binaries = {}
    for key, name in (("LAYERX_INDEXER_BIN", "layerx-indexer"),
                      ("LAYERX_GATEWAY_BIN", "layerx-gateway")):
        path = Path(os.environ.get(key, repo / "services/platform/target/debug" / name)).resolve()
        if not path.is_file() or not os.access(path, os.X_OK):
            raise MissingPrerequisite(f"{key}: prebuilt {name} required")
        binaries[name] = path
    for tool in ("openssl", "redis-server"):
        if shutil.which(tool) is None:
            raise MissingPrerequisite(tool)
    if importlib.util.find_spec("cryptography") is None:
        raise MissingPrerequisite("Python cryptography required")
    fixture_path = os.environ.get("LAYERX_HISTORY_RELAY_FIXTURE_CONFIG")
    if not fixture_path or not Path(fixture_path).is_file():
        raise MissingPrerequisite("LAYERX_HISTORY_RELAY_FIXTURE_CONFIG: dedicated real relay trust pins and idle loopback upstreams required")
    fixture = json.loads(Path(fixture_path).read_text())
    for field in ("network_id", "genesis_sha256", "sequencer_id", "sequencer_public_key", "sequencer_first_batch"):
        require(field in fixture, "real relay fixture is missing " + field)
    upstreams = fixture.get("upstreams")
    require(isinstance(upstreams, list) and upstreams, "real relay fixture has no upstreams")
    for origin in upstreams:
        parsed = urllib.parse.urlparse(origin)
        require(parsed.scheme in ("http", "https") and parsed.hostname in ("localhost", "127.0.0.1", "::1")
                and parsed.username is None and parsed.password is None,
                "history-readiness requires isolated loopback relay upstreams")
    saved = sys.argv
    sys.argv = [str(repo / "tests/relay-archive/e2e.py"), str(build)]
    try:
        spec = importlib.util.spec_from_file_location("history_relay_e2e", repo / "tests/relay-archive/e2e.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
    finally:
        sys.argv = saved
    relay = module.Scenario()
    checks = 0
    reservations = []
    try:
        relay.work.chmod(0o700)
        relay.bin = build / "bin"
        relay.runtime = repo / "services/platform/relay_archive"
        relay_port = module.unused_port()
        allowed = ("network_id", "genesis_sha256", "sequencer_id", "sequencer_public_key",
                   "sequencer_first_batch", "sequencer_last_batch", "upstreams", "ca_file")
        settings = {key: value for key, value in fixture.items() if key in allowed}
        if settings.get("ca_file"):
            settings["ca_file"] = str((Path(fixture_path).resolve().parent / settings["ca_file"]).resolve())
        settings.update(data_dir=str(relay.work / "history-relay-data"),
                        listen=f"127.0.0.1:{relay_port}", public_url=f"http://127.0.0.1:{relay_port}",
                        codec=str(relay.bin / "layerx-archive-codec"), allow_loopback_dev=True,
                        poll_interval_seconds=0.1)
        config_path = relay.work / "history-relay.json"
        module.write_json(config_path, settings)

        def start_relay(name):
            process = relay.start(name, [relay.bin / "layerxd", "--relay-archive", config_path],
                                  env=dict(os.environ, LAYERX_RELAY_ARCHIVE_RUNTIME=str(relay.runtime / "runtime.py")))
            relay.until(lambda: relay.request(settings["public_url"], "/v1/sync/head").get("head_batch"),
                        "real relay source head", 90)
            return process

        source = start_relay("history-relay")
        module.execute(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", relay.work / "tls.key", "-out", relay.work / "tls.crt",
                        "-days", "1", "-subj", "/CN=localhost",
                        "-addext", "subjectAltName=IP:127.0.0.1,DNS:localhost"])
        (relay.work / "tls.key").chmod(0o600)
        for _ in range(3):
            sock = socket.socket()
            sock.bind(("127.0.0.1", 0))
            reservations.append(sock)
        indexer_port, gateway_port, redis_port = [s.getsockname()[1] for s in reservations]
        indexer_url = f"http://127.0.0.1:{indexer_port}"
        gateway_url = f"http://127.0.0.1:{gateway_port}"
        env = {key: value for key, value in os.environ.items()
               if not key.startswith(("LAYERX_INDEXER_", "LAYERX_GATEWAY_", "LAYERX_EVENTS_"))}
        database = relay.work / "history.sqlite"
        indexer_env = history_readiness_environment(env, database, indexer_port, settings["public_url"])
        reservations[0].close()
        indexer = relay.start("history-indexer", [binaries["layerx-indexer"]], env=indexer_env)
        ca = relay.work / "gateway-ca.der"
        module.execute(["openssl", "x509", "-in", relay.work / "tls.crt", "-outform", "DER", "-out", ca])
        username = relay.work / "redis-username"
        password = relay.work / "redis-password"
        username.write_text("default")
        password.write_text(os.urandom(24).hex())
        username.chmod(0o600)
        password.chmod(0o600)
        redis_config = relay.work / "redis.conf"
        redis_config.write_text(f"bind 127.0.0.1\nport 0\ntls-port {redis_port}\n"
                               f"tls-cert-file {relay.work / 'tls.crt'}\n"
                               f"tls-key-file {relay.work / 'tls.key'}\n"
                               f"tls-ca-cert-file {relay.work / 'tls.crt'}\n"
                               "tls-auth-clients no\nsave \"\"\nappendonly no\n"
                               f"requirepass {password.read_text()}\n")
        redis_config.chmod(0o600)
        reservations[2].close()
        redis = relay.start("history-redis", ["redis-server", redis_config])
        protocol_source = (repo / "services/agent/crates/layerx-wire/src/limits.rs").read_text()
        protocol = re.search(r"STATE_COMMITMENT_PROTOCOL_VERSION: u16 = (\d+);", protocol_source)
        require(protocol is not None, "current wire protocol is missing")
        gateway_env = dict(env, LAYERX_GATEWAY_LISTEN=f"127.0.0.1:{gateway_port}",
                           LAYERX_GATEWAY_LISTENER="plain", LAYERX_GATEWAY_OUTBOUND_CA_DER=str(ca),
                           LAYERX_GATEWAY_REDIS_URL=f"rediss://localhost:{redis_port}",
                           LAYERX_GATEWAY_REDIS_USERNAME_FILE=str(username),
                           LAYERX_GATEWAY_REDIS_PASSWORD_FILE=str(password),
                           LAYERX_GATEWAY_NETWORK_ID="history-readiness",
                           LAYERX_GATEWAY_PROTOCOL_NETWORK_ID=str(fixture["network_id"]),
                           LAYERX_GATEWAY_LXP_WIRE_VERSION=protocol.group(1),
                           LAYERX_GATEWAY_INDEXER_URL=indexer_url)
        reservations[1].close()
        gateway = relay.start("history-gateway", [binaries["layerx-gateway"]], env=gateway_env)

        def get(base, path):
            for child in (indexer, gateway, redis):
                require(child.poll() is None, f"required child exited: {child.returncode}; {relay.work}")
            try:
                response = urllib.request.urlopen(base + path, timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                return response.status, json.loads(response.read())

        def ready(state):
            status, doc = get(indexer_url, "/readyz")
            require(doc.get("version") == 1 and doc.get("database") == "ok", "readiness lacks database liveness")
            sources = doc.get("sources")
            require(isinstance(sources, list) and len(sources) == 1 and sources[0]["source"] == "layerx",
                    "readiness omitted required LayerX source")
            if doc["status"] != state:
                return None
            require(status == (200 if state == "ready" else 503), "readiness HTTP status contradicts document")
            return sources[0]

        def gateway_state(state):
            status, doc = get(gateway_url, "/v1/status")
            return status == 200 and doc.get("services", {}).get("indexer") == state

        first = relay.until(lambda: ready("ready"), "fresh source ready")
        head = module.Scenario.request(relay, settings["public_url"], "/v1/sync/head")["head_batch"]
        require(first["reconciled"] and first["source_head"] == int(head)
                and first["indexed_cursor"] == int(head), "source head and persisted cursor differ")
        relay.until(lambda: gateway_state("available"), "gateway preserves available readiness")
        checks += 1
        idle = relay.until(lambda: (s if (s := ready("ready")) and
                                    s["last_success_at"] > first["last_success_at"] else None), "idle head observed")
        require(idle["indexed_cursor"] == first["indexed_cursor"] and idle["source_head"] == first["source_head"],
                "idle proof unexpectedly advanced the source")
        checks += 1
        with sqlite3.connect(database) as connection:
            before = connection.execute("SELECT chain, position, hash FROM cursors ORDER BY chain").fetchall()
            rows_before = connection.execute("SELECT count(*) FROM transfers").fetchone()[0]
            require(rows_before > 0, "real relay fixture requires at least one transfer receipt")
            events_before = connection.execute("SELECT count(*) FROM events").fetchone()[0]
            require(events_before > 0, "real relay produced no indexed receipt rows")
        relay.stop(source)
        failed = relay.until(lambda: (s if (s := ready("degraded")) and s["consecutive_failures"] >= 3 else None),
                             "repeated source failure")
        require(failed["last_error"] == "source_unavailable", "missing typed source failure")
        relay.until(lambda: gateway_state("degraded"), "gateway preserves degraded readiness")
        require(get(indexer_url, "/healthz")[0] == 200, "database liveness fell with disconnected source")
        checks += 1
        stalled = relay.until(lambda: ready("unavailable"), "stalled source unavailable")
        require(stalled["state"] == "stalled" and stalled["freshness_secs"] > 5,
                "stalled source lacks stale freshness")
        relay.until(lambda: gateway_state("unavailable"), "gateway preserves stalled readiness")
        checks += 1
        with sqlite3.connect(database) as connection:
            account = connection.execute("SELECT account FROM transfers ORDER BY id LIMIT 1").fetchone()
        require(account is not None, "real relay fixture has no history account")
        account = account[0]
        status, page = get(indexer_url, "/v1/history/" + account)
        require(status == 200 and page["items"] and page["freshness"]["status"] == "unavailable",
                "stale rows lack source freshness")
        checks += 1
        relay.stop(indexer)
        indexer = relay.start("history-indexer-restarted", [binaries["layerx-indexer"]], env=indexer_env)
        starting = relay.until(lambda: ready("unavailable"), "restart refuses persisted-only readiness")
        require(starting["state"] == "starting" and not starting["reconciled"]
                and starting["indexed_cursor"] == first["indexed_cursor"]
                and starting["last_success_at"] == stalled["last_success_at"],
                "restart reused persisted observation as fresh authority")
        require(get(indexer_url, "/healthz")[0] == 200, "restarted database is not live")
        with sqlite3.connect(database) as connection:
            require(connection.execute("SELECT chain, position, hash FROM cursors ORDER BY chain").fetchall() == before,
                    "restart changed persisted cursor")
            require(connection.execute("SELECT count(*) FROM transfers").fetchone()[0] == rows_before,
                    "disconnected restart changed persisted rows")
            require(connection.execute("SELECT count(*) FROM events").fetchone()[0] == events_before,
                    "disconnected restart changed indexed receipt rows")
        checks += 1
        source = start_relay("history-relay-restarted")
        recovered = relay.until(lambda: ready("ready"), "fresh source reconciles restart")
        require(recovered["reconciled"] and recovered["consecutive_failures"] == 0
                and recovered["source_head"] == first["source_head"]
                and recovered["last_success_at"] > starting["last_success_at"], "reconnect was not freshly observed")
        relay.until(lambda: gateway_state("available"), "gateway recovers after fresh observation")
        checks += 1
        relay.stop(indexer)
        empty_env = {key: value for key, value in indexer_env.items() if key != "LAYERX_INDEXER_EVM_URL"}
        indexer = relay.start("history-indexer-unconfigured", [binaries["layerx-indexer"]], env=empty_env)
        relay.until(lambda: indexer.poll() is not None, "missing ingestion source refused at startup")
        require(indexer.returncode != 0, "missing ingestion source was accepted")
        refusal = (relay.work / "history-indexer-unconfigured.log").read_text()
        require("layerx-indexer: configuration invalid: set LAYERX_INDEXER_RELAY_URL, "
                "LAYERX_INDEXER_EVM_URL or both" in refusal,
                "missing EVM source did not produce the expected configuration refusal")
        doc = relay.request(gateway_url, "/v1/status")
        require(doc["services"]["indexer"] == "unavailable", "gateway accepted missing indexer source")
        checks += 1
        return checks
    finally:
        for reservation in reservations:
            reservation.close()
        relay.close()


def unified_pagination_required(name):
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"missing real gate prerequisite: {name}")
    return value


def unified_pagination_local_url(value):
    import urllib.parse

    parsed = urllib.parse.urlsplit(value)
    if parsed.scheme not in ("http", "https", "postgres", "postgresql") or parsed.hostname not in ("127.0.0.1", "localhost", "::1"):
        raise RuntimeError("gate endpoints must be disposable loopback services")
    return parsed


def unified_pagination():
    import secrets
    import urllib.parse

    pagination_root = Path(__file__).resolve().parents[2]
    backend = unified_pagination_required("PAXEER_X_GATE_BACKEND_URL").rstrip("/")
    frontend = unified_pagination_required("PAXEER_X_GATE_FRONTEND_URL").rstrip("/")
    unified_pagination_local_url(backend)
    unified_pagination_local_url(frontend)
    db = unified_pagination_local_url(unified_pagination_required("PAXEER_X_GATE_DATABASE_URL"))
    database = db.path.lstrip("/")
    if not database.startswith("paxeer_x_gate_23_1"):
        raise RuntimeError("refusing any database outside the task's disposable namespace")
    for executable in ("psql", "node", "mix"):
        if not shutil.which(executable):
            raise RuntimeError(f"missing pinned gate dependency: {executable}")
    secret = unified_pagination_required("PAXEER_X_GATE_SECRET_KEY_BASE")
    pg_env = {**os.environ, "PGHOST": db.hostname, "PGPORT": str(db.port or 5432),
              "PGDATABASE": database, "PGUSER": urllib.parse.unquote(db.username or "postgres"),
              "PGPASSWORD": urllib.parse.unquote(db.password or "")}

    def sql(statement):
        result = subprocess.run(["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1"], input=statement,
                                text=True, capture_output=True, env=pg_env, timeout=30)
        if result.returncode:
            raise RuntimeError("real fixture SQL failed: " + result.stderr[-1500:])
        return result.stdout.strip()

    def api(address, params=None, expected_status=200):
        url = f"{backend}/api/v2/addresses/{address}/unified"
        if params:
            url += "?" + urllib.parse.urlencode(params)
        try:
            with urllib.request.urlopen(url, timeout=30) as response:
                status, payload = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, payload = error.code, error.read()
        assert status == expected_status, f"API status {status}; expected {expected_status}"
        return json.loads(payload)

    account, other, contract = ["0x" + secrets.token_hex(20) for _ in range(3)]
    block_hashes = [secrets.token_hex(32) for _ in range(122)]
    tx_hashes = [secrets.token_hex(32) for _ in range(122)]
    base = int(sql("SELECT COALESCE(MAX(number),0)+1000 FROM blocks;"))
    bytea = lambda value: "decode('" + value.removeprefix("0x") + "','hex')"
    for address in (account, other, contract):
        sql(f"INSERT INTO addresses(hash,inserted_at,updated_at) VALUES ({bytea(address)},now(),now());")
    sql(f"INSERT INTO tokens(contract_address_hash,type,symbol,decimals,inserted_at,updated_at) VALUES ({bytea(contract)},'ERC-20','PAGE',18,now(),now());")

    def insert_transaction(n, consensus=True):
        block_hash, tx_hash = block_hashes[n-1], tx_hashes[n-1]
        sql(f"""BEGIN;
        INSERT INTO blocks(hash,number,consensus,parent_hash,miner_hash,nonce,size,gas_limit,gas_used,timestamp,inserted_at,updated_at)
        VALUES ({bytea(block_hash)},{base+n},{str(consensus).lower()},{bytea(secrets.token_hex(32))},{bytea(other)},decode('0000000000000000','hex'),1,30000000,21000,now(),now(),now());
        INSERT INTO transactions(hash,block_hash,block_number,block_consensus,block_timestamp,"index",from_address_hash,to_address_hash,gas,gas_price,gas_used,cumulative_gas_used,input,nonce,r,s,v,value,status,inserted_at,updated_at)
        VALUES ({bytea(tx_hash)},{bytea(block_hash)},{base+n},{str(consensus).lower()},now(),0,{bytea(account)},{bytea(other)},21000,1,21000,21000,decode('','hex'),{n},1,1,27,1,1,now(),now());
        COMMIT;""")

    for n in range(1,121):
        insert_transaction(n)
    tie = 71
    sql(f"""INSERT INTO token_transfers(transaction_hash,block_hash,block_number,block_consensus,log_index,from_address_hash,to_address_hash,token_contract_address_hash,token_type,amount,inserted_at,updated_at)
    VALUES ({bytea(tx_hashes[tie-1])},{bytea(block_hashes[tie-1])},{base+tie},true,0,{bytea(account)},{bytea(other)},{bytea(contract)},'ERC-20',1,now(),now());
    INSERT INTO lx_custody_events(transaction_hash,block_hash,block_number,block_consensus,log_index,kind,direction,address_hash,amount,inserted_at,updated_at)
    VALUES ({bytea(tx_hashes[tie-1])},{bytea(block_hashes[tie-1])},{base+tie},true,0,'custody_deposit','deposit',{bytea(account)},1,now(),now());""")
    expected = [(base+n,0,"transaction","0x"+tx_hashes[n-1]) for n in range(1,121)]
    expected += [(base+tie,0,kind,"0x"+tx_hashes[tie-1]) for kind in ("token_transfer","custody_deposit")]
    expected.sort(reverse=True)
    key = lambda row: (row["block_number"],row["ordinal"],row["kind"],row["hash"].lower())
    first = api(account)
    assert len(first["activity"]) == 50 and first["next_page_params"]
    assert [key(row) for row in first["activity"]] == expected[:50]
    assert first["page_number"] == 1 and first["activity_total"] is None
    assert api(other)["activity_total"] is None
    no_history = api("0x" + secrets.token_hex(20))
    assert no_history["activity_total"] == 0 and no_history["next_page_params"] is None
    insert_transaction(121)
    insert_transaction(122, consensus=False)
    pages, rows, current = [], [], first
    for page_number in range(1,10):
        assert current["page_number"] == page_number
        page_rows = [key(row) for row in current["activity"]]
        pages.append(page_rows)
        rows.extend(page_rows)
        if current["next_page_params"] is None:
            break
        current = api(account,current["next_page_params"])
    assert rows == expected, "history skipped, duplicated, reordered or admitted a concurrent row"
    assert len(rows) == len(set(rows)) == 122
    assert [key(row) for row in api(account,{"cursor":first["page_cursor"]})["activity"]] == expected[:50]
    latest = api(account)
    assert latest["activity"][0]["hash"].lower() == "0x"+tx_hashes[120]
    assert all(row["hash"].lower() != "0x"+tx_hashes[121] for row in latest["activity"])
    for address, params in [(account,{"cursor":"malformed"}), (account,{"cursor":""}),
                            (account,{"cursor[]":"malformed"}),
                            (account,{**first["next_page_params"],"index":"0"}),
                            (other,first["next_page_params"]),
                            (account,{"block_number":str(base+tie),"index":"0"})]:
        assert api(address,params,422)["message"] == "Invalid activity cursor"

    signer = r'''
    Application.ensure_all_started(:crypto)
    key = System.fetch_env!("PAXEER_X_GATE_SECRET_KEY_BASE")
    {:ok, state} = Phoenix.Token.verify(key, "paxeer-x-unified-account-v1", System.fetch_env!("PAXEER_X_GATE_CURSOR"), max_age: 3600)
    token = Phoenix.Token.sign(key, "paxeer-x-unified-account-v1", state, signed_at: System.system_time(:second) - 3601)
    IO.puts("EXPIRED_CURSOR=" <> token)
    '''
    signed = subprocess.run(["mix","run","--no-compile","--no-deps-check","--no-start","-e",signer],
                            cwd=pagination_root/"services/explorer/backend", env={**os.environ,"PAXEER_X_GATE_SECRET_KEY_BASE":secret,
                            "PAXEER_X_GATE_CURSOR":first["page_cursor"]},capture_output=True,text=True,timeout=30)
    if signed.returncode:
        raise RuntimeError("real Phoenix cursor expiry setup failed")
    expired = next((line.split("=",1)[1] for line in signed.stdout.splitlines() if line.startswith("EXPIRED_CURSOR=")),None)
    assert expired and api(account,{"cursor":expired},422)["reason"] == "expired"

    browser = r'''
    const { chromium } = require('@playwright/test');
    const assert = require('node:assert/strict');
    const fs = require('node:fs');
    const input = JSON.parse(fs.readFileSync(0, 'utf8'));
    (async () => {
      const browser = await chromium.launch({ headless: true });
      try {
        const page = await browser.newPage();
        const seen = [];
        page.on('request', request => {
          if (request.url().includes('/unified?')) seen.push(request.url());
        });
        const url = input.frontend + '/paxeer-x/account/' + input.account + '?tab=activity&cursor=' + encodeURIComponent(input.cursor);
        await page.goto(url);
        const hashes = () => page.locator('[data-label="paxeer-x-activity"] [data-activity]').evaluateAll(rows => rows.map(row => row.getAttribute('data-activity').toLowerCase()));
        const waitPage = async (number, expected) => {
          await page.locator('[data-control="page"]').filter({hasText: `Page ${number}`}).waitFor();
          await page.waitForFunction(expected => {
            const rows = [...document.querySelectorAll('[data-label="paxeer-x-activity"] [data-activity]')].map(row => row.getAttribute('data-activity').toLowerCase());
            return JSON.stringify(rows) === JSON.stringify(expected);
          }, expected);
          assert.deepEqual(await hashes(), expected);
        };
        const expected = input.pages.map(rows => rows.map(row => row[3]));
        await waitPage(1, expected[0]);
        await page.getByText('50 activity entries on this page', {exact:true}).waitFor();
        await page.getByRole('button', {name:'Next page', exact:true}).click();
        await waitPage(2, expected[1]);
        const pageTwo = page.url();
        assert.notEqual(new URL(pageTwo).searchParams.get('cursor'), input.cursor);
        await page.reload();
        await waitPage(2, expected[1]);
        assert.equal(page.url(), pageTwo);
        await page.goBack();
        await waitPage(1, expected[0]);
        await page.goForward();
        await waitPage(2, expected[1]);
        await page.getByRole('button', {name:'Next page', exact:true}).click();
        await waitPage(3, expected[2]);
        assert.equal(await page.getByRole('button', {name:'Next page', exact:true}).isDisabled(), true);
        assert(seen.some(url => new URL(url).searchParams.get('cursor') === new URL(pageTwo).searchParams.get('cursor')));
        await page.goto(input.frontend + '/paxeer-x/account/' + input.account + '?tab=activity&cursor=malformed');
        await page.getByRole('alert').filter({hasText:'Unable to load this history page'}).waitFor();
        assert.equal(await page.locator('[data-label="paxeer-x-activity"]').count(), 0);
        await page.goto(input.frontend + '/paxeer-x/account/' + input.account + '?tab=activity&cursor=');
        await page.getByRole('alert').filter({hasText:'Unable to load this history page'}).waitFor();
        assert.equal(new URL(page.url()).searchParams.get('cursor'), '');
        assert.equal(await page.locator('[data-label="paxeer-x-activity"]').count(), 0);
        await page.getByRole('button',{name:'Start a new history view',exact:true}).click();
        await page.locator('[data-label="paxeer-x-activity"]').waitFor();
        console.log(JSON.stringify({browser:'passed',pages:expected.length,reload:true,back_forward:true,refusal:true}));
      } finally { await browser.close(); }
    })().catch(error => { console.error(error.message); process.exit(1); });
    '''
    result = subprocess.run(["node","-e",browser],cwd=pagination_root/"apps/explorer",text=True,
                            input=json.dumps({"frontend":frontend,"account":account,"cursor":first["page_cursor"],"pages":pages}),timeout=180)
    assert result.returncode == 0, "real browser pagination gate failed"
    sql(f"UPDATE blocks SET consensus=false WHERE hash={bytea(block_hashes[119])};")
    assert api(account,{"cursor":first["page_cursor"]},422)["message"] == "Invalid activity cursor"
    checks = ["real_database", "real_api", "full_key_ties", "concurrent_insert", "consensus_filter", "reorg_refusal",
              "malformed_cursor", "expired_cursor", "wrong_account", "real_browser", "reload", "back_forward"]
    print(json.dumps({"case":"unified-pagination","status":"passed","rows":122,"pages":len(pages),
                      "checks":checks,"skips":0}))
    return len(checks)


def main():
    parser = argparse.ArgumentParser(description="Paxeer X explorer data-boundary gates")
    parser.add_argument("--case", required=True, choices=CASES)
    args = parser.parse_args()
    if args.case == "unified-pagination":
        try:
            tests = unified_pagination()
        except (AssertionError, RuntimeError, OSError, subprocess.SubprocessError, urllib.error.URLError) as error:
            print(json.dumps({"case":"unified-pagination","status":"failed","reason":str(error),"skips":0}), file=sys.stderr)
            return 1
        print(f"PAXEER_X_GATE tests={tests} skipped=0", flush=True)
        return 0
    if args.case == "history-readiness":
        try:
            tests = history_readiness()
        except MissingPrerequisite as error:
            print("missing prerequisite: " + str(error), file=sys.stderr)
            return 3
        except AssertionError as error:
            print("FAIL " + str(error), file=sys.stderr)
            return 1
        print(f"PAXEER_X_GATE tests={tests} skipped=0", flush=True)
        return 0


if __name__ == "__main__":
    sys.exit(main())
