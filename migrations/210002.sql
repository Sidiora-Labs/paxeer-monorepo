SET LOCAL search_path = "discovery_notifications", pg_catalog;
CREATE TABLE IF NOT EXISTS discovery_notifications.deleted_accounts (
 account_key text PRIMARY KEY, deleted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON "discovery_notifications"."deleted_accounts" FROM PUBLIC;
