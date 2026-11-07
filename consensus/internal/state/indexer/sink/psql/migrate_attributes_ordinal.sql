-- Apply once to databases created with UNIQUE (event_id, key), before starting
-- the updated sink. Original attribute order was not stored; assign existing
-- rows deterministic positions. New writes retain their original positions.
BEGIN;
LOCK TABLE attributes IN ACCESS EXCLUSIVE MODE;
ALTER TABLE attributes ADD COLUMN ordinal INTEGER;
WITH positions AS (
  SELECT event_id, key,
         (ROW_NUMBER() OVER (PARTITION BY event_id ORDER BY key) - 1)::INTEGER AS ordinal
  FROM attributes
)
UPDATE attributes AS a SET ordinal = p.ordinal
FROM positions AS p WHERE a.event_id = p.event_id AND a.key = p.key;
ALTER TABLE attributes ALTER COLUMN ordinal SET NOT NULL;
ALTER TABLE attributes ADD CONSTRAINT attributes_ordinal_check CHECK (ordinal >= 0);
ALTER TABLE attributes DROP CONSTRAINT attributes_event_id_key_key;
ALTER TABLE attributes ADD PRIMARY KEY (event_id, ordinal);
COMMIT;
