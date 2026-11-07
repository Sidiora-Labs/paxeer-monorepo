"""Capture database-peer transcripts; this is fixture authoring, never a test dependency.

Run as a local PostgreSQL role in a disposable database, from the repo root:
  python3 services/platform/perps-data/internal/graphql/testdata/repair_m829/capture.py > .../postgres.json
Requires libpq only for authoring. The Go gate replays the committed transcript.
"""
import ctypes as c
import ctypes.util
import datetime as dt
import json
import os
from pathlib import Path
import pwd
import re
import subprocess

BASE = Path("services/platform/perps-data/internal")
pq = c.CDLL(ctypes.util.find_library("pq"))
for name, restype, args in [
    ("PQconnectdb", c.c_void_p, [c.c_char_p]),
    ("PQstatus", c.c_int, [c.c_void_p]),
    ("PQexec", c.c_void_p, [c.c_void_p, c.c_char_p]),
    ("PQresultStatus", c.c_int, [c.c_void_p]),
    ("PQresultErrorMessage", c.c_char_p, [c.c_void_p]),
    ("PQntuples", c.c_int, [c.c_void_p]),
    ("PQnfields", c.c_int, [c.c_void_p]),
    ("PQfname", c.c_char_p, [c.c_void_p, c.c_int]),
    ("PQftype", c.c_uint, [c.c_void_p, c.c_int]),
    ("PQgetisnull", c.c_int, [c.c_void_p, c.c_int, c.c_int]),
    ("PQgetvalue", c.c_char_p, [c.c_void_p, c.c_int, c.c_int]),
    ("PQclear", None, [c.c_void_p]),
    ("PQfinish", None, [c.c_void_p]),
]:
    f = getattr(pq, name)
    f.restype, f.argtypes = restype, args
capture_uid = os.geteuid()
try:
    if capture_uid == 0:
        os.seteuid(pwd.getpwnam("postgres").pw_uid)
    conn = pq.PQconnectdb(b"port=25432 dbname=postgres")
finally:
    os.seteuid(capture_uid)
assert pq.PQstatus(conn) == 0


def execute(sql):
    result = pq.PQexec(conn, sql.encode())
    try:
        if pq.PQresultStatus(result) not in (1, 2):
            raise RuntimeError(pq.PQresultErrorMessage(result).decode())
        cols = [pq.PQfname(result, i).decode() for i in range(pq.PQnfields(result))]
        types = [pq.PQftype(result, i) for i in range(len(cols))]
        rows = []
        for j in range(pq.PQntuples(result)):
            row = []
            for i, typ in enumerate(types):
                value = None if pq.PQgetisnull(result, j, i) else pq.PQgetvalue(result, j, i).decode()
                if typ == 1184 and value is not None:
                    value = dt.datetime.fromisoformat(value).isoformat().replace("+00:00", "Z")
                row.append(value)
            rows.append(row)
        return {"columns": cols, "types": types, "rows": rows}
    finally:
        pq.PQclear(result)


def source(path):
    return (BASE / path).read_text()


perps = source("graphql/perps.go")
subs = source("graphql/subscriptions.go")
records = source("records/records.go")


def expression(text, env):
    # Only Go's string concatenation expressions in the captured readers.
    text = re.sub(r"`([^`]*)`", lambda m: repr(m[1]), text, flags=re.S)
    text = re.sub(r"\bhexSQL\(", "hex_sql(", text)
    text = text.replace('strings.Join(columns, ", ")', "joined")
    return eval(text.replace("\n", " "), {"__builtins__": {}}, env)


def constant(text, name, env):
    match = re.search(r"(?m)^(?:const |var )?" + name + r" = (.*?)(?=\n\n)", text, re.S)
    return expression(match[1], env)


env = {"hex_sql": lambda col: "'0x' || encode(" + col + ", 'hex')"}
for name in ("newest", "orderlySymbolSQL", "indexPriceRowsSQL", "latestIndexSQL", "marketSQL", "marketStateSQL"):
    env[name] = constant(perps, name, env)
env["latestSQL"] = lambda table, columns: "(SELECT " + columns + " FROM perps." + table + " x WHERE x.market_id = m.market_id ORDER BY x.block DESC, x.tx_index DESC, x.log_index DESC, x.sub DESC LIMIT 1)"


def collect_query(name, code=perps, values=env):
    match = re.search(r'return collect\(ctx, r.DB, "' + name + r'", (.*?), \[\]any\{', code, re.S)
    return expression(match[1], values)


window_expr = re.search(r"columns\[i\] = (.*?)(?=\n\t})", perps, re.S)[1]
env["joined"] = ", ".join(expression(window_expr, {**env, "w": w}) for w in
    ["5 minutes", "30 minutes", "1 hour", "4 hours", "24 hours", "3 days", "7 days", "30 days"])
baseline_revision = "fedbeb3bb47c5bb972f66afc8189f92f5395f3eb"


def baseline_source(name):
    return subprocess.check_output(["git", "show", baseline_revision + ":" + str(BASE / "graphql" / name)], text=True)


baseline_perps = baseline_source("perps.go")
baseline_subs = baseline_source("subscriptions.go")
baseline_env = dict(env)
for name in ("latestMarkSQL", "marketStateSQL"):
    baseline_env[name] = constant(baseline_perps, name, baseline_env)
baseline_window = re.search(r"columns\[i\] = (.*?)(?=\n\t})", baseline_perps, re.S)[1]
baseline_env["joined"] = ", ".join(expression(baseline_window, {**baseline_env, "w": w}) for w in
    ["5 minutes", "30 minutes", "1 hour", "4 hours", "24 hours", "3 days", "7 days", "30 days"])
record_env = {"LastFundingRateHourlySQL": "trim_scale(funding_rate * 0.000000000000000001)::text"}
for name in ("LiquidationFill", "latestMarkSQL", "fillWindowSQL", "marketStateSQL"):
    record_env[name] = constant(records, name, record_env)

anchor = dt.datetime(2026, 11, 4, 7, 0, tzinfo=dt.timezone.utc)


def at(seconds):
    return (anchor + dt.timedelta(seconds=seconds)).isoformat().replace("+00:00", "Z")


def aggregate(market, index, mark=999):
    return {"type": "pricefeed_aggregate", "values": {
        "market_id": str(market), "index_price": str(index * 10**18), "mark_price": str(mark * 10**18),
        "basis": str((mark-index) * 10**18), "sources": "4", "required": "3"}}


def skipped(market):
    return {"type": "pricefeed_aggregate_skipped", "values": {"market_id": str(market), "sources": "1", "required": "3"}}


def block(n, seconds, events=(), marks=()):
    events = json.loads(json.dumps(events))
    for ev in events:
        if ev["type"] == "pricefeed_aggregate":
            ev["values"].update(height=str(n), timestamp_ms=str(int((anchor + dt.timedelta(seconds=seconds-7)).timestamp()*1000)))
    return {"number": n, "time": at(seconds), "events": list(events), "marks": list(marks)}


def mark(index, price, log=0, market=1):
    return {"market": str(market), "index": str(index * 10**18), "mark": str(price * 10**18), "tx": 2, "log": log}


b100 = block(100, -26*3600, [aggregate(2, 200, 250), skipped(3)], [mark(100, 105)])
b101 = block(101, -2*3600, [aggregate(1, 110)])
b102 = block(102, -3600, [skipped(1)])
b103 = block(103, -60, [aggregate(1, 120)], [mark(115, 106)])
b104 = block(104, -30, [aggregate(1, 125), aggregate(1, 127)])
b105 = block(105, -15, [aggregate(1, 0)])
b106 = block(106, -10, [], [mark(130, 107)])
scenarios = [
    ("empty", []), ("mark_and_index_only", [b100]),
    ("index_without_mark", [b100, b101]), ("skipped", [b100, b101, b102]),
    ("same_block_mark", [b100, b101, b102, b103]),
    ("aggregate_log_order", [b100, b101, b102, b103, b104]),
    ("zero_index", [b100, b101, b102, b103, b104, b105]),
    ("new_mark", [b100, b101, b102, b103, b104, b105, b106]),
    ("zero_baseline", [b100, block(101, -2*3600, [aggregate(1, 0)]), b104]),
]


def quote(value):
    return "'" + str(value).replace("'", "''") + "'"


def query(name, sql, args=()):
    # Store the exact request as well as the server's response, with clock
    # parameters tagged explicitly. Runtime readers use their wall clock.
    rendered = re.sub(r"\$(\d+)", lambda m: quote(args[int(m[1])-1]), sql)
    result = execute(rendered)
    result.update(name=name, sql=sql, args=[{"clock": True, "clock_seconds": int((a-anchor).total_seconds())} if isinstance(a, dt.datetime) else {"value": str(a)} for a in args])
    return result


execute("BEGIN; SET TIME ZONE 'UTC'; CREATE SCHEMA IF NOT EXISTS perps")
try:
    # A new transaction-local schema: no persistent database changes.
    for migration in sorted((BASE.parent / "migrations").glob("*.sql")):
        execute(migration.read_text())
    execute("TRUNCATE perps.market_tiers, perps.markets CASCADE")
    for mid, sym in [(1, "BTC"), (2, "ETH"), (3, "NONE")]:
        execute(f"INSERT INTO perps.markets (market_id, symbol, name, oracle_symbol, sz_decimals, px_decimals, max_leverage, maintenance_margin_bps, max_open_interest, enabled, source) VALUES ({mid}, '{sym}', '{sym}', '{sym}', 6, 2, 100, 50, 1000000, true, 'chain')")
    output = {"postgres": execute("SELECT version()"), "anchor": at(0), "baseline_revision": baseline_revision, "scenarios": []}
    for name, blocks in scenarios:
        execute("TRUNCATE perps.mark_prices, perps.index_prices RESTART IDENTITY")
        for b in blocks:
            common = {"block": b["number"], "block_hash": "\\x" + format(b["number"], "064x"),
                "block_time": b["time"], "sub": 0, "account": "\\x" + "00"*20}
            for i, ev in enumerate(b["events"]):
                vals = {**common, "tx_index": -1, "log_index": i, "tx_hash": common["block_hash"],
                    "receipt_kind": "block_event", "emitter": "\\x" + "00"*18 + "1020",
                    "kind": "aggregate" if ev["type"] == "pricefeed_aggregate" else "skipped",
                    **{k:v for k,v in ev["values"].items() if k != "height"}}
                # Account is the same module address the real decoder uses.
                import hashlib
                vals["account"] = "\\x" + hashlib.sha256(b"pricefeed").digest()[:20].hex()
                execute("INSERT INTO perps.index_prices (" + ",".join(vals) + ") VALUES (" + ",".join(quote(v) for v in vals.values()) + ")")
            for m in b["marks"]:
                vals = {**common, "tx_index": m["tx"], "log_index": m["log"], "tx_hash": "\\x" + format(b["number"]*100+m["tx"], "064x"),
                    "receipt_kind": "perps_endblock", "emitter": "\\x" + "00"*18 + "1015", "kind": "mark",
                    "market_id": m["market"], "mark_price": m["mark"], "index_price": m["index"], "mid_price": m["index"]}
                vals["account"] = "\\x" + hashlib.sha256(b"perps").digest()[:20].hex()
                execute("INSERT INTO perps.mark_prices (" + ",".join(vals) + ") VALUES (" + ",".join(quote(v) for v in vals.values()) + ")")
        qs = []
        qs.append(query("latest", env["latestIndexSQL"]+"ORDER BY market_id, "+env["newest"]))
        qs.append(query("markets", env["marketSQL"]+"ORDER BY market_id"))
        qs.append(query("market", env["marketSQL"]+"WHERE market_id = $1::numeric", [1]))
        for ids in ("{1}", "{1,2,3}"):
            qs.append(query("states", env["marketStateSQL"], [ids]))
        qs.append(query("pool", env["latestIndexSQL"]+"WHERE market_id = $1::numeric ORDER BY market_id, "+env["newest"], [1]))
        qs.append(query("tickers", collect_query("orderlyTickers"), [anchor-dt.timedelta(hours=24)]))
        qs.append(query("changes", collect_query("orderlyPriceChanges"), [anchor]))
        for const in ("latestPricesSQL", "latestPriceCursorSQL"):
            qs.append(query(const, constant(subs, const, env)))
        qs.append(query("subMarketSQL", constant(subs, "subMarketSQL", env), ["1"]))
        qs.append(query("symbolIDs", "SELECT symbol, market_id::text FROM perps.markets"))
        qs.append(query("feedIDs", "SELECT market_id::text, symbol FROM perps.markets ORDER BY market_id"))
        for mid in (1, 2, 3):
            qs.append(query("recordsPrice", record_env["latestMarkSQL"], [str(mid), "1000000000000000000"]))
            qs.append(query("recordsWindow", record_env["fillWindowSQL"], [str(mid), anchor-dt.timedelta(hours=24), "1000000000000000000"]))
            qs.append(query("recordsState", record_env["marketStateSQL"], [str(mid), "1000000000000000000"]))
        # Preserve original requests too: a regression receives the original
        # server response and fails on its observable stale/absent price.
        qs.append(query("baseline_latest", baseline_env["latestMarkSQL"]+"ORDER BY market_id, "+env["newest"]))
        for ids in ("{1}", "{1,2,3}"):
            qs.append(query("baseline_states", baseline_env["marketStateSQL"], [ids]))
        qs.append(query("baseline_pool", baseline_env["latestMarkSQL"]+"WHERE market_id = $1::numeric ORDER BY market_id, "+env["newest"], [1]))
        qs.append(query("baseline_tickers", collect_query("orderlyTickers", baseline_perps, baseline_env), [anchor-dt.timedelta(hours=24)]))
        qs.append(query("baseline_changes", collect_query("orderlyPriceChanges", baseline_perps, baseline_env), [anchor]))
        qs.append(query("baseline_subscription", constant(baseline_subs, "latestPricesSQL", baseline_env)))
        qs.append(query("baseline_submarket", constant(baseline_subs, "subMarketSQL", baseline_env), ["1"]))
        qs.append(query("baseline_cursor", "SELECT COALESCE(max(row_id), 0) FROM perps.mark_prices"))
        facts = execute("SELECT 'mark_prices' AS table_name, market_id::text, index_price::text, mark_price::text, block, tx_index, log_index, sub, kind, block_time, NULL::text AS timestamp_ms, NULL::text AS basis, NULL::text AS sources, NULL::text AS required FROM perps.mark_prices UNION ALL SELECT 'index_prices', market_id::text, index_price::text, mark_price::text, block, tx_index, log_index, sub, kind, block_time, timestamp_ms::text, basis::text, sources::text, required::text FROM perps.index_prices ORDER BY block, tx_index, log_index, sub")
        output["scenarios"].append({"name": name, "blocks": blocks, "facts": facts, "queries": qs})
    print(json.dumps(output, indent=2))
finally:
    execute("ROLLBACK")
    pq.PQfinish(conn)
