import sqlite3
from hashlib import sha256

from .x402 import refuse_payer_grant
from .x402_activity import bind_receive_activity
from .x402_receive import decode_receive
from .x402_rpc import rpc_hex


class PreparedGrantDraws:
    def __init__(self, path, actor, network, rpc, authority, receipt_keys):
        self.path, self.actor, self.network = path, actor, network
        self.rpc, self.authority, self.receipt_keys = rpc, authority, receipt_keys
        with sqlite3.connect(path) as db:
            db.execute("PRAGMA journal_mode=WAL")
            db.execute("BEGIN IMMEDIATE")
            db.execute(
                "CREATE TABLE IF NOT EXISTS grant_draws (idempotency_key TEXT PRIMARY KEY, principal TEXT NOT NULL, request_digest TEXT NOT NULL, canonical BLOB NOT NULL, receive BLOB NOT NULL, transaction_hash TEXT NOT NULL UNIQUE, period_key TEXT UNIQUE, attempted INTEGER NOT NULL DEFAULT 0, attempted_at INTEGER)"
            )
            columns = {row[1] for row in db.execute("PRAGMA table_info(grant_draws)")}
            if "activity_id" in columns and "transaction_hash" not in columns:
                rows = db.execute(
                    "SELECT idempotency_key, canonical, receive, activity_id FROM grant_draws"
                ).fetchall()
                for key, canonical, receive, previous in rows:
                    transaction_hash = bind_receive_activity(canonical, receive, actor, network, key)
                    historical = sha256(b"LXP/v1/activity-id\0" + canonical).hexdigest()
                    if previous not in (historical, transaction_hash):
                        raise ValueError("grant draw stored identity mismatch")
                    db.execute("UPDATE grant_draws SET activity_id = ? WHERE idempotency_key = ?",
                               (transaction_hash, key))
                db.execute("ALTER TABLE grant_draws RENAME COLUMN activity_id TO transaction_hash")

    def register(self, principal, request_digest, canonical, receive, key, period_key=None):
        # The ledger has retired payer grants. New registrations must not become sends.
        refuse_payer_grant()

    def _resolve(self, claimed, row, offer):
        refuse_payer_grant()

    def recover_pending(self, key):
        """Look up an existing uncertain submission without ever retransmitting it."""
        rpc_hex(key, 32)
        with sqlite3.connect(self.path) as db:
            row = db.execute(
                "SELECT canonical, receive, transaction_hash FROM grant_draws WHERE idempotency_key = ?",
                (key,),
            ).fetchone()
        if row is None:
            refuse_payer_grant()
        canonical, receive, transaction_hash = row
        if bind_receive_activity(canonical, receive, self.actor, self.network, key) != transaction_hash:
            raise ValueError("grant draw stored identity mismatch")
        result = self.rpc.receipt(transaction_hash)
        if result is None or result.get("rung") == "submitted":
            return None
        # A receipt cannot turn the retired payer-grant path into accepted payment evidence.
        return self._resolve(False, row, None)

    def __call__(self, principal, request_digest, body, offer):
        if not isinstance(body, dict) or set(body) != {"receive", "idempotencyKey"}:
            raise ValueError("grant draw request binding")
        key = body["idempotencyKey"]
        rpc_hex(key, 32)
        receive = rpc_hex(body["receive"], 733)
        with sqlite3.connect(self.path) as db:
            row = db.execute(
                "SELECT principal, request_digest, receive FROM grant_draws WHERE idempotency_key = ?",
                (key,),
            ).fetchone()
        if row is None:
            refuse_payer_grant()
        value = decode_receive(receive)
        if (row != (principal, request_digest, receive)
                or value["amount"] != offer.get("amount") or value["asset"] != offer.get("asset")
                or value["to"] != offer.get("payTo") or offer.get("scheme") not in ("metered", "subscription")):
            raise ValueError("grant draw request binding")
        return self.recover_pending(key)
