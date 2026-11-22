CREATE INDEX delivery_claim_seq ON perps.delivery_outbox (perps_seq)
WHERE state = 'pending' OR (state = 'quarantined' AND
    (failure_class = 'transient' OR (failure_class <> 'transient' AND broker_sequence IS NULL)));
CREATE INDEX delivery_open_age ON perps.delivery_outbox (created_at)
WHERE state = 'pending' OR (state = 'quarantined' AND broker_sequence IS NULL);
CREATE INDEX delivery_open_seq ON perps.delivery_outbox (perps_seq)
WHERE state = 'pending' OR (state = 'quarantined' AND broker_sequence IS NULL);
CREATE INDEX delivery_published_age ON perps.delivery_outbox (published_at, perps_seq)
WHERE state = 'published' AND broker_sequence IS NOT NULL;
CREATE INDEX delivery_transient_available ON perps.delivery_outbox (available_at)
WHERE state = 'quarantined' AND failure_class = 'transient';
