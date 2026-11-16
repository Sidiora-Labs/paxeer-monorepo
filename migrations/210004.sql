SET LOCAL search_path = discovery_notifications, pg_catalog;
ALTER TABLE discovery_notifications.inbox ADD COLUMN IF NOT EXISTS v2_id uuid GENERATED ALWAYS AS (
 (lpad(to_hex(floor(extract(epoch FROM created_at AT TIME ZONE 'UTC') * 1000)::bigint),12,'0')
  || '7' || substr(md5(id),14,3)
  || substr('89ab',((get_byte(decode(md5(id),'hex'),8) >> 4) & 3)+1,1)
  || substr(md5(id),18,15))::uuid
) STORED;
CREATE UNIQUE INDEX IF NOT EXISTS notification_v2_id ON discovery_notifications.inbox(v2_id);
