CREATE TABLE perps.chain_counters (
    name text PRIMARY KEY,
    value bigint NOT NULL CHECK (value >= 0)
);

CREATE FUNCTION perps.bump_chain_counter() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE perps.chain_counters SET value = value + 1 WHERE name = TG_TABLE_NAME;
    ELSE
        UPDATE perps.chain_counters SET value = value - 1 WHERE name = TG_TABLE_NAME;
    END IF;
    RETURN NULL;
END $$;

CREATE TRIGGER blocks_chain_counter AFTER INSERT OR DELETE ON perps.blocks FOR EACH ROW EXECUTE FUNCTION perps.bump_chain_counter();
CREATE TRIGGER tx_actions_chain_counter AFTER INSERT OR DELETE ON perps.tx_actions FOR EACH ROW EXECUTE FUNCTION perps.bump_chain_counter();

INSERT INTO perps.chain_counters (name, value) VALUES
    ('blocks', (SELECT count(*) FROM perps.blocks)),
    ('tx_actions', (SELECT count(*) FROM perps.tx_actions));
