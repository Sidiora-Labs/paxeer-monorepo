-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY idx_ph_balance_numeric
    ON stats.pool_holders (pool_address, (balance::numeric) DESC, holder_address ASC);

-- +goose Down
DROP INDEX CONCURRENTLY stats.idx_ph_balance_numeric;
