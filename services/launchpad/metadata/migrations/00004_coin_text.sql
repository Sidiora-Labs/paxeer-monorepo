-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION metadata.carry_coin_text() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.name := left(NEW.name, 100);
    NEW.symbol := left(NEW.symbol, 16);
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER carry_coin_text
BEFORE INSERT OR UPDATE OF name, symbol ON metadata.token_metadata
FOR EACH ROW EXECUTE FUNCTION metadata.carry_coin_text();

UPDATE metadata.token_metadata SET name = left(name, 100), symbol = left(symbol, 16)
WHERE char_length(name) > 100 OR char_length(symbol) > 16;

CREATE OR REPLACE VIEW metadata.token_search_v1 AS
SELECT token_address, pool_address, left(name, 100) AS name, left(symbol, 16) AS symbol,
       description, created_by, created_at
FROM metadata.token_metadata;
REVOKE ALL ON metadata.token_search_v1 FROM PUBLIC;

-- +goose Down
DROP TRIGGER carry_coin_text ON metadata.token_metadata;
DROP FUNCTION metadata.carry_coin_text();
CREATE OR REPLACE VIEW metadata.token_search_v1 AS
SELECT token_address, pool_address, name, symbol, description, created_by, created_at
FROM metadata.token_metadata;
