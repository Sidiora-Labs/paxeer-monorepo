SET LOCAL search_path = "stats_search", pg_catalog;

WITH migration_time AS (
    SELECT clock_timestamp() AS observed_at
), initial AS (
    INSERT INTO stats_search.generations_v2(
        generation,
        state,
        source_watermark,
        source_observed_at,
        row_count,
        digest,
        validated_at,
        activated_at
    )
    SELECT
        'initial',
        'active',
        1,
        migration_time.observed_at,
        0,
        'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
        migration_time.observed_at,
        migration_time.observed_at
    FROM migration_time
    WHERE NOT EXISTS (SELECT 1 FROM stats_search.generations_v2)
      AND NOT EXISTS (SELECT 1 FROM stats_search.active_v2)
    RETURNING generation
)
INSERT INTO stats_search.active_v2(singleton, generation, revision, privacy_revision)
SELECT true, generation, nextval('stats_search.revision_v2'), 1
FROM initial;
