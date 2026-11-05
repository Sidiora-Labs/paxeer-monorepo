CREATE SCHEMA IF NOT EXISTS discovery_notifications;
CREATE TABLE IF NOT EXISTS discovery_notifications.inbox (
 id text PRIMARY KEY, account_id text NOT NULL, trigger_id text NOT NULL, template_version text NOT NULL,
 document jsonb NOT NULL, created_at timestamptz NOT NULL, read_at timestamptz,
 UNIQUE(account_id,trigger_id,template_version)
);
DO $$
BEGIN
 IF EXISTS (
  SELECT 1 FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
  WHERE a.attrelid='discovery_notifications.inbox'::regclass AND a.attname='v2_id'
   AND pg_get_expr(d.adbin,d.adrelid) IN ('(md5(id))::uuid', 'md5(id)::uuid')
 ) THEN
  ALTER TABLE discovery_notifications.inbox DROP COLUMN v2_id;
 END IF;
END $$;
ALTER TABLE discovery_notifications.inbox ADD COLUMN IF NOT EXISTS v2_id uuid GENERATED ALWAYS AS (
 (lpad(to_hex(floor(extract(epoch FROM created_at AT TIME ZONE 'UTC') * 1000)::bigint),12,'0')
  || '7' || substr(md5(id),14,3)
  || substr('89ab',((get_byte(decode(md5(id),'hex'),8) >> 4) & 3)+1,1)
  || substr(md5(id),18,15))::uuid
) STORED;
CREATE UNIQUE INDEX IF NOT EXISTS notification_v2_id ON discovery_notifications.inbox(v2_id);
CREATE INDEX IF NOT EXISTS notification_recipient_time ON discovery_notifications.inbox(account_id,created_at DESC,id);
CREATE TABLE IF NOT EXISTS discovery_notifications.outbox (
 id text PRIMARY KEY REFERENCES discovery_notifications.inbox(id) ON DELETE CASCADE,
 deliveries jsonb NOT NULL, next_at timestamptz NOT NULL, lease_owner text, lease_until timestamptz,
 fence bigint NOT NULL DEFAULT 0, terminal boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS notification_due ON discovery_notifications.outbox(next_at) WHERE NOT terminal;

CREATE TABLE IF NOT EXISTS discovery_notifications.deleted_accounts (
 account_key text PRIMARY KEY, deleted_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
