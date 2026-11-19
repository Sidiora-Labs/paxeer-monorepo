CREATE INDEX index_prices_retention_time_idx
    ON perps.index_prices (block_time, row_id)
    WHERE kind IN ('aggregate', 'skipped');

CREATE INDEX index_prices_latest_idx
    ON perps.index_prices (market_id, block DESC, tx_index DESC, log_index DESC, sub DESC)
    INCLUDE (row_id);

CREATE INDEX index_prices_minute_aggregate_idx
    ON perps.index_prices (market_id, (date_trunc('minute', block_time AT TIME ZONE 'UTC')),
        block DESC, tx_index DESC, log_index DESC, sub DESC)
    INCLUDE (row_id, block_time)
    WHERE kind = 'aggregate';

CREATE INDEX protocol_events_price_retention_idx
    ON perps.protocol_events (block_time, row_id)
    WHERE event_name IN ('pricefeed_aggregate', 'pricefeed_aggregate_skipped');
