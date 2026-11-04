#!/usr/bin/env python3
"""Persistent database sessions for the Blockscout copy tool.

The destination advisory lock and every destination write share one connection.
The source connection keeps one exported repeatable-read snapshot until shutdown.
An interrupted boundary is resumed only if each previously selected table still
has the same keys and copied values. A completed boundary permits a new cut.
"""

import contextlib
import hashlib
import json
import os
import signal
import socket
import subprocess
import sys
import uuid


LOCK_SQL = "SELECT pg_try_advisory_lock(1885431141, 1668247673)"
RUN_TABLE = 'public."paxeer_x_copy_run"'
PROGRESS = 'public."paxeer_x_copy_progress"'


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def ident(value):
    return '"' + value.replace('"', '""') + '"'


class CopyError(RuntimeError):
    pass


class PsqlSession:
    """Line-framed psql transport; readers/writers may also be transcript peers."""

    def __init__(self, reader, writer, process=None):
        self.reader = reader
        self.writer = writer
        self.process = process
        self.failed = False

    @classmethod
    def connect(cls, url):
        process = subprocess.Popen(
            ["psql", "--no-psqlrc", "--quiet", "--tuples-only", "--no-align",
             "--set", "ON_ERROR_STOP=1", "--dbname", url],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True,
            encoding="utf-8", bufsize=1,
        )
        session = cls(process.stdout, process.stdin, process)
        session.query("SET TIME ZONE 'UTC'; SET DateStyle = 'ISO, YMD'; "
                      "SET IntervalStyle = 'postgres'; SET extra_float_digits = 3; "
                      "SET bytea_output = 'hex'")
        return session

    def _send(self, sql, rows=None):
        if self.failed:
            raise CopyError("database session is no longer usable")
        marker = "paxeer_copy_" + uuid.uuid4().hex
        try:
            self.writer.write(sql.rstrip().rstrip(";") + ";\n")
            if rows is not None:
                for row in rows:
                    self.writer.write(row)
                self.writer.write("\\.\n")
            self.writer.write("\\echo " + marker + "\n")
            self.writer.flush()
        except (OSError, ValueError):
            self.failed = True
            raise CopyError("database session lost while sending a request") from None
        return marker

    def _receive(self, marker):
        while True:
            line = self.reader.readline()
            if not line:
                self.failed = True
                raise CopyError("database session lost before command completion")
            if line.rstrip("\n") == marker:
                return
            yield line

    def stream(self, sql):
        return self._receive(self._send(sql))

    def query(self, sql):
        return "".join(self.stream(sql)).rstrip("\n")

    def copy_from(self, sql, rows):
        return "".join(self._receive(self._send(sql, rows))).rstrip("\n")

    def close(self):
        # Closing a connection releases its advisory lock and snapshot. Never
        # reconnect transparently: that would let a run continue without ownership.
        with contextlib.suppress(OSError, ValueError):
            self.writer.close()
        if self.process is not None:
            try:
                self.process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self.process.terminate()
                self.process.wait(timeout=2)
        self.reader.close()


def fingerprint(session, table, projection, predicate):
    # COPY's text encoding escapes embedded newlines and NULLs unambiguously.
    # Sort in the database so the digest is independent of scan/batch order.
    # Every copied column, including all identity columns, participates.
    sql = ("COPY (SELECT row_to_json(c)::text AS row_content FROM (SELECT "
           f"{projection} FROM public.{ident(table)} {predicate}) AS c "
           'ORDER BY row_to_json(c)::text COLLATE "C") TO STDOUT')
    digest = hashlib.sha256()
    count = 0
    for row in session.stream(sql):
        digest.update(row.encode("utf-8"))
        count += 1
    return {"count": count, "digest": digest.hexdigest()}


class CopyRun:
    def __init__(self, source, destination, source_identity):
        self.source = source
        self.destination = destination
        self.source_identity = source_identity
        self.snapshot = None
        self.boundary = None
        self.expected = {}
        self.owned = False

    def acquire(self):
        if self.destination.query(LOCK_SQL) != "t":
            raise CopyError("another copy run owns this destination")
        self.owned = True
        self.source.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY")
        self.snapshot = self.source.query("SELECT pg_export_snapshot()")
        if not self.snapshot:
            raise CopyError("source did not export a snapshot")

    def check(self):
        if not self.owned or self.source.failed or self.destination.failed:
            raise CopyError("copy run has lost its database session")
        # Probe both connections before accepting any operation. The actual
        # destination operation still uses the connection that holds the lock.
        self.source.query("SELECT 1")
        self.destination.query("SELECT 1")

    def boundary_at(self, ceiling, timestamp, epoch, tables, dry_run=False):
        self.check()
        proposed = dict(ceiling=ceiling, timestamp=timestamp, epoch=epoch,
                        tables=tables, source=self.source_identity,
                        snapshot=self.snapshot)
        if dry_run:
            self.boundary = proposed
            return proposed
        self.destination.query(
            f"CREATE TABLE IF NOT EXISTS {RUN_TABLE} ("
            "singleton boolean PRIMARY KEY CHECK (singleton), "
            "boundary text NOT NULL, completed boolean NOT NULL DEFAULT false)"
        )
        previous = self.destination.query(
            f"SELECT boundary FROM {RUN_TABLE} WHERE singleton AND NOT completed"
        )
        if previous:
            proposed = json.loads(previous)
            if proposed["source"] != self.source_identity or proposed["tables"] != tables:
                raise CopyError("interrupted boundary belongs to a different source or table list")
        else:
            self.destination.query(
                f"INSERT INTO {RUN_TABLE} (singleton, boundary, completed) VALUES "
                f"(true, {literal(json.dumps(proposed, sort_keys=True))}, false) "
                "ON CONFLICT (singleton) DO UPDATE SET boundary = EXCLUDED.boundary, completed = false"
            )
        self.boundary = proposed
        return proposed

    def prepare(self, table, columns, projection, predicate):
        self.check()
        # Compare projected values using the destination column names. The
        # source expression may derive trace_address, but its alias is identical.
        expected = fingerprint(self.source, table, projection, predicate)
        expected["columns"] = columns
        encoded = json.dumps(expected, sort_keys=True)
        previous = self.destination.query(
            f"SELECT source_fingerprint FROM {PROGRESS} WHERE table_name = {literal(table)} "
            f"AND snapshot_id = {literal(self.boundary['snapshot'])}"
        )
        if previous and previous != encoded:
            raise CopyError(f"{table}: source keys or content changed since the interrupted snapshot")
        self.destination.query(
            f"INSERT INTO {PROGRESS} (table_name, ceiling, snapshot_id, source_fingerprint) "
            f"VALUES ({literal(table)}, {self.boundary['ceiling']}, "
            f"{literal(self.boundary['snapshot'])}, {literal(encoded)}) "
            "ON CONFLICT (table_name) DO UPDATE SET snapshot_id = EXCLUDED.snapshot_id, "
            "source_fingerprint = EXCLUDED.source_fingerprint"
        )
        self.expected[table] = expected
        return expected

    def copy(self, table, columns, projection, predicate):
        self.check()
        rows = self.source.stream(
            f"COPY (SELECT {projection} FROM public.{ident(table)} {predicate}) TO STDOUT"
        )
        try:
            self.destination.copy_from(
                f'COPY public."paxeer_x_copy_stage" ({columns}) FROM STDIN', rows
            )
        except Exception:
            # A failed producer must never leave a partially loaded COPY usable.
            self.source.failed = self.destination.failed = True
            raise
        return ""

    def commit(self, table, columns, ceiling, high_key):
        self.check()
        ceiling = str(int(ceiling))
        high_key = "NULL" if high_key == "NULL" else str(int(high_key))
        return self.destination.query(
            f"WITH inserted AS (INSERT INTO public.{ident(table)} ({columns}) "
            f'SELECT {columns} FROM public."paxeer_x_copy_stage" '
            "ON CONFLICT DO NOTHING RETURNING 1), "
            "counted AS (SELECT count(*)::bigint AS row_count FROM inserted), "
            f"recorded AS (INSERT INTO {PROGRESS} AS p "
            "(table_name, ceiling, highest_key, rows_copied, updated_at) "
            f"SELECT {literal(table)}, {ceiling}, {high_key}, counted.row_count, now() FROM counted "
            "ON CONFLICT (table_name) DO UPDATE SET ceiling = EXCLUDED.ceiling, "
            "highest_key = GREATEST(p.highest_key, EXCLUDED.highest_key), "
            "rows_copied = p.rows_copied + EXCLUDED.rows_copied, "
            "updated_at = now()) SELECT row_count FROM counted"
        )

    def verify(self, table, columns, predicate):
        self.check()
        actual = fingerprint(self.destination, table, columns, predicate)
        expected = self.expected[table]
        return dict(source=expected["count"], target=actual["count"],
                    matches=(expected["count"] == actual["count"] and
                             expected["digest"] == actual["digest"]))

    def complete(self):
        self.check()
        self.destination.query(f"UPDATE {RUN_TABLE} SET completed = true WHERE singleton")
        return ""

    def dispatch(self, operation, args):
        if operation == "boundary":
            result = self.boundary_at(*args[:4], dry_run=args[4] == "1")
            return "\n".join(str(result[k]) for k in ("ceiling", "timestamp", "epoch", "snapshot"))
        if operation == "prepare":
            self.prepare(*args)
            return ""
        if operation == "copy":
            return self.copy(*args)
        if operation == "commit":
            return self.commit(*args)
        if operation == "verify":
            result = self.verify(*args)
            return f"{result['source']} {result['target']} {int(result['matches'])}"
        if operation == "complete":
            return self.complete()
        if operation in ("src", "dst"):
            self.check()
            session = self.source if operation == "src" else self.destination
            return session.query(*args)
        raise CopyError("unknown copy operation")

    def close(self):
        self.owned = False
        try:
            self.source.close()
        finally:
            self.destination.close()


def handle_request(run, reader, writer):
    try:
        request = json.loads(reader.readline())
        response = {"result": run.dispatch(request["operation"], request["args"])}
    except Exception as error:
        # Database errors themselves stay on psql's stderr, not in the RPC body.
        response = {"error": str(error)}
    writer.write(json.dumps(response) + "\n")
    writer.flush()


def serve(path):
    parent = os.getppid()
    with contextlib.ExitStack() as cleanup:
        source = PsqlSession.connect(os.environ["SRC_DATABASE_URL"])
        cleanup.callback(source.close)
        destination = PsqlSession.connect(os.environ["DST_DATABASE_URL"])
        cleanup.callback(destination.close)
        identity = hashlib.sha256(os.environ["SRC_DATABASE_URL"].encode()).hexdigest()
        run = CopyRun(source, destination, identity)
        run.acquire()
        listener = cleanup.enter_context(socket.socket(socket.AF_UNIX))
        listener.bind(path)
        listener.listen(1)
        listener.settimeout(1)
        print("ready", flush=True)
        while os.getppid() == parent:
            try:
                connection, _ = listener.accept()
            except socket.timeout:
                continue
            with connection, connection.makefile("r", encoding="utf-8") as reader, \
                    connection.makefile("w", encoding="utf-8") as writer:
                handle_request(run, reader, writer)


def request(path, operation, args):
    with socket.socket(socket.AF_UNIX) as connection:
        connection.connect(path)
        with connection.makefile("rw", encoding="utf-8") as channel:
            channel.write(json.dumps(dict(operation=operation, args=args)) + "\n")
            channel.flush()
            response = json.loads(channel.readline())
    if "error" in response:
        raise CopyError(response["error"])
    return response["result"]


def main():
    if sys.argv[1] == "serve":
        def stop(_signum, _frame):
            raise SystemExit(0)
        signal.signal(signal.SIGTERM, stop)
        signal.signal(signal.SIGINT, stop)
        serve(sys.argv[2])
    else:
        print(request(sys.argv[2], sys.argv[3], sys.argv[4:]))


if __name__ == "__main__":
    try:
        main()
    except (CopyError, OSError) as error:
        print(f"error: {error}", file=sys.stderr)
        raise SystemExit(2)
