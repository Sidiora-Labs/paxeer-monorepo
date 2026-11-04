import sqlite3

from .x402 import refuse_payer_grant


class PreparedGrantDraws:
    def __init__(self, path, actor, network, rpc, authority, receipt_keys):
        self.path, self.actor, self.network = path, actor, network
        self.rpc, self.authority, self.receipt_keys = rpc, authority, receipt_keys
        with sqlite3.connect(path) as db:
            db.execute("PRAGMA journal_mode=WAL")
            db.execute(
                "CREATE TABLE IF NOT EXISTS grant_draws (idempotency_key TEXT PRIMARY KEY, principal TEXT NOT NULL, request_digest TEXT NOT NULL, canonical BLOB NOT NULL, receive BLOB NOT NULL, activity_id TEXT NOT NULL UNIQUE, period_key TEXT UNIQUE, attempted INTEGER NOT NULL DEFAULT 0, attempted_at INTEGER)"
            )

    def register(
        self, principal, request_digest, canonical, receive, key, period_key=None
    ):
        refuse_payer_grant()

    def _resolve(self, claimed, row, offer):
        refuse_payer_grant()

    def __call__(self, principal, request_digest, body, offer):
        refuse_payer_grant()
